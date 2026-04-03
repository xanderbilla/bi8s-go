// Package repository contains all database access structures.
// Separating database logic natively creates a scalable clean architecture
// enabling swapouts without polluting handler/service tiers.
package repository

import (
	"context"
	"crypto/rand"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

// MovieRepository signifies a data-agnostic boundary.
// It serves as the single source of truth for Movie operations avoiding tight couplings towards DynamoDB explicitly.
type MovieRepository interface {
	GetAllAdmin(ctx context.Context) ([]model.Movie, error)
	GetRecentContent(ctx context.Context, contentTypeFilter string) ([]model.Movie, error)
	Get(ctx context.Context, id string) (*model.Movie, error)
	GetAdmin(ctx context.Context, id string) (*model.Movie, error)
	Create(ctx context.Context, movie model.Movie) error
	Delete(ctx context.Context, id string) error
	GetMoviesByPersonId(ctx context.Context, personId string) ([]model.Movie, error)
	GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error)
	GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error)
	GetMoviesByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string) ([]model.Movie, error)
	GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error)
	GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string) ([]model.Movie, error)
}

// DynamoMovieRepository fulfills the repository boundary manipulating datasets utilizing the AWS DynamoDB SDK v2.
type DynamoMovieRepository struct {
	client *dynamodb.Client
	table  string
}

// GetAllAdmin returns all movies without any filtering (for admin use).
// Returns all movies regardless of visibility or status.
func (d *DynamoMovieRepository) GetAllAdmin(ctx context.Context) ([]model.Movie, error) {
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

	var movies []model.Movie
	err = attributevalue.UnmarshalListOfMaps(result.Items, &movies)
	if err != nil {
		return nil, err
	}

	return movies, nil
}

