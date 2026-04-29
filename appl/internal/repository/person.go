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

type PersonRepository interface {
	GetAll(ctx context.Context) ([]model.Person, error)
	Get(ctx context.Context, id string) (*model.Person, error)
	Create(ctx context.Context, person model.Person) error
	Delete(ctx context.Context, id string) error
}

type PersonDynamoRepository struct {
	*BaseRepository
}

func NewPersonDynamoRepository(client *dynamodb.Client, tableName string) *PersonDynamoRepository {
	return &PersonDynamoRepository{
		BaseRepository: NewBaseRepository(client, tableName),
	}
}

func (r *PersonDynamoRepository) GetAll(ctx context.Context) ([]model.Person, error) {
	return WithTimeoutResult(ctx, "person.GetAll", func(ctx context.Context) ([]model.Person, error) {
		input := &dynamodb.ScanInput{TableName: aws.String(r.GetTableName())}
		items, err := ScanAllPaged(ctx, r.GetClient(), input, DefaultMaxScanPages)
		if err != nil {
			return nil, err
		}

		var persons []model.Person
		if err := attributevalue.UnmarshalListOfMaps(items, &persons); err != nil {
			return nil, err
		}
		return persons, nil
	})
}

func (r *PersonDynamoRepository) Get(ctx context.Context, id string) (*model.Person, error) {
	return WithTimeoutResult(ctx, "person.Get", func(ctx context.Context) (*model.Person, error) {
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

		var person model.Person
		if err := attributevalue.UnmarshalMap(out.Item, &person); err != nil {
			return nil, err
		}

		return &person, nil
	})
}

func (r *PersonDynamoRepository) Create(ctx context.Context, person model.Person) error {
	return r.WithTimeout(ctx, "person.Create", func(ctx context.Context) error {
		item, err := attributevalue.MarshalMap(person)
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

func (r *PersonDynamoRepository) Delete(ctx context.Context, id string) error {
	return r.WithTimeout(ctx, "person.Delete", func(ctx context.Context) error {
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
