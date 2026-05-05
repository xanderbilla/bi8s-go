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

// ContentCastEntry represents a row in the content_cast relation table.
// PK: personId, SK: contentId — enables efficient lookups by person.
type ContentCastEntry struct {
	PersonID    string `dynamodbav:"personId"`
	ContentID   string `dynamodbav:"contentId"`
	ContentType string `dynamodbav:"contentType"`
	Visibility  string `dynamodbav:"visibility"`
}

type ContentCastRepository interface {
	// GetContentIdsByPersonId returns contentIds for a person, with optional contentType filter.
	GetContentIdsByPersonId(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]string, map[string]types.AttributeValue, error)
	// PutEntry writes a cast relation entry.
	PutEntry(ctx context.Context, entry ContentCastEntry) error
	// DeleteEntry removes a single cast relation.
	DeleteEntry(ctx context.Context, personId, contentId string) error
	// DeleteAllByContentId removes all cast entries for a given content item.
	DeleteAllByContentId(ctx context.Context, contentId string, castIds []string) error
}

type DynamoContentCastRepository struct {
	*BaseRepository
}

func NewContentCastRepository(client *dynamodb.Client, tableName string) ContentCastRepository {
	return &DynamoContentCastRepository{BaseRepository: NewBaseRepository(client, tableName)}
}

func (r *DynamoContentCastRepository) GetContentIdsByPersonId(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]string, map[string]types.AttributeValue, error) {
	return WithTimeoutResultPage(ctx, "get_content_ids_by_person", func(ctx context.Context) ([]string, map[string]types.AttributeValue, error) {
		vals := map[string]types.AttributeValue{
			":personId": &types.AttributeValueMemberS{Value: personId},
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
			KeyConditionExpression:    aws.String("personId = :personId"),
			ExpressionAttributeValues: vals,
			FilterExpression:          filterExpr,
			Limit:                     aws.Int32(limit),
			ExclusiveStartKey:         startKey,
		}

		items, nextKey, err := QueryPage(ctx, r.GetClient(), input)
		if err != nil {
			return nil, nil, err
		}

		var entries []ContentCastEntry
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

func (r *DynamoContentCastRepository) PutEntry(ctx context.Context, entry ContentCastEntry) error {
	return r.WithTimeout(ctx, "put_cast_entry", func(ctx context.Context) error {
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

func (r *DynamoContentCastRepository) DeleteEntry(ctx context.Context, personId, contentId string) error {
	return r.WithTimeout(ctx, "delete_cast_entry", func(ctx context.Context) error {
		_, err := r.GetClient().DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(r.GetTableName()),
			Key: map[string]types.AttributeValue{
				"personId":  &types.AttributeValueMemberS{Value: personId},
				"contentId": &types.AttributeValueMemberS{Value: contentId},
			},
		})
		return err
	})
}

func (r *DynamoContentCastRepository) DeleteAllByContentId(ctx context.Context, contentId string, castIds []string) error {
	for _, personId := range castIds {
		if err := r.DeleteEntry(ctx, personId, contentId); err != nil {
			return fmt.Errorf("delete_all_cast_entries: %w", err)
		}
	}
	return nil
}