// GetRecentContent returns content filtered by type and sorted by creation date (most recent first).
// Only returns movies with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.
// contentTypeFilter can be "all", "movie", or "tv" (case-insensitive).
func (d *DynamoMovieRepository) GetRecentContent(ctx context.Context, contentTypeFilter string) ([]model.Movie, error) {
	var filterExpression string
	expressionAttributeValues := map[string]types.AttributeValue{
		":visibility":   &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
		":released":     &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
		":inProduction": &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
	}

	// Convert contentTypeFilter to uppercase for comparison
	if contentTypeFilter == "movie" {
		filterExpression = "visibility = :visibility AND (#status = :released OR #status = :inProduction) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "MOVIE"}
	} else if contentTypeFilter == "tv" {
		filterExpression = "visibility = :visibility AND (#status = :released OR #status = :inProduction) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "TV"}
	} else {
		// "all" or any other value - no contentType filter
		filterExpression = "visibility = :visibility AND (#status = :released OR #status = :inProduction)"
	}

	input := &dynamodb.ScanInput{
		TableName:        &d.table,
		FilterExpression: aws.String(filterExpression),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: expressionAttributeValues,
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Count == 0 {
		return nil, nil
	}

	var movies []model.Movie
	err = attributevalue.UnmarshalListOfMaps(result.Items, &movies)
	if err != nil {
		return nil, err
	}

	// Sort by creation date (most recent first)
	sort.Slice(movies, func(i, j int) bool {
		return movies[i].Audit.CreatedAt.After(movies[j].Audit.CreatedAt)
	})

	return movies, nil
}

// Get extracts singular records constrained entirely by specific unique partition identifers ensuring maximum speeds.
// Only returns movie if visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.
func (d *DynamoMovieRepository) Get(ctx context.Context, id string) (*model.Movie, error) {
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

	var movie model.Movie
	err = attributevalue.UnmarshalMap(result.Item, &movie)
	if err != nil {
		return nil, err
	}

	// Filter by visibility and status
	if movie.Visibility != model.VisibilityPublic {
		return nil, nil
	}
	if movie.Status != model.StatusReleased && movie.Status != model.StatusInProduction {
		return nil, nil
	}

	return &movie, nil
}

// GetAdmin extracts a single movie by ID without any filtering (for admin use).
// Returns the movie regardless of visibility or status.
func (d *DynamoMovieRepository) GetAdmin(ctx context.Context, id string) (*model.Movie, error) {
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

	var movie model.Movie
	err = attributevalue.UnmarshalMap(result.Item, &movie)
	if err != nil {
		return nil, err
	}

	return &movie, nil
}

// Create provisions a newly submitted map structure within the database securely targeting missing IDs strictly avoiding overwrites!
func (d *DynamoMovieRepository) Create(ctx context.Context, movie model.Movie) error {
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

// GetMoviesByPersonId returns all movies where the person is in the cast.
// Only returns movies with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.
// Uses Scan with FilterExpression to check if personId exists in castIds array.
// Note: For large datasets, consider using a separate movie-cast relationship table.
func (d *DynamoMovieRepository) GetMoviesByPersonId(ctx context.Context, personId string) ([]model.Movie, error) {
	input := &dynamodb.ScanInput{
		TableName:        aws.String(d.table),
		FilterExpression: aws.String("contains(castIds, :personId) AND visibility = :visibility AND (#status = :released OR #status = :inProduction)"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":personId":     &types.AttributeValueMemberS{Value: personId},
			":visibility":   &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
			":released":     &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
			":inProduction": &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
		},
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Count == 0 {
		return []model.Movie{}, nil
	}

	var movies []model.Movie
	err = attributevalue.UnmarshalListOfMaps(result.Items, &movies)
	if err != nil {
		return nil, err
	}

	return movies, nil
}

// GetContentByPersonId returns all content where the person is in the cast, filtered by content type.
// Only returns content with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.
// contentTypeFilter can be "all", "movie", or "tv" (case-insensitive).
// Results are sorted by release date (most recent first).
func (d *DynamoMovieRepository) GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error) {
	var filterExpression string
	expressionAttributeValues := map[string]types.AttributeValue{
		":personId":     &types.AttributeValueMemberS{Value: personId},
		":visibility":   &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
		":released":     &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
		":inProduction": &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
	}

	// Build filter expression based on content type
	if contentTypeFilter == "movie" {
		filterExpression = "contains(castIds, :personId) AND visibility = :visibility AND (#status = :released OR #status = :inProduction) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "MOVIE"}
	} else if contentTypeFilter == "tv" {
		filterExpression = "contains(castIds, :personId) AND visibility = :visibility AND (#status = :released OR #status = :inProduction) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "TV"}
	} else {
		// "all" or any other value - no contentType filter
		filterExpression = "contains(castIds, :personId) AND visibility = :visibility AND (#status = :released OR #status = :inProduction)"
	}

	input := &dynamodb.ScanInput{
		TableName:        aws.String(d.table),
		FilterExpression: aws.String(filterExpression),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: expressionAttributeValues,
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Count == 0 {
		return []model.Movie{}, nil
	}

	var movies []model.Movie
	err = attributevalue.UnmarshalListOfMaps(result.Items, &movies)
	if err != nil {
		return nil, err
	}

	// Sort by release date (most recent first)
	sort.Slice(movies, func(i, j int) bool {
		// Handle empty release dates
		if movies[i].ReleaseDate == "" && movies[j].ReleaseDate == "" {
			return false
		}
		if movies[i].ReleaseDate == "" {
			return false
		}
		if movies[j].ReleaseDate == "" {
			return true
		}
		return movies[i].ReleaseDate > movies[j].ReleaseDate
	})

	return movies, nil
}

// GetContentByPersonIdAdmin returns all content where the person is in the cast, filtered by content type.
// Admin endpoint - returns all content regardless of visibility or status.
// contentTypeFilter can be "all", "movie", or "tv" (case-insensitive).
// Results are sorted by release date (most recent first).
func (d *DynamoMovieRepository) GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error) {
	var filterExpression string
	expressionAttributeValues := map[string]types.AttributeValue{
		":personId": &types.AttributeValueMemberS{Value: personId},
	}

	// Build filter expression based on content type
	if contentTypeFilter == "movie" {
		filterExpression = "contains(castIds, :personId) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "MOVIE"}
	} else if contentTypeFilter == "tv" {
		filterExpression = "contains(castIds, :personId) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "TV"}
	} else {
		// "all" or any other value - no contentType filter
		filterExpression = "contains(castIds, :personId)"
	}

	input := &dynamodb.ScanInput{
		TableName:                 aws.String(d.table),
		FilterExpression:          aws.String(filterExpression),
		ExpressionAttributeValues: expressionAttributeValues,
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Count == 0 {
		return []model.Movie{}, nil
	}

	var movies []model.Movie
	err = attributevalue.UnmarshalListOfMaps(result.Items, &movies)
	if err != nil {
		return nil, err
	}

	// Sort by release date (most recent first)
	sort.Slice(movies, func(i, j int) bool {
		// Handle empty release dates
		if movies[i].ReleaseDate == "" && movies[j].ReleaseDate == "" {
			return false
		}
		if movies[i].ReleaseDate == "" {
			return false
		}
		if movies[j].ReleaseDate == "" {
			return true
		}
		return movies[i].ReleaseDate > movies[j].ReleaseDate
	})

	return movies, nil
}

// GetMoviesByAttributeId returns all movies that have the specified attribute ID.
// Only returns movies with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.
// contentTypeFilter can be "all", "movie", or "tv" (case-insensitive).
// This checks genres, tags, and moodTags using the attributeIds array.
// Uses Scan with FilterExpression to check if attributeId exists in attributeIds array.
func (d *DynamoMovieRepository) GetMoviesByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string) ([]model.Movie, error) {
	var filterExpression string
	expressionAttributeValues := map[string]types.AttributeValue{
		":attributeId":  &types.AttributeValueMemberS{Value: attributeId},
		":visibility":   &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
		":released":     &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
		":inProduction": &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
	}

	// Build filter expression based on content type
	if contentTypeFilter == "movie" {
		filterExpression = "contains(attributeIds, :attributeId) AND visibility = :visibility AND (#status = :released OR #status = :inProduction) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "MOVIE"}
	} else if contentTypeFilter == "tv" {
		filterExpression = "contains(attributeIds, :attributeId) AND visibility = :visibility AND (#status = :released OR #status = :inProduction) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "TV"}
	} else {
		// "all" or any other value - no contentType filter
		filterExpression = "contains(attributeIds, :attributeId) AND visibility = :visibility AND (#status = :released OR #status = :inProduction)"
	}

	input := &dynamodb.ScanInput{
		TableName:        aws.String(d.table),
		FilterExpression: aws.String(filterExpression),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: expressionAttributeValues,
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Count == 0 {
		return []model.Movie{}, nil
	}

	var movies []model.Movie
	err = attributevalue.UnmarshalListOfMaps(result.Items, &movies)
	if err != nil {
		return nil, err
	}

	return movies, nil
}

// GetBanner returns a random banner content with optional contentType filter.
// Only returns movies with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.
// If contentTypeFilter is "movie" or "tv", filters by that contentType. If "all" or empty, returns from all content.
func (d *DynamoMovieRepository) GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error) {
	var filterExpression string
	expressionAttributeValues := map[string]types.AttributeValue{
		":visibility":   &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
		":released":     &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
		":inProduction": &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
	}

	// Convert contentTypeFilter to uppercase for comparison
	if contentTypeFilter == "movie" {
		filterExpression = "visibility = :visibility AND (#status = :released OR #status = :inProduction) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "MOVIE"}
	} else if contentTypeFilter == "tv" {
		filterExpression = "visibility = :visibility AND (#status = :released OR #status = :inProduction) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "TV"}
	} else {
		// "all" or any other value - no contentType filter
		filterExpression = "visibility = :visibility AND (#status = :released OR #status = :inProduction)"
	}

	input := &dynamodb.ScanInput{
		TableName:        aws.String(d.table),
		FilterExpression: aws.String(filterExpression),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: expressionAttributeValues,
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Count == 0 {
		return nil, nil
	}

	var movies []model.Movie
	err = attributevalue.UnmarshalListOfMaps(result.Items, &movies)
	if err != nil {
		return nil, err
	}

	// Return a random movie from the results
	if len(movies) > 0 {
		// Use crypto/rand for secure random selection
		randomIndex, err := generateSecureRandomIndex(len(movies))
		if err != nil {
			// Fallback to time-based seed if crypto/rand fails
			randomIndex = int(time.Now().UnixNano()) % len(movies)
		}
		return &movies[randomIndex], nil
	}

	return nil, nil
}

// generateSecureRandomIndex generates a cryptographically secure random index
func generateSecureRandomIndex(max int) (int, error) {
	if max <= 0 {
		return 0, nil
	}

	// Generate random bytes
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return 0, err
	}

	// Convert bytes to uint64
	randomNum := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56

	return int(randomNum % uint64(max)), nil
}

// GetDiscoverContent returns content for discovery based on type (latest, popular, trending).
// Only returns movies with visibility=PUBLIC and status=RELEASED or IN_PRODUCTION.
// discoverType can be "latest" (sorted by release date), "popular" (TODO), or "trending" (TODO).
// contentTypeFilter can be "all", "movie", or "tv" (case-insensitive).
func (d *DynamoMovieRepository) GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string) ([]model.Movie, error) {
	var filterExpression string
	expressionAttributeValues := map[string]types.AttributeValue{
		":visibility":   &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
		":released":     &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
		":inProduction": &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
	}

	// Build filter expression based on content type
	if contentTypeFilter == "movie" {
		filterExpression = "visibility = :visibility AND (#status = :released OR #status = :inProduction) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "MOVIE"}
	} else if contentTypeFilter == "tv" {
		filterExpression = "visibility = :visibility AND (#status = :released OR #status = :inProduction) AND contentType = :contentType"
		expressionAttributeValues[":contentType"] = &types.AttributeValueMemberS{Value: "TV"}
	} else {
		// "all" or any other value - no contentType filter
		filterExpression = "visibility = :visibility AND (#status = :released OR #status = :inProduction)"
	}

	input := &dynamodb.ScanInput{
		TableName:        &d.table,
		FilterExpression: aws.String(filterExpression),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: expressionAttributeValues,
	}

	result, err := d.client.Scan(ctx, input)
	if err != nil {
		return nil, err
	}

	if result.Count == 0 {
		return []model.Movie{}, nil
	}

	var movies []model.Movie
	err = attributevalue.UnmarshalListOfMaps(result.Items, &movies)
	if err != nil {
		return nil, err
	}

	// Sort based on discover type
	switch discoverType {
	case "latest":
		// Sort by release date (most recent first)
		sort.Slice(movies, func(i, j int) bool {
			// Handle empty release dates
			if movies[i].ReleaseDate == "" && movies[j].ReleaseDate == "" {
				return false
			}
			if movies[i].ReleaseDate == "" {
				return false
			}
			if movies[j].ReleaseDate == "" {
				return true
			}
			return movies[i].ReleaseDate > movies[j].ReleaseDate
		})
	case "popular":
		// TODO: Sort by popularity (stats.totalViews or stats.averageRating)
		// For now, return as-is
	case "trending":
		// TODO: Sort by trending score (combination of recent views, likes, etc.)
		// For now, return as-is
	default:
		// Default to latest
		sort.Slice(movies, func(i, j int) bool {
			if movies[i].ReleaseDate == "" && movies[j].ReleaseDate == "" {
				return false
			}
			if movies[i].ReleaseDate == "" {
				return false
			}
			if movies[j].ReleaseDate == "" {
				return true
			}
			return movies[i].ReleaseDate > movies[j].ReleaseDate
		})
	}

	return movies, nil
}

// NewMovieRepository initializes structural pointers constructing a workable MovieRepository interface bridging explicit Amazon contexts perfectly.
func NewMovieRepository(client *dynamodb.Client, table string) MovieRepository {
	return &DynamoMovieRepository{
		client: client,
		table:  table,
	}
}
