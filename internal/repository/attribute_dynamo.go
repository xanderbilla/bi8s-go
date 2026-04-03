// Package repository contains all database access structures.
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

// AttributeRepository defines the interface for attribute data operations.
type AttributeRepository interface {
	GetAll(ctx context.Context) ([]model.Attribute, error)
	Get(ctx context.Context, id string) (*model.Attribute, error)
	GetByName(ctx context.Context, name string) (*model.Attribute, error)
	Create(ctx context.Context, attribute model.Attribute) error
	Delete(ctx context.Context, id string) error
}

// AttributeDynamoRepository implements AttributeRepository using DynamoDB.
type AttributeDynamoRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewAttributeDynamoRepository creates a new DynamoDB-backed attribute repository.
func NewAttributeDynamoRepository(client *dynamodb.Client, tableName string) *AttributeDynamoRepository {
	return &AttributeDynamoRepository{
		client:    client,
		tableName: tableName,
	}
}

// GetAll returns all attributes from DynamoDB.
func (r *AttributeDynamoRepository) GetAll(ctx context.Context) ([]model.Attribute, error) {
	out, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(r.tableName),
	})
	if err != nil {
		return nil, err
	}

	var attributes []model.Attribute
	if err := attributevalue.UnmarshalListOfMaps(out.Items, &attributes); err != nil {
		return nil, err
	}

	return attributes, nil
}

// Get returns a single attribute by ID.
func (r *AttributeDynamoRepository) Get(ctx context.Context, id string) (*model.Attribute, error) {
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

	var attribute model.Attribute
	if err := attributevalue.UnmarshalMap(out.Item, &attribute); err != nil {
		return nil, err
	}

	return &attribute, nil
}

// GetByName returns a single attribute by name using Scan with filter.
func (r *AttributeDynamoRepository) GetByName(ctx context.Context, name string) (*model.Attribute, error) {
	out, err := r.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("#name = :name"),
		ExpressionAttributeNames: map[string]string{
			"#name": "name",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":name": &types.AttributeValueMemberS{Value: name},
		},
	})
	if err != nil {
		return nil, err
	}

	if len(out.Items) == 0 {
		return nil, nil
	}

	var attribute model.Attribute
	if err := attributevalue.UnmarshalMap(out.Items[0], &attribute); err != nil {
		return nil, err
	}

	return &attribute, nil
}

// Create saves a new attribute to DynamoDB.
func (r *AttributeDynamoRepository) Create(ctx context.Context, attribute model.Attribute) error {
	item, err := attributevalue.MarshalMap(attribute)
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

// Delete removes an attribute from DynamoDB by ID.
func (r *AttributeDynamoRepository) Delete(ctx context.Context, id string) error {
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
