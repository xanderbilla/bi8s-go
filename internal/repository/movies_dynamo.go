package repository

import (
	"context"
	"log"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

// Movie represents a single movie record as stored in DynamoDB and returned by the API.
type Movie struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Year  int    `json:"year"`
}

// MovieRepository defines the contract for movie data access.
// Anything that satisfies this interface can be swapped in — DynamoDB today, Postgres tomorrow.
type MovieRepository interface {
	GetAll(ctx context.Context) ([]Movie, error)
	Get(ctx context.Context, id string) (*Movie, error)
	Create(ctx context.Context, movie Movie) error
	Delete(ctx context.Context, id string) error
}

// DynamoMovieRepository is the DynamoDB-backed implementation of MovieRepository.
type DynamoMovieRepository struct {
	client *dynamodb.Client
	table  string // the DynamoDB table name to read/write from
}

// Create is not yet implemented.
func (d *DynamoMovieRepository) Create(ctx context.Context, movie Movie) error {
	item, err := attributevalue.MarshalMap(movie)
	if err != nil {
		return err
	}

	input := &dynamodb.PutItemInput{
		TableName: &d.table,
		Item:      item,
	}

	_, err = d.client.PutItem(ctx, input)
	if err != nil {
		log.Printf("Couldn't add item to table. Here's why: %v\n", err)
	}
	return err

}

// Delete is not yet implemented.
func (d *DynamoMovieRepository) Delete(ctx context.Context, id string) error {
	panic("unimplemented")
}

// Get is not yet implemented.
func (d *DynamoMovieRepository) Get(ctx context.Context, id string) (*Movie, error) {
	panic("unimplemented")
}

// GetAll does a full table scan and returns every movie in the table.
// Note: Scan reads the entire table — fine for small datasets, but consider
// using Query with an index for large tables to keep costs and latency down.
func (d *DynamoMovieRepository) GetAll(ctx context.Context) ([]Movie, error) {

	input := &dynamodb.ScanInput{
		TableName: &d.table,
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	var movies []Movie

	// DynamoDB returns items as attribute maps — unmarshal them into our Movie structs.
	err = attributevalue.UnmarshalListOfMaps(result.Items, &movies)
	if err != nil {
		return nil, err
	}

	return movies, nil
}

// NewMovieRepository creates a DynamoDB-backed MovieRepository for the given table.
func NewMovieRepository(client *dynamodb.Client, table string) MovieRepository {
	return &DynamoMovieRepository{
		client: client,
		table:  table,
	}
}
