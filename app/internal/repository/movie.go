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
	GetAllAdmin(ctx context.Context, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error)
	GetRecentContent(ctx context.Context, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error)
	Get(ctx context.Context, id string) (*model.Movie, error)
	GetAdmin(ctx context.Context, id string) (*model.Movie, error)
	Create(ctx context.Context, movie model.Movie) error
	Update(ctx context.Context, movie model.Movie) error
	Delete(ctx context.Context, id string) error
	// GetMoviesByPersonId is kept for internal/non-paginated use.
	GetMoviesByPersonId(ctx context.Context, personId string) ([]model.Movie, error)
	GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error)
	GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error)
	GetMoviesByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error)
	GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error)
	GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error)
}

type DynamoMovieRepository struct {
	*BaseRepository
	visibilityCreatedAtIndex   string
	visibilityContentTypeIndex string
	contentCastRepo            ContentCastRepository
	contentAttributeRepo       ContentAttributeRepository
}

func NewMovieRepository(
	client *dynamodb.Client,
	tableName string,
	visibilityCreatedAtIndex string,
	visibilityContentTypeIndex string,
	contentCastRepo ContentCastRepository,
	contentAttributeRepo ContentAttributeRepository,
) MovieRepository {
	return &DynamoMovieRepository{
		BaseRepository:             NewBaseRepository(client, tableName),
		visibilityCreatedAtIndex:   visibilityCreatedAtIndex,
		visibilityContentTypeIndex: visibilityContentTypeIndex,
		contentCastRepo:            contentCastRepo,
		contentAttributeRepo:       contentAttributeRepo,
	}
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

func defaultLimit(limit int32) int32 {
	if limit <= 0 {
		return 20
	}
	return limit
}

// GetAllAdmin returns a single page of all content items (admin view).
func (d *DynamoMovieRepository) GetAllAdmin(ctx context.Context, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	return WithTimeoutResultPage(ctx, "get_all_admin", func(ctx context.Context) ([]model.Movie, map[string]types.AttributeValue, error) {
		items, nextKey, err := ScanPage(ctx, d.GetClient(), &dynamodb.ScanInput{
			TableName:         aws.String(d.GetTableName()),
			Limit:             aws.Int32(defaultLimit(limit)),
			ExclusiveStartKey: startKey,
		})
		if err != nil {
			return nil, nil, err
		}
		if len(items) == 0 {
			return []model.Movie{}, nil, nil
		}
		var movies []model.Movie
		if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
			return nil, nil, err
		}
		return movies, nextKey, nil
	})
}

// GetRecentContent queries the visibility-createdAt-index GSI to return public content
// sorted by creation date descending.
func (d *DynamoMovieRepository) GetRecentContent(ctx context.Context, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	return WithTimeoutResultPage(ctx, "get_recent_content", func(ctx context.Context) ([]model.Movie, map[string]types.AttributeValue, error) {
		vals := map[string]types.AttributeValue{
			":visibility":   &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
			":released":     &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
			":inProduction": &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
			":ended":        &types.AttributeValueMemberS{Value: string(model.StatusEnded)},
		}
		filterExpr := "(#status = :released OR #status = :inProduction OR #status = :ended)"
		filterExpr = applyContentTypeFilter(filterExpr, contentTypeFilter, vals)

		items, nextKey, err := QueryPage(ctx, d.GetClient(), &dynamodb.QueryInput{
			TableName:                 aws.String(d.GetTableName()),
			IndexName:                 aws.String(d.visibilityCreatedAtIndex),
			KeyConditionExpression:    aws.String("visibility = :visibility"),
			FilterExpression:          aws.String(filterExpr),
			ExpressionAttributeNames:  map[string]string{"#status": "status"},
			ExpressionAttributeValues: vals,
			ScanIndexForward:          aws.Bool(false),
			Limit:                     aws.Int32(defaultLimit(limit)),
			ExclusiveStartKey:         startKey,
		})
		if err != nil {
			return nil, nil, err
		}
		if len(items) == 0 {
			return []model.Movie{}, nil, nil
		}
		var movies []model.Movie
		if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
			return nil, nil, err
		}
		return movies, nextKey, nil
	})
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
		if err != nil {
			return err
		}
		if err := d.syncContentCastEntries(ctx, movie.ID, movie.ContentType, movie.Visibility, movie.CastIds); err != nil {
			return err
		}
		return d.syncContentAttributeEntries(ctx, movie.ID, movie.ContentType, movie.Visibility, movie.AttributeIds)
	})
}

