package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

// EncoderRepository defines the interface for encoder job operations
type EncoderRepository interface {
	Create(ctx context.Context, job model.EncoderJob) error
	Get(ctx context.Context, jobID string) (*model.EncoderJob, error)
	Update(ctx context.Context, job model.EncoderJob) error
	GetByContentId(ctx context.Context, contentID string) ([]model.EncoderJob, error)
}

// DynamoEncoderRepository implements EncoderRepository using DynamoDB
type DynamoEncoderRepository struct {
	client    *dynamodb.Client
	tableName string
}

// NewEncoderRepository creates a new encoder repository
func NewEncoderRepository(client *dynamodb.Client, tableName string) EncoderRepository {
	return &DynamoEncoderRepository{
		client:    client,
		tableName: tableName,
	}
}

// Create saves a new encoder job to DynamoDB
func (r *DynamoEncoderRepository) Create(ctx context.Context, job model.EncoderJob) error {
	item, err := attributevalue.MarshalMap(job)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		TableName:           aws.String(r.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(jobId)"),
	}

	_, err = r.client.PutItem(ctx, input)
	return err
}

// Get retrieves an encoder job by ID
func (r *DynamoEncoderRepository) Get(ctx context.Context, jobID string) (*model.EncoderJob, error) {
	input := &dynamodb.GetItemInput{
		TableName: aws.String(r.tableName),
		Key: map[string]types.AttributeValue{
			"jobId": &types.AttributeValueMemberS{Value: jobID},
		},
		ConsistentRead: aws.Bool(true),
	}

	result, err := r.client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, nil
	}

	var job model.EncoderJob
	err = attributevalue.UnmarshalMap(result.Item, &job)
	if err != nil {
		return nil, err
	}

	return &job, nil
}

// Update updates an existing encoder job
func (r *DynamoEncoderRepository) Update(ctx context.Context, job model.EncoderJob) error {
	item, err := attributevalue.MarshalMap(job)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(r.tableName),
		Item:      item,
	}

	_, err = r.client.PutItem(ctx, input)
	return err
}


// GetByContentId retrieves all encoder jobs for a specific content ID
// Note: This uses a scan operation. For production, consider adding a GSI on contentId
func (r *DynamoEncoderRepository) GetByContentId(ctx context.Context, contentID string) ([]model.EncoderJob, error) {
	input := &dynamodb.ScanInput{
		TableName:        aws.String(r.tableName),
		FilterExpression: aws.String("contentId = :contentId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":contentId": &types.AttributeValueMemberS{Value: contentID},
		},
	}

	result, err := r.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	var jobs []model.EncoderJob
	err = attributevalue.UnmarshalListOfMaps(result.Items, &jobs)
	if err != nil {
		return nil, err
	}

	return jobs, nil
}
