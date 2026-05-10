package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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
	return GetByID[model.Person](ctx, r.BaseRepository, "person.Get", id)
}

func (r *PersonDynamoRepository) Create(ctx context.Context, person model.Person) error {
	return CreateWithIDCondition(ctx, r.BaseRepository, "person.Create", person)
}

func (r *PersonDynamoRepository) Delete(ctx context.Context, id string) error {
	return DeleteByID(ctx, r.BaseRepository, "person.Delete", id)
}