func (d *DynamoMovieRepository) Update(ctx context.Context, movie model.Movie) error {
	return d.WithTimeout(ctx, "update_movie", func(ctx context.Context) error {
		existing, err := d.GetAdmin(ctx, movie.ID)
		if err != nil {
			return err
		}

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
		if err != nil {
			return err
		}

		if existing != nil {
			if err := d.contentCastRepo.DeleteAllByContentId(ctx, movie.ID, existing.CastIds); err != nil {
				return err
			}
			if err := d.contentAttributeRepo.DeleteAllByContentId(ctx, movie.ID, existing.AttributeIds); err != nil {
				return err
			}
		}

		if err := d.syncContentCastEntries(ctx, movie.ID, movie.ContentType, movie.Visibility, movie.CastIds); err != nil {
			return err
		}
		return d.syncContentAttributeEntries(ctx, movie.ID, movie.ContentType, movie.Visibility, movie.AttributeIds)
	})
}

func (d *DynamoMovieRepository) Delete(ctx context.Context, id string) error {
	return d.WithTimeout(ctx, "delete_movie", func(ctx context.Context) error {
		existing, err := d.GetAdmin(ctx, id)
		if err != nil {
			return err
		}

		_, err = d.GetClient().DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName:           aws.String(d.GetTableName()),
			Key:                 map[string]types.AttributeValue{"id": &types.AttributeValueMemberS{Value: id}},
			ConditionExpression: aws.String("attribute_exists(id)"),
		})
		if err != nil {
			return err
		}

		if existing != nil {
			if err := d.contentCastRepo.DeleteAllByContentId(ctx, id, existing.CastIds); err != nil {
				return err
			}
			if err := d.contentAttributeRepo.DeleteAllByContentId(ctx, id, existing.AttributeIds); err != nil {
				return err
			}
		}

		return nil
	})
}

// GetMoviesByPersonId is a non-paginated scan kept for internal use.
func (d *DynamoMovieRepository) GetMoviesByPersonId(ctx context.Context, personId string) ([]model.Movie, error) {
	return WithTimeoutResult(ctx, "get_movies_by_person", func(ctx context.Context) ([]model.Movie, error) {
		vals := publicVisibilityValues()
		vals[":personId"] = &types.AttributeValueMemberS{Value: personId}

		items, err := ScanAllPaged(ctx, d.GetClient(), &dynamodb.ScanInput{
			TableName:                 aws.String(d.GetTableName()),
			FilterExpression:          aws.String(publicVisibleByCastFilter),
			ExpressionAttributeNames:  map[string]string{"#status": "status"},
			ExpressionAttributeValues: vals,
		}, DefaultMaxScanPages)
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
	})
}

// GetContentByPersonId queries the content_cast table for the person's contentIds, then
// fetches each content item and applies public visibility + status filter.
func (d *DynamoMovieRepository) GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	contentIds, nextKey, err := d.contentCastRepo.GetContentIdsByPersonId(ctx, personId, contentTypeFilter, defaultLimit(limit), startKey)
	if err != nil {
		return nil, nil, err
	}
	if len(contentIds) == 0 {
		return []model.Movie{}, nil, nil
	}

	movies := make([]model.Movie, 0, len(contentIds))
	for _, id := range contentIds {
		m, err := d.Get(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		if m != nil {
			movies = append(movies, *m)
		}
	}
	sortByReleaseDateDesc(movies)
	return movies, nextKey, nil
}

func (d *DynamoMovieRepository) syncContentCastEntries(ctx context.Context, contentID string, contentType model.ContentType, visibility model.Visibility, castIDs []string) error {
	for _, personID := range castIDs {
		if personID == "" {
			continue
		}
		if err := d.contentCastRepo.PutEntry(ctx, ContentCastEntry{
			PersonID:    personID,
			ContentID:   contentID,
			ContentType: string(contentType),
			Visibility:  string(visibility),
		}); err != nil {
			return err
		}
	}
	return nil
}

func (d *DynamoMovieRepository) syncContentAttributeEntries(ctx context.Context, contentID string, contentType model.ContentType, visibility model.Visibility, attributeIDs []string) error {
	for _, attributeID := range attributeIDs {
		if attributeID == "" {
			continue
		}
		if err := d.contentAttributeRepo.PutEntry(ctx, ContentAttributeEntry{
			AttributeID: attributeID,
			ContentID:   contentID,
			ContentType: string(contentType),
			Visibility:  string(visibility),
		}); err != nil {
			return err
		}
	}
	return nil
}

