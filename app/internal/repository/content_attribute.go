package repository

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

// ContentAttributeEntry represents a row in the content_attribute relation table.
// PK: attributeId, SK: contentId -- enables efficient lookups by attribute.
type ContentAttributeEntry struct {
	AttributeID string `dynamodbav:"attributeId"`
	ContentID   string `dynamodbav:"contentId"`
	ContentType string `dynamodbav:"contentType"`
	Visibility  string `dynamodbav:"visibility"`
}

type ContentAttributeRepository interface {
	// GetContentIdsByAttributeId returns contentIds for an attribute, with optional contentType filter.
	GetContentIdsByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]string, map[string]types.AttributeValue, error)
	// PutEntry writes an attribute relation entry.
	PutEntry(ctx context.Context, entry ContentAttributeEntry) error
	// DeleteEntry removes a single attribute relation.
	DeleteEntry(ctx context.Context, attributeId, contentId string) error
	// DeleteAllByContentId removes all attribute entries for a given content item.
	DeleteAllByContentId(ctx context.Context, contentId string, attributeIds []string) error
}

type DynamoContentAttributeRepository struct {
	*BaseRepository
}

func NewContentAttributeRepository(client *dynamodb.Client, tableName string) ContentAttributeRepository {
	return &DynamoContentAttributeRepository{BaseRepository: NewBaseRepository(client, tableName)}
}

func (r *DynamoContentAttributeRepository) GetContentIdsByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]string, map[string]types.AttributeValue, error) {
	return WithTimeoutResultPage(ctx, "get_content_ids_by_attribute", func(ctx context.Context) ([]string, map[string]types.AttributeValue, error) {
		vals := map[string]types.AttributeValue{
			":attributeId": &types.AttributeValueMemberS{Value: attributeId},
		}
		var filterExpr *string
		if ct, ok := model.ParseContentType(contentTypeFilter); ok {
			vals[":contentType"] = &types.AttributeValueMemberS{Value: string(ct)}
			f := "contentType = :contentType"
			filterExpr = &f
		}

		if limit <= 0 {
			limit = 20
		}
		input := &dynamodb.QueryInput{
			TableName:                 aws.String(r.GetTableName()),
			KeyConditionExpression:    aws.String("attributeId = :attributeId"),
			ExpressionAttributeValues: vals,
			FilterExpression:          filterExpr,
			Limit:                     aws.Int32(limit),
			ExclusiveStartKey:         startKey,
		}

		items, nextKey, err := QueryPage(ctx, r.GetClient(), input)
		if err != nil {
			return nil, nil, err
		}

		var entries []ContentAttributeEntry
		if err := attributevalue.UnmarshalListOfMaps(items, &entries); err != nil {
			return nil, nil, err
		}

		ids := make([]string, 0, len(entries))
		for _, e := range entries {
			ids = append(ids, e.ContentID)
		}
		return ids, nextKey, nil
	})
}

func (r *DynamoContentAttributeRepository) PutEntry(ctx context.Context, entry ContentAttributeEntry) error {
	return r.WithTimeout(ctx, "put_attribute_entry", func(ctx context.Context) error {
		item, err := attributevalue.MarshalMap(entry)
		if err != nil {
			return err
		}
		_, err = r.GetClient().PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(r.GetTableName()),
			Item:      item,
		})
		return err
	})
}

func (r *DynamoContentAttributeRepository) DeleteEntry(ctx context.Context, attributeId, contentId string) error {
	return r.WithTimeout(ctx, "delete_attribute_entry", func(ctx context.Context) error {
		_, err := r.GetClient().DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(r.GetTableName()),
			Key: map[string]types.AttributeValue{
				"attributeId": &types.AttributeValueMemberS{Value: attributeId},
				"contentId":   &types.AttributeValueMemberS{Value: contentId},
			},
		})
		return err
	})
}

func (r *DynamoContentAttributeRepository) DeleteAllByContentId(ctx context.Context, contentId string, attributeIds []string) error {
	for _, attributeId := range attributeIds {
		if attributeId == "" {
			continue
		}
		if err := r.DeleteEntry(ctx, attributeId, contentId); err != nil {
			return fmt.Errorf("delete_all_attribute_entries: %w", err)
		}
	}
	return nil
}
