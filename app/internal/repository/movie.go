package repository

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"sort"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/xanderbilla/bi8s-go/internal/model"
)

type MovieRepository interface {
	GetAllAdmin(ctx context.Context) ([]model.Movie, error)
	GetRecentContent(ctx context.Context, contentTypeFilter string) ([]model.Movie, error)
	Get(ctx context.Context, id string) (*model.Movie, error)
	GetAdmin(ctx context.Context, id string) (*model.Movie, error)
	Create(ctx context.Context, movie model.Movie) error
	Update(ctx context.Context, movie model.Movie) error
	Delete(ctx context.Context, id string) error
	GetMoviesByPersonId(ctx context.Context, personId string) ([]model.Movie, error)
	GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error)
	GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error)
	GetMoviesByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string) ([]model.Movie, error)
	GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error)
	GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string) ([]model.Movie, error)
}

type DynamoMovieRepository struct {
	*BaseRepository
}

func NewMovieRepository(client *dynamodb.Client, tableName string) MovieRepository {
	return &DynamoMovieRepository{
		BaseRepository: NewBaseRepository(client, tableName),
	}
}

func (d *DynamoMovieRepository) scanAll(ctx context.Context, input *dynamodb.ScanInput) ([]map[string]types.AttributeValue, error) {
	return WithTimeoutResult(ctx, "scan_all", func(ctx context.Context) ([]map[string]types.AttributeValue, error) {
		return ScanAllPaged(ctx, d.GetClient(), input, DefaultMaxScanPages)
	})
}

func publicVisibilityValues() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		":visibility":   &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
		":released":     &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
		":inProduction": &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
		":ended":        &types.AttributeValueMemberS{Value: string(model.StatusEnded)},
	}
}

func applyContentTypeFilter(baseExpr string, filter string, vals map[string]types.AttributeValue) string {
	ct, ok := model.ParseContentType(filter)
	if !ok {
		return baseExpr
	}
	vals[":contentType"] = &types.AttributeValueMemberS{Value: string(ct)}
	return baseExpr + " AND contentType = :contentType"
}

func sortByReleaseDateDesc(movies []model.Movie) {
	sort.Slice(movies, func(i, j int) bool {
		a, b := movies[i].ReleaseDate, movies[j].ReleaseDate
		if a == "" {
			return false
		}
		if b == "" {
			return true
		}
		return a > b
	})
}

func sortByPopularity(movies []model.Movie) {
	sort.SliceStable(movies, func(i, j int) bool {
		a, b := movies[i].Stats, movies[j].Stats
		if a.AverageRating != b.AverageRating {
			return a.AverageRating > b.AverageRating
		}
		if a.TotalViews != b.TotalViews {
			return a.TotalViews > b.TotalViews
		}
		return movies[i].ReleaseDate > movies[j].ReleaseDate
	})
}

func sortByTrending(movies []model.Movie) {
	sort.SliceStable(movies, func(i, j int) bool {
		a, b := movies[i].Stats, movies[j].Stats
		if a.TotalViews != b.TotalViews {
			return a.TotalViews > b.TotalViews
		}
		return movies[i].Audit.CreatedAt.After(movies[j].Audit.CreatedAt)
	})
}

func (d *DynamoMovieRepository) GetAllAdmin(ctx context.Context) ([]model.Movie, error) {
	items, err := d.scanAll(ctx, &dynamodb.ScanInput{TableName: aws.String(d.GetTableName())})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []model.Movie{}, nil
	}
	var movies []model.Movie
	if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
		return nil, err
	}
	return movies, nil
}

func (d *DynamoMovieRepository) GetRecentContent(ctx context.Context, contentTypeFilter string) ([]model.Movie, error) {
	vals := publicVisibilityValues()
	expr := applyContentTypeFilter(publicVisibleFilter, contentTypeFilter, vals)

	items, err := d.scanAll(ctx, &dynamodb.ScanInput{
		TableName:                 aws.String(d.GetTableName()),
		FilterExpression:          aws.String(expr),
		ExpressionAttributeNames:  map[string]string{"#status": "status"},
		ExpressionAttributeValues: vals,
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []model.Movie{}, nil
	}
	var movies []model.Movie
	if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
		return nil, err
	}
	sort.Slice(movies, func(i, j int) bool {
		return movies[i].Audit.CreatedAt.After(movies[j].Audit.CreatedAt)
	})
	return movies, nil
}

