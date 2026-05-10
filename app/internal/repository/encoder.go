package repository

import (
	"context"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

type EncoderRepository interface {
	Create(ctx context.Context, job model.EncoderJob) error
	Get(ctx context.Context, jobID string) (*model.EncoderJob, error)
	Update(ctx context.Context, job *model.EncoderJob) error
	GetByContentId(ctx context.Context, contentID string) ([]model.EncoderJob, error)
}

type DynamoEncoderRepository struct {
	*BaseRepository

	contentIDIndex string
}

func NewEncoderRepository(client *dynamodb.Client, tableName, contentIDIndex string) EncoderRepository {
	return &DynamoEncoderRepository{
		BaseRepository: NewBaseRepository(client, tableName),
		contentIDIndex: contentIDIndex,
	}
}

func (r *DynamoEncoderRepository) scanAll(ctx context.Context, input *dynamodb.ScanInput) ([]map[string]types.AttributeValue, error) {
	return ScanAllPaged(ctx, r.GetClient(), input, DefaultMaxScanPages)
}

func (r *DynamoEncoderRepository) Create(ctx context.Context, job model.EncoderJob) error {
	return r.WithTimeout(ctx, "encoder.Create", func(ctx context.Context) error {
		item, err := attributevalue.MarshalMap(job)
		if err != nil {
			return err
		}

		input := &dynamodb.PutItemInput{
			TableName:           aws.String(r.GetTableName()),
			Item:                item,
			ConditionExpression: aws.String("attribute_not_exists(id)"),
		}

		_, err = r.GetClient().PutItem(ctx, input)
		return err
	})
}

func (r *DynamoEncoderRepository) Get(ctx context.Context, jobID string) (*model.EncoderJob, error) {
	return WithTimeoutResult(ctx, "encoder.Get", func(ctx context.Context) (*model.EncoderJob, error) {
		input := &dynamodb.GetItemInput{
			TableName: aws.String(r.GetTableName()),
			Key: map[string]types.AttributeValue{
				"id": &types.AttributeValueMemberS{Value: jobID},
			},
			ConsistentRead: aws.Bool(true),
		}

		result, err := r.GetClient().GetItem(ctx, input)
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
	})
}

func (r *DynamoEncoderRepository) Update(ctx context.Context, job *model.EncoderJob) error {
	return r.WithTimeout(ctx, "encoder.Update", func(ctx context.Context) error {
		oldVersion := job.Version
		job.Version++

		item, err := attributevalue.MarshalMap(*job)
		if err != nil {
			job.Version = oldVersion
			return err
		}

		input := &dynamodb.PutItemInput{
			TableName:           aws.String(r.GetTableName()),
			Item:                item,
			ConditionExpression: aws.String("attribute_exists(id) AND (attribute_not_exists(version) OR version = :v)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":v": &types.AttributeValueMemberN{Value: strconv.Itoa(oldVersion)},
			},
		}

		if _, err := r.GetClient().PutItem(ctx, input); err != nil {
			job.Version = oldVersion
			return err
		}
		return nil
	})
}

func (r *DynamoEncoderRepository) GetByContentId(ctx context.Context, contentID string) ([]model.EncoderJob, error) {
	return WithTimeoutResult(ctx, "encoder.GetByContentId", func(ctx context.Context) ([]model.EncoderJob, error) {
		if r.contentIDIndex != "" {
			return r.queryByContentID(ctx, contentID)
		}
		return r.scanByContentID(ctx, contentID)
	})
}

func (r *DynamoEncoderRepository) queryByContentID(ctx context.Context, contentID string) ([]model.EncoderJob, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(r.GetTableName()),
		IndexName:              aws.String(r.contentIDIndex),
		KeyConditionExpression: aws.String("contentId = :contentId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":contentId": &types.AttributeValueMemberS{Value: contentID},
		},
	}
	items, err := QueryAllPaged(ctx, r.GetClient(), input, DefaultMaxScanPages)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []model.EncoderJob{}, nil
	}
	var jobs []model.EncoderJob
	if err := attributevalue.UnmarshalListOfMaps(items, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}

func (r *DynamoEncoderRepository) scanByContentID(ctx context.Context, contentID string) ([]model.EncoderJob, error) {
	items, err := r.scanAll(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(r.GetTableName()),
		FilterExpression: aws.String("contentId = :contentId"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":contentId": &types.AttributeValueMemberS{Value: contentID},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []model.EncoderJob{}, nil
	}
	var jobs []model.EncoderJob
	if err := attributevalue.UnmarshalListOfMaps(items, &jobs); err != nil {
		return nil, err
	}
	return jobs, nil
}
