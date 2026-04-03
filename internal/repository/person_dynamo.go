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

// PersonRepository defines the interface for person data operations.
type PersonRepository interface {
	GetAll(ctx context.Context) ([]model.Person, error)
	Get(ctx context.Context, id string) (*model.Person, error)
	Create(ctx context.Context, person model.Person) error
	Delete(ctx context.Context, id string) error
}

// PersonDynamoRepository implements PersonRepository using DynamoDB.
type PersonDynamoRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewPersonDynamoRepository creates a new DynamoDB-backed person repository.
func NewPersonDynamoRepository(client *dynamodb.Client, tableName string) *PersonDynamoRepository {
	return &PersonDynamoRepository{
		client:    client,
		tableName: tableName,
	}
}

// GetAll returns all persons from DynamoDB.
func (r *PersonDynamoRepository) GetAll(ctx context.Context) ([]model.Person, error) {
	out, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(r.tableName),
	})
	if err != nil {
		return nil, err
	}

	var persons []model.Person
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &persons); err != nil {
		return nil, err
	}

	return persons, nil
}

// Get returns a single person by ID.
func (r *PersonDynamoRepository) Get(ctx context.Context, id string) (*model.Person, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
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
}

// Create saves a new person to DynamoDB.
func (r *PersonDynamoRepository) Create(ctx context.Context, person model.Person) error {
	item, err := attributevalue.MarshalMap(person)
	if err != nil {
		return err
	}

	condition := expression.AttributeNotExists(expression.Name("id"))
	expr, err := expression.NewBuilder().WithCondition(condition).Build()
	if err != nil {
		return err
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:                 aws.String(r.tableName),
		Item:                      item,
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})

	return err
}

// Delete removes a person from DynamoDB by ID.
func (r *PersonDynamoRepository) Delete(ctx context.Context, id string) error {
	condition := expression.AttributeExists(expression.Name("id"))
	expr, err := expression.NewBuilder().WithCondition(condition).Build()
	if err != nil {
		return err
	}

	_, err = r.client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
		ConditionExpression:       expr.Condition(),
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
	})

	return err
}
