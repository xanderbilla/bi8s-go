package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

type PersonRepository interface {
	GetAll(ctx context.Context, limit int32, startKey map[string]types.AttributeValue) ([]model.Person, map[string]types.AttributeValue, error)
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

func (r *PersonDynamoRepository) GetAll(ctx context.Context, limit int32, startKey map[string]types.AttributeValue) ([]model.Person, map[string]types.AttributeValue, error) {
	return WithTimeoutResultPage(ctx, "person.GetAll", func(ctx context.Context) ([]model.Person, map[string]types.AttributeValue, error) {
		items, nextKey, err := ScanPage(ctx, r.GetClient(), &dynamodb.ScanInput{
			TableName:         aws.String(r.GetTableName()),
			Limit:             aws.Int32(defaultLimit(limit)),
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, nil, err
		}
		if len(items) == 0 {
			return []model.Person{}, nil, nil
		}
		var persons []model.Person
		if err := attributevalue.UnmarshalListOfMaps(items, &persons); err != nil {
			return nil, nil, err
		}
		return persons, nextKey, nil
	})
}

func (r *PersonDynamoRepository) Get(ctx context.Context, id string) (*model.Person, error) {
	return GetByID[model.Person](ctx, r.BaseRepository, "person.Get", id)
}

func (r *PersonDynamoRepository) Create(ctx context.Context, person model.Person) error {
	return CreateWithIDCondition(ctx, r.BaseRepository, "person.Create", person)
}

func (r *PersonDynamoRepository) Delete(ctx context.Context, id string) error {
	return DeleteByID(ctx, r.BaseRepository, "person.Delete", id)
}
