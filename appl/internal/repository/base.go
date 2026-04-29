package repository

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/xanderbilla/bi8s-go/internal/ctxutil"
	"github.com/xanderbilla/bi8s-go/internal/errs"
)

var DefaultMaxScanPages = 10

func ConfigureMaxScanPages(n int) {
	if n > 0 {
		DefaultMaxScanPages = n
	}
}

type DynamoAPI interface {
	GetItem(ctx context.Context, params *dynamodb.GetItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, params *dynamodb.PutItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	DeleteItem(ctx context.Context, params *dynamodb.DeleteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DeleteItemOutput, error)
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
	Scan(ctx context.Context, params *dynamodb.ScanInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
}

type BaseRepository struct {
	client    DynamoAPI
	tableName string
}

func NewBaseRepository(client DynamoAPI, tableName string) *BaseRepository {
	return &BaseRepository{
		client:    client,
		tableName: tableName,
	}
}

func (b *BaseRepository) GetClient() DynamoAPI {
	return b.client
}

func (b *BaseRepository) GetTableName() string {
	return b.tableName
}

func (b *BaseRepository) WithTimeout(ctx context.Context, operation string, fn func(context.Context) error) error {
	ctx, cancel := ctxutil.WithDBTimeout(ctx)
	defer cancel()

	if err := fn(ctx); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

func WithTimeoutResult[T any](ctx context.Context, operation string, fn func(context.Context) (T, error)) (T, error) {
	ctx, cancel := ctxutil.WithDBTimeout(ctx)
	defer cancel()

	result, err := fn(ctx)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("%s: %w", operation, err)
	}
	return result, nil
}

func ScanAllPaged(ctx context.Context, client DynamoAPI, input *dynamodb.ScanInput, maxPages int) ([]map[string]types.AttributeValue, error) {
	if maxPages <= 0 {
		maxPages = DefaultMaxScanPages
	}
	var items []map[string]types.AttributeValue
	for pages := 0; ; pages++ {
		out, err := client.Scan(ctx, input)
		if err != nil {
			return nil, err
		}
		items = append(items, out.Items...)
		if out.LastEvaluatedKey == nil {
			return items, nil
		}
		if pages+1 >= maxPages {
			return nil, errs.ErrResultTooLarge
		}
		input.ExclusiveStartKey = out.LastEvaluatedKey
	}
}

func QueryAllPaged(ctx context.Context, client DynamoAPI, input *dynamodb.QueryInput, maxPages int) ([]map[string]types.AttributeValue, error) {
	if maxPages <= 0 {
		maxPages = DefaultMaxScanPages
	}
	var items []map[string]types.AttributeValue
	for pages := 0; ; pages++ {
		out, err := client.Query(ctx, input)
		if err != nil {
			return nil, err
		}
		items = append(items, out.Items...)
		if out.LastEvaluatedKey == nil {
			return items, nil
		}
		if pages+1 >= maxPages {
			return nil, errs.ErrResultTooLarge
		}
		input.ExclusiveStartKey = out.LastEvaluatedKey
	}
}
