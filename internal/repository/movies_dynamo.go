package repository

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// Movie is the data structure that represents a movie in our app.
// Each field has two struct tags:
//   - `json` controls the key name in the HTTP response body.
//   - `dynamodbav` controls the attribute name in DynamoDB.
//
// Both are set to lowercase so they match consistently everywhere.
type Movie struct {
	ID    string `json:"id" dynamodbav:"id"`
	Title string `json:"title" dynamodbav:"title"`
	Year  int    `json:"year" dynamodbav:"year"`
}

// MovieRepository is an interface (a contract) that describes what operations
// the database layer must support. By using an interface, we can swap the
// underlying database (e.g. from DynamoDB to PostgreSQL) without touching
// any handler or service code — only this layer changes.
type MovieRepository interface {
	GetAll(ctx context.Context) ([]Movie, error)
	Get(ctx context.Context, id string) (*Movie, error)
	Create(ctx context.Context, movie Movie) error
	Delete(ctx context.Context, id string) error
}

// DynamoMovieRepository is our concrete implementation of MovieRepository.
// It holds a DynamoDB client and the table name it should read/write from.
type DynamoMovieRepository struct {
	client *dynamodb.Client
	table  string // name of the DynamoDB table, e.g. "bi8s-dev"
}

// GetAll fetches every movie in the table using a Scan operation.
// A Scan reads the entire table from top to bottom, which is fine for small datasets.
// TODO: Replace with a Query using a partition key — Scan has a 1MB limit per call
// and gets expensive on large tables. Use ExclusiveStartKey + LastEvaluatedKey for pagination.
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

	// DynamoDB returns data as a list of attribute maps, not plain Go structs.
	// UnmarshalListOfMaps converts each map back into a Movie struct for us.
	err = attributevalue.UnmarshalListOfMaps(result.Items, &movies)
	if err != nil {
		return nil, err
	}

	return movies, nil
}

// Get looks up a single movie by its ID.
// We use ConsistentRead: true so we always get the latest version of the item,
// not a potentially stale cached copy. If no movie is found, we return nil with no error.
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

	// No movie found with that ID — return nil instead of an error
	// so the caller can distinguish "not found" from an actual failure.
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

// Create saves a new movie to DynamoDB.
// We first convert the Movie struct into the format DynamoDB expects (a map of attributes),
// then call PutItem to write it. The condition expression makes sure we never accidentally
// overwrite a movie that already has the same ID.
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

// Delete removes a movie from DynamoDB by its ID.
// It uses a condition expression to make sure the movie actually exists before trying to delete it.
// If the movie doesn't exist, DynamoDB will return an error instead of silently doing nothing.
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

// NewMovieRepository wires up and returns a DynamoDB-backed MovieRepository.
// Call this once at startup and pass the result into the service layer.
func NewMovieRepository(client *dynamodb.Client, table string) MovieRepository {
	return &DynamoMovieRepository{
		client: client,
		table:  table,
	}
}
