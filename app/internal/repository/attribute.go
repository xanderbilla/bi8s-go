package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

type AttributeRepository interface {
	GetAll(ctx context.Context) ([]model.Attribute, error)
	Get(ctx context.Context, id string) (*model.Attribute, error)
	GetByName(ctx context.Context, name string) (*model.Attribute, error)
	Create(ctx context.Context, attribute model.Attribute) error
	Delete(ctx context.Context, id string) error
}

type AttributeDynamoRepository struct {
	*BaseRepository
}

func NewAttributeDynamoRepository(client *dynamodb.Client, tableName string) *AttributeDynamoRepository {
	return &AttributeDynamoRepository{
		BaseRepository: NewBaseRepository(client, tableName),
	}
}

func (r *AttributeDynamoRepository) GetAll(ctx context.Context) ([]model.Attribute, error) {
	return WithTimeoutResult(ctx, "attribute.GetAll", func(ctx context.Context) ([]model.Attribute, error) {
		items, err := ScanAllPaged(ctx, r.GetClient(), &dynamodb.ScanInput{
			TableName: aws.String(r.GetTableName()),
		}, DefaultMaxScanPages)
		if err != nil {
			return nil, err
		}

		var attributes []model.Attribute
		if err := attributevalue.UnmarshalListOfMaps(items, &attributes); err != nil {
			return nil, err
		}
		return attributes, nil
	})
}

func (r *AttributeDynamoRepository) Get(ctx context.Context, id string) (*model.Attribute, error) {
	return WithTimeoutResult(ctx, "attribute.Get", func(ctx context.Context) (*model.Attribute, error) {
		out, err := r.GetClient().GetItem(ctx, &dynamodb.GetItemInput{
			TableName:      aws.String(r.GetTableName()),
			ConsistentRead: aws.Bool(true),
			Key: map[string]types.AttributeValue{
				"id": &types.AttributeValueMemberS{Value: id},
			},
		})
		if err != nil {
			return nil, err
		}

		if out.Item == nil {
			return nil, nil
		}

		var attribute model.Attribute
		if err := attributevalue.UnmarshalMap(out.Item, &attribute); err != nil {
			return nil, err
		}

		return &attribute, nil
	})
}

func (r *AttributeDynamoRepository) GetByName(ctx context.Context, name string) (*model.Attribute, error) {
	return WithTimeoutResult(ctx, "attribute.GetByName", func(ctx context.Context) (*model.Attribute, error) {
		items, err := ScanAllPaged(ctx, r.GetClient(), &dynamodb.ScanInput{
			TableName:        aws.String(r.GetTableName()),
			FilterExpression: aws.String("#name = :name"),
			ExpressionAttributeNames: map[string]string{
				"#name": "name",
			},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":name": &types.AttributeValueMemberS{Value: name},
			},
			Limit: aws.Int32(1),
		}, DefaultMaxScanPages)
		if err != nil {
			return nil, err
		}

		if len(items) == 0 {
			return nil, nil
		}

		var attribute model.Attribute
		if err := attributevalue.UnmarshalMap(items[0], &attribute); err != nil {
			return nil, err
		}

		return &attribute, nil
	})
}

func (r *AttributeDynamoRepository) Create(ctx context.Context, attribute model.Attribute) error {
	return r.WithTimeout(ctx, "attribute.Create", func(ctx context.Context) error {
		item, err := attributevalue.MarshalMap(attribute)
		if err != nil {
			return err
		}

		condition := expression.AttributeNotExists(expression.Name("id"))
		expr, err := expression.NewBuilder().WithCondition(condition).Build()
		if err != nil {
			return err
		}

		_, err = r.GetClient().PutItem(ctx, &dynamodb.PutItemInput{
			TableName:                 aws.String(r.GetTableName()),
			Item:                      item,
			ConditionExpression:       expr.Condition(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
		})

		return err
	})
}

func (r *AttributeDynamoRepository) Delete(ctx context.Context, id string) error {
	return r.WithTimeout(ctx, "attribute.Delete", func(ctx context.Context) error {
		condition := expression.AttributeExists(expression.Name("id"))
		expr, err := expression.NewBuilder().WithCondition(condition).Build()
		if err != nil {
			return err
		}

		_, err = r.GetClient().DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(r.GetTableName()),
			Key: map[string]types.AttributeValue{
				"id": &types.AttributeValueMemberS{Value: id},
			},
			ConditionExpression:       expr.Condition(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
		})

		return err
	})
}