func (d *DynamoMovieRepository) Get(ctx context.Context, id string) (*model.Movie, error) {
	return WithTimeoutResult(ctx, "get_movie", func(ctx context.Context) (*model.Movie, error) {
		result, err := d.GetClient().GetItem(ctx, &dynamodb.GetItemInput{
			TableName:      aws.String(d.GetTableName()),
			Key:            map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}},
			ConsistentRead: aws.Bool(true),
		})
		if err != nil {
			return nil, err
		}
		if result.Item == nil {
			return nil, nil
		}
		var movie model.Movie
		if err := attributevalue.UnmarshalMap(result.Item, &movie); err != nil {
			return nil, err
		}
		if movie.Visibility != model.VisibilityPublic {
			return nil, nil
		}
		if movie.Status != model.StatusReleased && movie.Status != model.StatusInProduction && movie.Status != model.StatusEnded {
			return nil, nil
		}
		return &movie, nil
	})
}

func (d *DynamoMovieRepository) GetAdmin(ctx context.Context, id string) (*model.Movie, error) {
	return WithTimeoutResult(ctx, "get_movie_admin", func(ctx context.Context) (*model.Movie, error) {
		result, err := d.GetClient().GetItem(ctx, &dynamodb.GetItemInput{
			TableName:      aws.String(d.GetTableName()),
			Key:            map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}},
			ConsistentRead: aws.Bool(true),
		})
		if err != nil {
			return nil, err
		}
		if result.Item == nil {
			return nil, nil
		}
		var movie model.Movie
		if err := attributevalue.UnmarshalMap(result.Item, &movie); err != nil {
			return nil, err
		}
		return &movie, nil
	})
}

func (d *DynamoMovieRepository) Create(ctx context.Context, movie model.Movie) error {
	return d.WithTimeout(ctx, "create_movie", func(ctx context.Context) error {
		item, err := attributevalue.MarshalMap(movie)
		if err != nil {
			return err
		}
		_, err = d.GetClient().PutItem(ctx, &dynamodb.PutItemInput{
			TableName:           aws.String(d.GetTableName()),
			Item:                item,
			ConditionExpression: aws.String("attribute_not_exists(id)"),
		})
		return err
	})
}

func (d *DynamoMovieRepository) Update(ctx context.Context, movie model.Movie) error {
	return d.WithTimeout(ctx, "update_movie", func(ctx context.Context) error {
		oldVersion := movie.Audit.Version
		movie.Audit.Version++
		now := time.Now()
		movie.Audit.UpdatedAt = &now

		item, err := attributevalue.MarshalMap(movie)
		if err != nil {
			return err
		}

		_, err = d.GetClient().PutItem(ctx, &dynamodb.PutItemInput{
			TableName:           aws.String(d.GetTableName()),
			Item:                item,
			ConditionExpression: aws.String("attribute_exists(id) AND version = :expectedVersion"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":expectedVersion": &types.AttributeValueMemberN{
					Value: strconv.Itoa(oldVersion),
				},
			},
		})
		return err
	})
}

func (d *DynamoMovieRepository) Delete(ctx context.Context, id string) error {
	return d.WithTimeout(ctx, "delete_movie", func(ctx context.Context) error {
		_, err := d.GetClient().DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName:           aws.String(d.GetTableName()),
			Key:                 map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}},
			ConditionExpression: aws.String("attribute_exists(id)"),
		})
		return err
	})
}

func (d *DynamoMovieRepository) GetMoviesByPersonId(ctx context.Context, personId string) ([]model.Movie, error) {
	vals := publicVisibilityValues()
	vals[":personId"] = &types.AttributeValueMemberS{Value: personId}

	items, err := d.scanAll(ctx, &dynamodb.ScanInput{
		TableName:                 aws.String(d.GetTableName()),
		FilterExpression:          aws.String(publicVisibleByCastFilter),
		ExpressionAttributeNames:  map[string]string{"#status": "status"},
		ExpressionAttributeValues: vals,
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []model.Movie{}, nil
	}
	var movies []model.Movie
	if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
		return nil, err
	}
	return movies, nil
}

