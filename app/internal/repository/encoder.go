package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

// EncoderRepository retrieves encoder job records from DynamoDB.
type EncoderRepository interface {
	GetFinishedByContentID(ctx context.Context, contentID string) (*model.EncoderJob, error)
}

// DynamoEncoderRepository is the DynamoDB-backed implementation.
type DynamoEncoderRepository struct {
	*BaseRepository
	contentIDIndex string
}

// NewEncoderRepository creates a new DynamoEncoderRepository.
func NewEncoderRepository(client *dynamodb.Client, tableName, contentIDIndex string) *DynamoEncoderRepository {
	return &DynamoEncoderRepository{
		BaseRepository: NewBaseRepository(client, tableName),
		contentIDIndex: contentIDIndex,
	}
}

// GetFinishedByContentID queries the contentId GSI and returns the most
// recent FINISHED job for the given content ID. Returns errs.NotFound when
// no finished record exists.
func (r *DynamoEncoderRepository) GetFinishedByContentID(ctx context.Context, contentID string) (*model.EncoderJob, error) {
	return WithTimeoutResult(ctx, "encoder.GetFinishedByContentID", func(ctx context.Context) (*model.EncoderJob, error) {
		keyEx := expression.Key("contentId").Equal(expression.Value(contentID))
		filterEx := expression.Name("status").Equal(expression.Value("FINISHED"))
		expr, err := expression.NewBuilder().
			WithKeyCondition(keyEx).
			WithFilter(filterEx).
			Build()
		if err != nil {
			return nil, err
		}

		out, err := r.GetClient().Query(ctx, &dynamodb.QueryInput{
			TableName:                 aws.String(r.GetTableName()),
			IndexName:                 aws.String(r.contentIDIndex),
			KeyConditionExpression:    expr.KeyCondition(),
			FilterExpression:          expr.Filter(),
			ExpressionAttributeNames:  expr.Names(),
			ExpressionAttributeValues: expr.Values(),
			Limit:                     aws.Int32(1),
		})
		if err != nil {
			return nil, err
		}
		if len(out.Items) == 0 {
			return nil, errs.NewNotFound("playback data not available for this content")
		}

		var job model.EncoderJob
		if err := attributevalue.UnmarshalMap(out.Items[0], &job); err != nil {
			return nil, err
		}
		return &job, nil
	})
}
