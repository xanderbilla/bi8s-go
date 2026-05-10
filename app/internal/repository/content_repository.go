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

type ContentRepository interface {
	GetAllAdmin(ctx context.Context, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error)
	Get(ctx context.Context, id string) (*model.Movie, error)
	GetAdmin(ctx context.Context, id string) (*model.Movie, error)
	Create(ctx context.Context, movie model.Movie) error
	Update(ctx context.Context, movie model.Movie) error
	Delete(ctx context.Context, id string) error

	GetContentByPersonIdSimple(ctx context.Context, personId string) ([]model.Movie, error)
	GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error)
	GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error)
	GetContentByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error)
	GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error)
	GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error)
}

type DynamoContentRepository struct {
	*BaseRepository
	visibilityCreatedAtIndex   string
	visibilityContentTypeIndex string
	visibilityReleaseDateIndex string
	contentCastRepo            ContentCastRepository
	contentAttributeRepo       ContentAttributeRepository
}

func NewContentRepository(
	client *dynamodb.Client,
	tableName string,
	visibilityCreatedAtIndex string,
	visibilityContentTypeIndex string,
	visibilityReleaseDateIndex string,
	contentCastRepo ContentCastRepository,
	contentAttributeRepo ContentAttributeRepository,
) ContentRepository {
	return &DynamoContentRepository{
		BaseRepository:             NewBaseRepository(client, tableName),
		visibilityCreatedAtIndex:   visibilityCreatedAtIndex,
		visibilityContentTypeIndex: visibilityContentTypeIndex,
		visibilityReleaseDateIndex: visibilityReleaseDateIndex,
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
		a, b := movies[i].EffectiveReleaseDate(), movies[j].EffectiveReleaseDate()
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

func discoverReadLimit(remaining int32) int32 {
	if remaining <= 0 {
		remaining = 1
	}
	batch := remaining * 5
	if batch < 50 {
		batch = 50
	}
	if batch > 200 {
		batch = 200
	}
	return batch
}

func (d *DynamoContentRepository) GetAllAdmin(ctx context.Context, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
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

func (d *DynamoContentRepository) Get(ctx context.Context, id string) (*model.Movie, error) {
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

func (d *DynamoContentRepository) GetAdmin(ctx context.Context, id string) (*model.Movie, error) {
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

func (d *DynamoContentRepository) Create(ctx context.Context, movie model.Movie) error {
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

func (d *DynamoContentRepository) Update(ctx context.Context, movie model.Movie) error {
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

func (d *DynamoContentRepository) Delete(ctx context.Context, id string) error {
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

func (d *DynamoContentRepository) GetContentByPersonIdSimple(ctx context.Context, personId string) ([]model.Movie, error) {
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

func (d *DynamoContentRepository) GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
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

func (d *DynamoContentRepository) syncContentCastEntries(ctx context.Context, contentID string, contentType model.ContentType, visibility model.Visibility, castIDs []string) error {
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

func (d *DynamoContentRepository) syncContentAttributeEntries(ctx context.Context, contentID string, contentType model.ContentType, visibility model.Visibility, attributeIDs []string) error {
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

func (d *DynamoContentRepository) GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
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

func (d *DynamoContentRepository) GetContentByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
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

func (d *DynamoContentRepository) GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error) {
	return WithTimeoutResult(ctx, "get_banner", func(ctx context.Context) (*model.Movie, error) {
		vals := map[string]types.AttributeValue{
			":visibility":      &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
			":released":        &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
			":inProduction":    &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
			":ended":           &types.AttributeValueMemberS{Value: string(model.StatusEnded)},
			":returningSeries": &types.AttributeValueMemberS{Value: string(model.StatusReturningSeries)},
			":pilot":           &types.AttributeValueMemberS{Value: string(model.StatusPilot)},
		}
		filterExpr := "(#status = :released OR #status = :inProduction OR #status = :ended OR #status = :returningSeries OR #status = :pilot)"
		filterExpr = applyContentTypeFilter(filterExpr, contentTypeFilter, vals)

		var (
			items   []map[string]types.AttributeValue
			nextKey map[string]types.AttributeValue
			err     error
		)
		for page := 0; page < 10; page++ {
			items, nextKey, err = QueryPage(ctx, d.GetClient(), &dynamodb.QueryInput{
				TableName:                 aws.String(d.GetTableName()),
				IndexName:                 aws.String(d.visibilityCreatedAtIndex),
				KeyConditionExpression:    aws.String("visibility = :visibility"),
				FilterExpression:          aws.String(filterExpr),
				ExpressionAttributeNames:  map[string]string{"#status": "status"},
				ExpressionAttributeValues: vals,
				ScanIndexForward:          aws.Bool(false),
				Limit:                     aws.Int32(50),
				ExclusiveStartKey:         nextKey,
			})
			if err != nil {
				return nil, err
			}
			if len(items) > 0 {
				break
			}
			if len(nextKey) == 0 {
				return nil, nil
			}
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

func (d *DynamoContentRepository) GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	if discoverType == "latest" {
		return d.getLatestByReleaseDate(ctx, contentTypeFilter, limit, startKey)
	}
	if discoverType == "recent" {
		return d.getRecentByCreatedAt(ctx, contentTypeFilter, limit, startKey)
	}
	return WithTimeoutResultPage(ctx, "get_discover_content", func(ctx context.Context) ([]model.Movie, map[string]types.AttributeValue, error) {
		vals := map[string]types.AttributeValue{
			":visibility":      &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
			":released":        &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
			":inProduction":    &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
			":ended":           &types.AttributeValueMemberS{Value: string(model.StatusEnded)},
			":returningSeries": &types.AttributeValueMemberS{Value: string(model.StatusReturningSeries)},
			":pilot":           &types.AttributeValueMemberS{Value: string(model.StatusPilot)},
		}
		filterExpr := "(#status = :released OR #status = :inProduction OR #status = :ended OR #status = :returningSeries OR #status = :pilot)"
		filterExpr = applyContentTypeFilter(filterExpr, contentTypeFilter, vals)

		targetLimit := defaultLimit(limit)
		fetchLimit := targetLimit
		if discoverType == "popular" || discoverType == "trending" {
			fetchLimit = 100
		}

		cursor := startKey
		collected := make([]map[string]types.AttributeValue, 0)
		for int32(len(collected)) < fetchLimit {
			remaining := fetchLimit - int32(len(collected))
			items, nextKey, err := QueryPage(ctx, d.GetClient(), &dynamodb.QueryInput{
				TableName:                 aws.String(d.GetTableName()),
				IndexName:                 aws.String(d.visibilityCreatedAtIndex),
				KeyConditionExpression:    aws.String("visibility = :visibility"),
				FilterExpression:          aws.String(filterExpr),
				ExpressionAttributeNames:  map[string]string{"#status": "status"},
				ExpressionAttributeValues: vals,
				ScanIndexForward:          aws.Bool(false),
				Limit:                     aws.Int32(discoverReadLimit(remaining)),
				ExclusiveStartKey:         cursor,
			})
			if err != nil {
				return nil, nil, err
			}
			if len(items) > 0 {
				collected = append(collected, items...)
			}
			if len(nextKey) == 0 {
				cursor = nil
				break
			}
			cursor = nextKey
		}

		if len(collected) == 0 {
			return []model.Movie{}, cursor, nil
		}
		var movies []model.Movie
		if err := attributevalue.UnmarshalListOfMaps(collected, &movies); err != nil {
			return nil, nil, err
		}

		switch discoverType {
		case "popular":
			sortByPopularity(movies)
		case "trending":
			sortByTrending(movies)
		}

		if (discoverType == "popular" || discoverType == "trending") && len(movies) > int(targetLimit) {
			movies = movies[:targetLimit]
			cursor = nil
		}
		return movies, cursor, nil
	})
}

func (d *DynamoContentRepository) getRecentByCreatedAt(ctx context.Context, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	return WithTimeoutResultPage(ctx, "get_recent_by_created_at", func(ctx context.Context) ([]model.Movie, map[string]types.AttributeValue, error) {
		vals := map[string]types.AttributeValue{
			":visibility":      &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
			":released":        &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
			":inProduction":    &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
			":ended":           &types.AttributeValueMemberS{Value: string(model.StatusEnded)},
			":returningSeries": &types.AttributeValueMemberS{Value: string(model.StatusReturningSeries)},
			":pilot":           &types.AttributeValueMemberS{Value: string(model.StatusPilot)},
		}
		filterExpr := "(#status = :released OR #status = :inProduction OR #status = :ended OR #status = :returningSeries OR #status = :pilot)"
		filterExpr = applyContentTypeFilter(filterExpr, contentTypeFilter, vals)

		targetLimit := defaultLimit(limit)
		cursor := startKey
		collected := make([]map[string]types.AttributeValue, 0)
		for int32(len(collected)) < targetLimit {
			remaining := targetLimit - int32(len(collected))
			items, nextKey, err := QueryPage(ctx, d.GetClient(), &dynamodb.QueryInput{
				TableName:                 aws.String(d.GetTableName()),
				IndexName:                 aws.String(d.visibilityCreatedAtIndex),
				KeyConditionExpression:    aws.String("visibility = :visibility"),
				FilterExpression:          aws.String(filterExpr),
				ExpressionAttributeNames:  map[string]string{"#status": "status"},
				ExpressionAttributeValues: vals,
				ScanIndexForward:          aws.Bool(false),
				Limit:                     aws.Int32(discoverReadLimit(remaining)),
				ExclusiveStartKey:         cursor,
			})
			if err != nil {
				return nil, nil, err
			}
			if len(items) > 0 {
				collected = append(collected, items...)
			}
			if len(nextKey) == 0 {
				cursor = nil
				break
			}
			cursor = nextKey
		}
		if len(collected) == 0 {
			return []model.Movie{}, cursor, nil
		}
		if int32(len(collected)) > targetLimit {
			collected = collected[:targetLimit]
		}
		var movies []model.Movie
		if err := attributevalue.UnmarshalListOfMaps(collected, &movies); err != nil {
			return nil, nil, err
		}
		return movies, cursor, nil
	})
}

func (d *DynamoContentRepository) getLatestByReleaseDate(ctx context.Context, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	return WithTimeoutResultPage(ctx, "get_latest_content", func(ctx context.Context) ([]model.Movie, map[string]types.AttributeValue, error) {
		vals := map[string]types.AttributeValue{
			":visibility":      &types.AttributeValueMemberS{Value: string(model.VisibilityPublic)},
			":released":        &types.AttributeValueMemberS{Value: string(model.StatusReleased)},
			":inProduction":    &types.AttributeValueMemberS{Value: string(model.StatusInProduction)},
			":ended":           &types.AttributeValueMemberS{Value: string(model.StatusEnded)},
			":returningSeries": &types.AttributeValueMemberS{Value: string(model.StatusReturningSeries)},
			":pilot":           &types.AttributeValueMemberS{Value: string(model.StatusPilot)},
		}
		filterExpr := "(#status = :released OR #status = :inProduction OR #status = :ended OR #status = :returningSeries OR #status = :pilot)"
		filterExpr = applyContentTypeFilter(filterExpr, contentTypeFilter, vals)

		targetLimit := defaultLimit(limit)
		cursor := startKey
		collected := make([]map[string]types.AttributeValue, 0)
		for int32(len(collected)) < targetLimit {
			remaining := targetLimit - int32(len(collected))
			items, nextKey, err := QueryPage(ctx, d.GetClient(), &dynamodb.QueryInput{
				TableName:                 aws.String(d.GetTableName()),
				IndexName:                 aws.String(d.visibilityReleaseDateIndex),
				KeyConditionExpression:    aws.String("visibility = :visibility"),
				FilterExpression:          aws.String(filterExpr),
				ExpressionAttributeNames:  map[string]string{"#status": "status"},
				ExpressionAttributeValues: vals,
				ScanIndexForward:          aws.Bool(false),
				Limit:                     aws.Int32(discoverReadLimit(remaining)),
				ExclusiveStartKey:         cursor,
			})
			if err != nil {
				return nil, nil, err
			}
			if len(items) > 0 {
				collected = append(collected, items...)
			}
			if len(nextKey) == 0 {
				cursor = nil
				break
			}
			cursor = nextKey
		}
		if len(collected) == 0 {
			return []model.Movie{}, cursor, nil
		}
		if int32(len(collected)) > targetLimit {
			collected = collected[:targetLimit]
		}
		var movies []model.Movie
		if err := attributevalue.UnmarshalListOfMaps(collected, &movies); err != nil {
			return nil, nil, err
		}
		return movies, cursor, nil
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