func (d *DynamoMovieRepository) GetContentByPersonId(ctx context.Context, personId, contentTypeFilter string) ([]model.Movie, error) {
	vals := publicVisibilityValues()
	vals[":personId"] = &types.AttributeValueMemberS{Value: personId}
	expr := applyContentTypeFilter(publicVisibleByCastFilter, contentTypeFilter, vals)

	items, err := d.scanAll(ctx, &dynamodb.ScanInput{
		TableName:                 aws.String(d.GetTableName()),
		FilterExpression:          aws.String(expr),
		ExpressionAttributeNames:  map[string]string{"#status": "status"},
		ExpressionAttributeValues: vals,
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []model.Movie{}, nil
	}
	var movies []model.Movie
	if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
		return nil, err
	}
	sortByReleaseDateDesc(movies)
	return movies, nil
}

func (d *DynamoMovieRepository) GetContentByPersonIdAdmin(ctx context.Context, personId, contentTypeFilter string) ([]model.Movie, error) {
	vals := map[string]types.AttributeValue{
		":personId": &types.AttributeValueMemberS{Value: personId},
	}
	expr := applyContentTypeFilter("contains(castIds, :personId)", contentTypeFilter, vals)

	items, err := d.scanAll(ctx, &dynamodb.ScanInput{
		TableName:                 aws.String(d.GetTableName()),
		FilterExpression:          aws.String(expr),
		ExpressionAttributeValues: vals,
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []model.Movie{}, nil
	}
	var movies []model.Movie
	if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
		return nil, err
	}
	sortByReleaseDateDesc(movies)
	return movies, nil
}

func (d *DynamoMovieRepository) GetMoviesByAttributeId(ctx context.Context, attributeId, contentTypeFilter string) ([]model.Movie, error) {
	vals := publicVisibilityValues()
	vals[":attributeId"] = &types.AttributeValueMemberS{Value: attributeId}
	expr := applyContentTypeFilter(publicVisibleByAttributeFilter, contentTypeFilter, vals)

	items, err := d.scanAll(ctx, &dynamodb.ScanInput{
		TableName:                 aws.String(d.GetTableName()),
		FilterExpression:          aws.String(expr),
		ExpressionAttributeNames:  map[string]string{"#status": "status"},
		ExpressionAttributeValues: vals,
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []model.Movie{}, nil
	}
	var movies []model.Movie
	if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
		return nil, err
	}
	return movies, nil
}

func (d *DynamoMovieRepository) GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error) {
	vals := publicVisibilityValues()
	expr := applyContentTypeFilter(publicVisibleFilter, contentTypeFilter, vals)

	items, err := d.scanAll(ctx, &dynamodb.ScanInput{
		TableName:                 aws.String(d.GetTableName()),
		FilterExpression:          aws.String(expr),
		ExpressionAttributeNames:  map[string]string{"#status": "status"},
		ExpressionAttributeValues: vals,
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	var movies []model.Movie
	if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
		return nil, err
	}
	idx, err := secureRandIndex(len(movies))
	if err != nil {

		idx = int(time.Now().UnixNano()) % len(movies)
	}
	return &movies[idx], nil
}

func (d *DynamoMovieRepository) GetDiscoverContent(ctx context.Context, discoverType, contentTypeFilter string) ([]model.Movie, error) {
	vals := publicVisibilityValues()
	expr := applyContentTypeFilter(publicVisibleFilter, contentTypeFilter, vals)

	items, err := d.scanAll(ctx, &dynamodb.ScanInput{
		TableName:                 aws.String(d.GetTableName()),
		FilterExpression:          aws.String(expr),
		ExpressionAttributeNames:  map[string]string{"#status": "status"},
		ExpressionAttributeValues: vals,
	})
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return []model.Movie{}, nil
	}
	var movies []model.Movie
	if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
		return nil, err
	}

	switch discoverType {
	case "popular":
		sortByPopularity(movies)
	case "trending":
		sortByTrending(movies)
	default:
		sortByReleaseDateDesc(movies)
	}
	return movies, nil
}

func secureRandIndex(max int) (int, error) {
	if max <= 1 {
		return 0, nil
	}
	var n uint64
	if err := binary.Read(rand.Reader, binary.LittleEndian, &n); err != nil {
		return 0, err
	}
	return int(n % uint64(max)), nil
}