// GetContentByPersonIdAdmin scans the content table for items where castIds contains personId.
func (d *DynamoMovieRepository) GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	return WithTimeoutResultPage(ctx, "get_content_by_person_admin", func(ctx context.Context) ([]model.Movie, map[string]types.AttributeValue, error) {
		vals := map[string]types.AttributeValue{
			":personId": &types.AttributeValueMemberS{Value: personId},
		}
		expr := applyContentTypeFilter("contains(castIds, :personId)", contentTypeFilter, vals)

		items, nextKey, err := ScanPage(ctx, d.GetClient(), &dynamodb.ScanInput{
			TableName:                 aws.String(d.GetTableName()),
			FilterExpression:          aws.String(expr),
			ExpressionAttributeValues: vals,
			Limit:                     aws.Int32(defaultLimit(limit)),
			ExclusiveStartKey:         startKey,
		})
		if err != nil {
			return nil, nil, err
		}
		if len(items) == 0 {
			return []model.Movie{}, nil, nil
		}
		var movies []model.Movie
		if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
			return nil, nil, err
		}
		sortByReleaseDateDesc(movies)
		return movies, nextKey, nil
	})
}

// GetMoviesByAttributeId queries the content_attribute table for the attribute's contentIds, then
// fetches each content item and applies public visibility + status filter.
func (d *DynamoMovieRepository) GetMoviesByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	contentIds, nextKey, err := d.contentAttributeRepo.GetContentIdsByAttributeId(ctx, attributeId, contentTypeFilter, defaultLimit(limit), startKey)
	if err != nil {
		return nil, nil, err
	}
	if len(contentIds) == 0 {
		return []model.Movie{}, nil, nil
	}

	movies := make([]model.Movie, 0, len(contentIds))
	for _, id := range contentIds {
		m, err := d.Get(ctx, id)
		if err != nil {
			return nil, nil, err
		}
		if m != nil {
			movies = append(movies, *m)
		}
	}
	sortByReleaseDateDesc(movies)
	return movies, nextKey, nil
}

// GetBanner queries the visibility-createdAt-index GSI and picks a random public content item.
func (d *DynamoMovieRepository) GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error) {
	return WithTimeoutResult(ctx, "get_banner", func(ctx context.Context) (*model.Movie, error) {
		vals := map[string]types.AttributeValue{
			":visibility":   &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
			":released":     &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
			":inProduction": &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
			":ended":        &types.AttributeValueMemberS{Value: string(model.StatusEnded)},
		}
		filterExpr := "(#status = :released OR #status = :inProduction OR #status = :ended)"
		filterExpr = applyContentTypeFilter(filterExpr, contentTypeFilter, vals)

		items, _, err := QueryPage(ctx, d.GetClient(), &dynamodb.QueryInput{
			TableName:                 aws.String(d.GetTableName()),
			IndexName:                 aws.String(d.visibilityCreatedAtIndex),
			KeyConditionExpression:    aws.String("visibility = :visibility"),
			FilterExpression:          aws.String(filterExpr),
			ExpressionAttributeNames:  map[string]string{"#status": "status"},
			ExpressionAttributeValues: vals,
			ScanIndexForward:          aws.Bool(false),
			Limit:                     aws.Int32(50),
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
	})
}

// GetDiscoverContent queries the visibility-createdAt-index GSI and sorts in-memory by discover type.
func (d *DynamoMovieRepository) GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	return WithTimeoutResultPage(ctx, "get_discover_content", func(ctx context.Context) ([]model.Movie, map[string]types.AttributeValue, error) {
		vals := map[string]types.AttributeValue{
			":visibility":   &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
			":released":     &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
			":inProduction": &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
			":ended":        &types.AttributeValueMemberS{Value: string(model.StatusEnded)},
		}
		filterExpr := "(#status = :released OR #status = :inProduction OR #status = :ended)"
		filterExpr = applyContentTypeFilter(filterExpr, contentTypeFilter, vals)

		fetchLimit := defaultLimit(limit)
		if discoverType == "popular" || discoverType == "trending" {
			fetchLimit = 100
		}

		items, nextKey, err := QueryPage(ctx, d.GetClient(), &dynamodb.QueryInput{
			TableName:                 aws.String(d.GetTableName()),
			IndexName:                 aws.String(d.visibilityCreatedAtIndex),
			KeyConditionExpression:    aws.String("visibility = :visibility"),
			FilterExpression:          aws.String(filterExpr),
			ExpressionAttributeNames:  map[string]string{"#status": "status"},
			ExpressionAttributeValues: vals,
			ScanIndexForward:          aws.Bool(false),
			Limit:                     aws.Int32(fetchLimit),
			ExclusiveStartKey:         startKey,
		})
		if err != nil {
			return nil, nil, err
		}
		if len(items) == 0 {
			return []model.Movie{}, nil, nil
		}
		var movies []model.Movie
		if err := attributevalue.UnmarshalListOfMaps(items, &movies); err != nil {
			return nil, nil, err
		}

		switch discoverType {
		case "popular":
			sortByPopularity(movies)
		case "trending":
			sortByTrending(movies)
		}

		if (discoverType == "popular" || discoverType == "trending") && int(defaultLimit(limit)) < len(movies) {
			movies = movies[:defaultLimit(limit)]
			nextKey = nil
		}
		return movies, nextKey, nil
	})
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
