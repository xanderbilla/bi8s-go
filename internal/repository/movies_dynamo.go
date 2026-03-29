// Package repository contains all database access structures.
// Separating database logic natively creates a scalable clean architecture 
// enabling swapouts without polluting handler/service tiers.
package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Movie provides the domain schema mapping for DynamoDB storage representations.
// Employs structure tags specifying exact routing paths across endpoints and generic maps.
//   - json defines REST delivery representations.
//   - dynamodbav connects fields smoothly to corresponding Dynamo columns natively.
//   - validate establishes strict validation bounds intercepting bad payloads promptly.
type Movie struct {
	ID          string `json:"id" dynamodbav:"id" validate:"omitempty,min=1,max=64"`
	Title       string `json:"title" dynamodbav:"title" validate:"required,min=1,max=128"`
	Description string `json:"description" dynamodbav:"description" validate:"required,min=1,max=255"`
	Poster      string `json:"poster,omitempty" dynamodbav:"poster,omitempty"`
	Cover       string `json:"cover,omitempty" dynamodbav:"cover,omitempty"`
	Performer   string `json:"performer" dynamodbav:"performer" validate:"required,min=1,max=128"`
	Year        int    `json:"year" dynamodbav:"year" validate:"required,gte=1888,lte=2100"`
}

// MovieRepository signifies a data-agnostic boundary.
// It serves as the single source of truth for Movie operations avoiding tight couplings towards DynamoDB explicitly.
type MovieRepository interface {
	GetAll(ctx context.Context) ([]Movie, error)
	Get(ctx context.Context, id string) (*Movie, error)
	Create(ctx context.Context, movie Movie) error
	Delete(ctx context.Context, id string) error
}

// DynamoMovieRepository fulfills the repository boundary manipulating datasets utilizing the AWS DynamoDB SDK v2.
type DynamoMovieRepository struct {
	client *dynamodb.Client
	table  string
}

// GetAll iterates the configured table returning the collection.
// WARNING: Scans execute expensive broad sweeps over whole distributions.
// For scaled applications consider constructing targeted Queries via secondary index bounds mapping keys explicitly over unconstrained sequential scanning!
func (d *DynamoMovieRepository) GetAll(ctx context.Context) ([]Movie, error) {
	input := &dynamodb.ScanInput{
		TableName: &d.table,
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Count == 0 {
		return nil, nil
	}

	var movies []Movie
	err = attributevalue.UnmarshalListOfMaps(result.Items, &movies)
	if err != nil {
		return nil, err
	}

	return movies, nil
}

// Get extracts singular records constrained entirely by specific unique partition identifers ensuring maximum speeds.
func (d *DynamoMovieRepository) Get(ctx context.Context, id string) (*Movie, error) {
	input := &dynamodb.GetItemInput{
		TableName: &d.table,
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
		ConsistentRead: aws.Bool(true),
	}

	result, err := d.client.GetItem(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, nil
	}

	var movie Movie
	err = attributevalue.UnmarshalMap(result.Item, &movie)
	if err != nil {
		return nil, err
	}

	return &movie, nil
}

// Create provisions a newly submitted map structure within the database securely targeting missing IDs strictly avoiding overwrites!
func (d *DynamoMovieRepository) Create(ctx context.Context, movie Movie) error {
	item, err := attributevalue.MarshalMap(movie)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		TableName:           &d.table,
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(id)"),
	}

	_, err = d.client.PutItem(ctx, input)
	return err
}

// Delete strips records bound directly toward specified IDs validating existence implicitly upon processing.
func (d *DynamoMovieRepository) Delete(ctx context.Context, id string) error {
	input := &dynamodb.DeleteItemInput{
		TableName: &d.table,
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
		ConditionExpression: aws.String("attribute_exists(id)"),
	}

	_, err := d.client.DeleteItem(ctx, input)
	return err
}

// NewMovieRepository initializes structural pointers constructing a workable MovieRepository interface bridging explicit Amazon contexts perfectly.
func NewMovieRepository(client *dynamodb.Client, table string) MovieRepository {
	return &DynamoMovieRepository{
		client: client,
		table:  table,
	}
}
