package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/storage"
	"github.com/xanderbilla/bi8s-go/internal/utils"
	"github.com/xanderbilla/bi8s-go/internal/validation"

	goredis "github.com/redis/go-redis/v9"
)

type bannerCacheEntry struct {
	movie     *model.Movie
	expiresAt time.Time
}

type ContentService struct {
	repo          repository.ContentRepository
	personRepo    repository.PersonRepository
	attributeRepo repository.AttributeRepository
	fileUploader  storage.FileUploader
	searchService *SearchService
	bannerCache   sync.Map
	redisClient   *goredis.Client
	encoderRepo   repository.EncoderRepository
}

func NewContentService(repo repository.ContentRepository, personRepo repository.PersonRepository, attributeRepo repository.AttributeRepository, fileUploader storage.FileUploader) *ContentService {
	return &ContentService{
		repo:          repo,
		personRepo:    personRepo,
		attributeRepo: attributeRepo,
		fileUploader:  fileUploader,
		searchService: NewSearchService(nil, false),
	}
}

const bannerRedisTTL = 5 * time.Minute

const bannerLocalTTL = 5 * time.Second
const contentRedisTTL = 10 * time.Minute
const discoverRedisTTL = 2 * time.Minute
const personContentRedisTTL = 5 * time.Minute
const attributeContentRedisTTL = 5 * time.Minute

func (s *ContentService) SetRedisClient(client *goredis.Client) {
	s.redisClient = client
}

func (s *ContentService) SetEncoderRepo(repo repository.EncoderRepository) {
	s.encoderRepo = repo
}

func (s *ContentService) SetSearchService(searchService *SearchService) {
	if searchService == nil {
		s.searchService = NewSearchService(nil, false)
		return
	}
	s.searchService = searchService
}

func (s *ContentService) GetAllAdmin(ctx context.Context, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	return s.repo.GetAllAdmin(ctx, limit, startKey)
}

func (s *ContentService) Get(ctx context.Context, id string) (*model.Movie, error) {
	if cached, ok := cacheGetJSON[model.Movie](ctx, s.redisClient, contentCacheKey(id)); ok {
		return cached, nil
	}

	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errs.ErrContentNotFound
	}

	cacheSetJSON(ctx, s.redisClient, contentCacheKey(m.ID), m, contentRedisTTL, "content", "contentId", m.ID)
	return m, nil
}

func (s *ContentService) GetAdmin(ctx context.Context, id string) (*model.Movie, error) {
	m, err := s.repo.GetAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errs.ErrContentNotFound
	}
	return m, nil
}

func (s *ContentService) Create(ctx context.Context, movie model.Movie, posterInput, coverInput *model.FileUploadInput) (model.Movie, error) {
	if movie.ID == "" {
		movie.ID = utils.GenerateID()
	}

	validatedCasts, err := validation.ValidateAndPopulateCasts(ctx, movie.Casts, s.personRepo)
	if err != nil {
		return model.Movie{}, err
	}
	movie.Casts = validatedCasts

	if err := validation.ValidateAndPopulateAttributeGroups(ctx, s.attributeRepo,
		validation.AttributeGroup{Refs: movie.Genres, ExpectedType: model.AttributeTypeGenre, Assign: func(v []model.EntityRef) { movie.Genres = v }},
		validation.AttributeGroup{Refs: movie.Tags, ExpectedType: model.AttributeTypeTag, Assign: func(v []model.EntityRef) { movie.Tags = v }},
		validation.AttributeGroup{Refs: movie.MoodTags, ExpectedType: model.AttributeTypeMood, Assign: func(v []model.EntityRef) { movie.MoodTags = v }},
		validation.AttributeGroup{Refs: movie.Studios, ExpectedType: model.AttributeTypeStudio, Assign: func(v []model.EntityRef) { movie.Studios = v }},
	); err != nil {
		return model.Movie{}, err
	}

	castIds := make([]string, len(movie.Casts))
	for i, cast := range movie.Casts {
		castIds[i] = cast.ID
	}
	movie.CastIds = castIds

	attributeIds := make([]string, 0)
	for _, genre := range movie.Genres {
		attributeIds = append(attributeIds, genre.ID)
	}
	for _, tag := range movie.Tags {
		attributeIds = append(attributeIds, tag.ID)
	}
	for _, moodTag := range movie.MoodTags {
		attributeIds = append(attributeIds, moodTag.ID)
	}
	for _, studio := range movie.Studios {
		attributeIds = append(attributeIds, studio.ID)
	}
	movie.AttributeIds = attributeIds

	if movie.Visibility == "" {
		movie.Visibility = model.VisibilityPublic
	}

	now := time.Now()
	movie.Audit = model.Audit{
		CreatedAt: now,
		Version:   1,
	}
	movie.CreatedAt = now.UTC().Format(time.RFC3339)

	movie.Stats = model.ContentStats{
		TotalViews:    0,
		TotalLikes:    0,
		AverageRating: 0.0,
	}

	var uploadedKeys []string

	if posterInput != nil {
		posterKey, err := s.uploadFileToStorage(ctx, movie.ID, "poster", posterInput)
		if err != nil {
			s.cleanupUploadedKeys(ctx, uploadedKeys)
			return model.Movie{}, err
		}
		movie.PosterPath = posterKey
		uploadedKeys = append(uploadedKeys, posterKey)
	}

	if coverInput != nil {
		coverKey, err := s.uploadFileToStorage(ctx, movie.ID, "cover", coverInput)
		if err != nil {
			s.cleanupUploadedKeys(ctx, uploadedKeys)
			return model.Movie{}, err
		}
		movie.BackdropPath = coverKey
		uploadedKeys = append(uploadedKeys, coverKey)
	}

	if movie.ContentType == model.ContentTypeTV && movie.ReleaseDate == "" {
		movie.ReleaseDate = movie.FirstAirDate
	}

	if err := s.repo.Create(ctx, movie); err != nil {
		s.cleanupUploadedKeys(ctx, uploadedKeys)
		return model.Movie{}, err
	}
	if err := s.searchService.IndexContent(ctx, movie); err != nil {
		logger.WarnContext(ctx, "search content indexing failed", "contentId", movie.ID, "error", err.Error())
	}

	return movie, nil
}

func (s *ContentService) ResyncAllJoinTables(ctx context.Context) (int, error) {
	var (
		startKey map[string]types.AttributeValue
		total    int
	)
	for {
		movies, nextKey, err := s.repo.GetAllAdmin(ctx, 100, startKey)
		if err != nil {
			return total, err
		}
		for _, movie := range movies {
			if err := s.repo.ResyncJoinTables(ctx, movie); err != nil {
				return total, fmt.Errorf("resync join tables for content %s: %w", movie.ID, err)
			}
			total++
		}
		if len(nextKey) == 0 {
			break
		}
		startKey = nextKey
	}
	return total, nil
}

func (s *ContentService) cleanupUploadedKeys(ctx context.Context, keys []string) {
	cleanupUploadedKeys(ctx, s.fileUploader, keys)
}

func (s *ContentService) uploadFileToStorage(ctx context.Context, movieID, purpose string, input *model.FileUploadInput) (string, error) {
	return uploadInputToStorage(ctx, s.fileUploader, "movies", movieID, purpose, input)
}

func (s *ContentService) UpdateCore(ctx context.Context, id string, patch model.Movie) (*model.Movie, error) {
	existing, err := s.repo.GetAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errs.ErrContentNotFound
	}

	existing.Title = patch.Title
	existing.Overview = patch.Overview
	existing.ReleaseDate = patch.ReleaseDate
	existing.FirstAirDate = patch.FirstAirDate
	existing.Adult = patch.Adult
	existing.ContentRating = patch.ContentRating
	existing.OriginalLanguage = patch.OriginalLanguage
	existing.OriginCountry = patch.OriginCountry
	existing.Runtime = patch.Runtime
	existing.Status = patch.Status
	existing.Tagline = patch.Tagline
	existing.Visibility = patch.Visibility
	existing.Assets = patch.Assets

	if existing.ContentType == model.ContentTypeTV && existing.ReleaseDate == "" {
		existing.ReleaseDate = existing.FirstAirDate
	}

	if err := s.repo.Update(ctx, *existing); err != nil {
		return nil, err
	}
	if err := s.searchService.IndexContent(ctx, *existing); err != nil {
		logger.WarnContext(ctx, "search content indexing failed after content update", "contentId", existing.ID, "error", err.Error())
	}
	if s.redisClient != nil {
		cacheSetJSON(ctx, s.redisClient, contentCacheKey(existing.ID), existing, contentRedisTTL, "content", "contentId", existing.ID)
	}
	return existing, nil
}

func (s *ContentService) UpdatePosterImage(ctx context.Context, id string, posterInput *model.FileUploadInput) (*model.Movie, error) {
	return s.updateContentImage(ctx, id, posterInput, "poster")
}

func (s *ContentService) UpdateBackdropImage(ctx context.Context, id string, backdropInput *model.FileUploadInput) (*model.Movie, error) {
	return s.updateContentImage(ctx, id, backdropInput, "backdrop")
}

func (s *ContentService) updateContentImage(ctx context.Context, id string, input *model.FileUploadInput, purpose string) (*model.Movie, error) {
	if input == nil {
		return nil, errs.NewBadRequest("image file is required")
	}

	existing, err := s.repo.GetAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errs.ErrContentNotFound
	}

	var oldKey string
	switch purpose {
	case "poster":
		oldKey = strings.TrimSpace(existing.PosterPath)
	case "backdrop":
		oldKey = strings.TrimSpace(existing.BackdropPath)
	}

	if oldKey != "" {
		_ = s.fileUploader.Delete(ctx, strings.TrimPrefix(oldKey, "/"))
	}

	newKey, err := s.uploadFileToStorage(ctx, existing.ID, purpose, input)
	if err != nil {
		return nil, err
	}

	if purpose == "poster" {
		existing.PosterPath = newKey
	} else {
		existing.BackdropPath = newKey
	}

	if err := s.repo.Update(ctx, *existing); err != nil {
		_ = s.fileUploader.Delete(ctx, newKey)
		return nil, err
	}
	if err := s.searchService.IndexContent(ctx, *existing); err != nil {
		logger.WarnContext(ctx, "search content indexing failed after image update", "contentId", existing.ID, "error", err.Error())
	}
	if s.redisClient != nil {
		cacheSetJSON(ctx, s.redisClient, contentCacheKey(existing.ID), existing, contentRedisTTL, "content", "contentId", existing.ID)
	}

	return existing, nil
}

func (s *ContentService) AddRelation(ctx context.Context, contentID string, relationID string) (*model.Movie, error) {
	return s.mutateRelation(ctx, contentID, relationID, true)
}

func (s *ContentService) RemoveRelation(ctx context.Context, contentID string, relationID string) (*model.Movie, error) {
	return s.mutateRelation(ctx, contentID, relationID, false)
}

func (s *ContentService) mutateRelation(ctx context.Context, contentID string, relationID string, add bool) (*model.Movie, error) {
	existing, err := s.repo.GetAdmin(ctx, contentID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, errs.ErrContentNotFound
	}

	if attr, err := s.attributeRepo.Get(ctx, relationID); err == nil && attr != nil {
		if add {
			if err := addContentAttribute(existing, *attr); err != nil {
				return nil, err
			}
		} else {
			removeContentAttribute(existing, relationID)
		}
	} else {
		person, pErr := s.personRepo.Get(ctx, relationID)
		if pErr != nil || person == nil {
			return nil, errs.NewBadRequest("relation id must reference an existing attribute or person")
		}
		if add {
			if !containsEntity(existing.Casts, relationID) {
				existing.Casts = append(existing.Casts, model.EntityRef{ID: person.ID, Name: person.Name})
			}
		} else {
			existing.Casts = removeEntity(existing.Casts, relationID)
		}
	}

	rebuildContentJoins(existing)
	if err := s.repo.Update(ctx, *existing); err != nil {
		return nil, err
	}
	if err := s.searchService.IndexContent(ctx, *existing); err != nil {
		logger.WarnContext(ctx, "search content indexing failed after relation mutation", "contentId", existing.ID, "error", err.Error())
	}
	if s.redisClient != nil {
		cacheSetJSON(ctx, s.redisClient, contentCacheKey(existing.ID), existing, contentRedisTTL, "content", "contentId", existing.ID)
	}
	return existing, nil
}

func containsEntity(items []model.EntityRef, id string) bool {
	for _, it := range items {
		if it.ID == id {
			return true
		}
	}
	return false
}

func removeEntity(items []model.EntityRef, id string) []model.EntityRef {
	filtered := make([]model.EntityRef, 0, len(items))
	for _, it := range items {
		if it.ID != id {
			filtered = append(filtered, it)
		}
	}
	return filtered
}

func addContentAttribute(movie *model.Movie, attr model.Attribute) error {
	ref := model.EntityRef{ID: attr.ID, Name: attr.Name}
	for _, t := range attr.AttributeType {
		switch t {
		case model.AttributeTypeTag:
			if !containsEntity(movie.Tags, attr.ID) {
				movie.Tags = append(movie.Tags, ref)
			}
			return nil
		case model.AttributeTypeMood:
			if !containsEntity(movie.MoodTags, attr.ID) {
				movie.MoodTags = append(movie.MoodTags, ref)
			}
			return nil
		case model.AttributeTypeGenre:
			if !containsEntity(movie.Genres, attr.ID) {
				movie.Genres = append(movie.Genres, ref)
			}
			return nil
		case model.AttributeTypeStudio:
			if !containsEntity(movie.Studios, attr.ID) {
				movie.Studios = append(movie.Studios, ref)
			}
			return nil
		}
	}
	return errs.NewBadRequest("attribute type is not assignable to content")
}

func removeContentAttribute(movie *model.Movie, attrID string) {
	movie.Tags = removeEntity(movie.Tags, attrID)
	movie.MoodTags = removeEntity(movie.MoodTags, attrID)
	movie.Genres = removeEntity(movie.Genres, attrID)
	movie.Studios = removeEntity(movie.Studios, attrID)
}

func rebuildContentJoins(movie *model.Movie) {
	movie.CastIds = movie.CastIds[:0]
	for _, cast := range movie.Casts {
		if cast.ID != "" && !slices.Contains(movie.CastIds, cast.ID) {
			movie.CastIds = append(movie.CastIds, cast.ID)
		}
	}

	attributeIDs := make([]string, 0, len(movie.Genres)+len(movie.Tags)+len(movie.MoodTags)+len(movie.Studios))
	appendUnique := func(items []model.EntityRef) {
		for _, it := range items {
			if it.ID != "" && !slices.Contains(attributeIDs, it.ID) {
				attributeIDs = append(attributeIDs, it.ID)
			}
		}
	}
	appendUnique(movie.Genres)
	appendUnique(movie.Tags)
	appendUnique(movie.MoodTags)
	appendUnique(movie.Studios)
	movie.AttributeIds = attributeIDs
}

func (s *ContentService) Delete(ctx context.Context, id string) error {
	movie, err := s.repo.GetAdmin(ctx, id)
	if err != nil {
		return err
	}
	if movie == nil {
		// Record no longer in DB — still purge any orphaned search document.
		if serr := s.searchService.DeleteContent(ctx, id); serr != nil {
			logger.WarnContext(ctx, "search content delete indexing failed", "contentId", id, "error", serr.Error())
		}
		return errs.ErrContentNotFound
	}

	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	cleanupKeys := make([]string, 0, 2)
	if strings.TrimSpace(movie.PosterPath) != "" {
		cleanupKeys = append(cleanupKeys, movie.PosterPath)
	}
	if strings.TrimSpace(movie.BackdropPath) != "" {
		cleanupKeys = append(cleanupKeys, movie.BackdropPath)
	}
	for _, asset := range movie.Assets {
		for _, key := range asset.Keys {
			if strings.TrimSpace(key.Value) == "" {
				continue
			}
			cleanupKeys = append(cleanupKeys, key.Value)
		}
	}

	for _, key := range cleanupKeys {
		if err := s.fileUploader.Delete(ctx, key); err != nil {
			logger.WarnContext(ctx, "failed deleting content asset key from s3", "contentId", id, "key", key, "error", err.Error())
		}
	}

	prefixes := []string{"movies/" + id + "/", "videos/" + id + "/"}
	for _, prefix := range prefixes {
		if err := s.fileUploader.DeletePrefix(ctx, prefix); err != nil {
			logger.WarnContext(ctx, "failed deleting content prefix from s3", "contentId", id, "prefix", prefix, "error", err.Error())
		}
	}

	if s.redisClient != nil {
		cacheDel(ctx, s.redisClient, contentCacheKey(id), "content", "contentId", id)
	}
	if err := s.searchService.DeleteContent(ctx, id); err != nil {
		logger.WarnContext(ctx, "search content delete indexing failed", "contentId", id, "error", err.Error())
	}
	return nil
}

func (s *ContentService) GetContentByPersonIdSimple(ctx context.Context, personId string) ([]model.Movie, error) {
	return s.repo.GetContentByPersonIdSimple(ctx, personId)
}

func (s *ContentService) GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {

	if s.redisClient != nil && len(startKey) == 0 {
		key := personContentCacheKey(personId, contentTypeFilter, limit)
		if cached, ok := cacheGetJSON[[]model.Movie](ctx, s.redisClient, key); ok {
			return *cached, nil, nil
		}
		movies, nextKey, err := s.repo.GetContentByPersonId(ctx, personId, contentTypeFilter, limit, startKey)
		if err != nil {
			return nil, nil, err
		}
		cacheSetJSON(ctx, s.redisClient, key, movies, personContentRedisTTL, "person-content", "personId", personId)
		return movies, nextKey, nil
	}
	return s.repo.GetContentByPersonId(ctx, personId, contentTypeFilter, limit, startKey)
}

func (s *ContentService) GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	return s.repo.GetContentByPersonIdAdmin(ctx, personId, contentTypeFilter, limit, startKey)
}

func (s *ContentService) GetContentByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {

	if s.redisClient != nil && len(startKey) == 0 {
		key := attributeContentCacheKey(attributeId, contentTypeFilter, limit)
		if cached, ok := cacheGetJSON[[]model.Movie](ctx, s.redisClient, key); ok {
			return *cached, nil, nil
		}
		movies, nextKey, err := s.repo.GetContentByAttributeId(ctx, attributeId, contentTypeFilter, limit, startKey)
		if err != nil {
			return nil, nil, err
		}
		cacheSetJSON(ctx, s.redisClient, key, movies, attributeContentRedisTTL, "attribute-content", "attributeId", attributeId)
		return movies, nextKey, nil
	}
	return s.repo.GetContentByAttributeId(ctx, attributeId, contentTypeFilter, limit, startKey)
}

func (s *ContentService) GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error) {
	if s.redisClient != nil {

		poolKey := bannerPoolCacheKey(contentTypeFilter)
		var pool []model.Movie
		if cached, ok := cacheGetJSON[[]model.Movie](ctx, s.redisClient, poolKey); ok {
			pool = *cached
		} else {
			var err error
			pool, err = s.repo.GetBannerCandidates(ctx, contentTypeFilter)
			if err != nil {
				return nil, err
			}
			if len(pool) > 0 {
				cacheSetJSON(ctx, s.redisClient, poolKey, pool, bannerRedisTTL, "banner-pool", "filter", contentTypeFilter)
			}
		}
		if len(pool) == 0 {
			return nil, nil
		}
		idx := int(time.Now().UnixNano()) % len(pool)
		return &pool[idx], nil
	}

	if entry, ok := s.bannerCache.Load(contentTypeFilter); ok {
		if e := entry.(bannerCacheEntry); time.Now().Before(e.expiresAt) {
			return e.movie, nil
		}
	}
	movie, err := s.repo.GetBanner(ctx, contentTypeFilter)
	if err != nil {
		return nil, err
	}
	if movie != nil {
		s.bannerCache.Store(contentTypeFilter, bannerCacheEntry{
			movie:     movie,
			expiresAt: time.Now().Add(bannerLocalTTL),
		})
	}
	return movie, nil
}

func discoverCacheKey(discoverType, contentTypeFilter string, limit int32) string {
	return fmt.Sprintf("bi8s:discover:%s:%s:%d", discoverType, contentTypeFilter, limit)
}

func personContentCacheKey(personID, contentTypeFilter string, limit int32) string {
	return fmt.Sprintf("bi8s:person-content:%s:%s:%d", personID, contentTypeFilter, limit)
}

func attributeContentCacheKey(attributeID, contentTypeFilter string, limit int32) string {
	return fmt.Sprintf("bi8s:attr-content:%s:%s:%d", attributeID, contentTypeFilter, limit)
}

func (s *ContentService) GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {

	if s.redisClient != nil && len(startKey) == 0 {
		key := discoverCacheKey(discoverType, contentTypeFilter, limit)
		if cached, ok := cacheGetJSON[[]model.Movie](ctx, s.redisClient, key); ok {
			return *cached, nil, nil
		}
		movies, nextKey, err := s.repo.GetDiscoverContent(ctx, discoverType, contentTypeFilter, limit, startKey)
		if err == nil && len(movies) > 0 {
			cacheSetJSON(ctx, s.redisClient, key, movies, discoverRedisTTL, "discover", "type", discoverType)
		}
		return movies, nextKey, err
	}
	return s.repo.GetDiscoverContent(ctx, discoverType, contentTypeFilter, limit, startKey)
}

func (s *ContentService) UploadAssets(ctx context.Context, contentID string, assetType model.AssetType, files []*multipart.FileHeader) ([]string, error) {

	content, err := s.repo.GetAdmin(ctx, contentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, errs.ErrContentNotFound
	}

	contentTypePath := content.ContentType.ToPath()

	uploadedPaths := make([]string, 0, len(files))
	newAssetKeys := make([]model.AssetKey, 0, len(files))
	assetTypeLower := strings.ToLower(string(assetType))
	var uploadErr error

	for i, fileHeader := range files {
		ext := filepath.Ext(fileHeader.Filename)
		if ext == "" {
			ext = ".mp4"
		}
		fileUUID := strings.ReplaceAll(utils.GenerateID(), "-", "")[:16]
		fileName := fmt.Sprintf("%s%s", fileUUID, ext)
		s3Path := fmt.Sprintf("%s/%s/assets/%s/%s", contentTypePath, contentID, assetTypeLower, fileName)

		s3Key, err := s.uploadSingleAsset(ctx, fileHeader, s3Path, fileName)
		if err != nil {
			uploadErr = fmt.Errorf("upload file %d: %w", i+1, err)
			break
		}
		path := "/" + s3Key
		uploadedPaths = append(uploadedPaths, path)
		newAssetKeys = append(newAssetKeys, model.AssetKey{
			ID:    utils.GenerateID(),
			Value: path,
		})
	}

	if len(newAssetKeys) > 0 {
		assetFound := false
		for i := range content.Assets {
			if content.Assets[i].Type == assetType {
				content.Assets[i].Keys = append(content.Assets[i].Keys, newAssetKeys...)
				assetFound = true
				break
			}
		}

		if !assetFound {
			content.Assets = append(content.Assets, model.Asset{
				Type: assetType,
				Keys: newAssetKeys,
			})
		}

		if err := s.repo.Update(ctx, *content); err != nil {
			return uploadedPaths, fmt.Errorf("failed to update content: %w", err)
		}
		if err := s.searchService.IndexContent(ctx, *content); err != nil {
			logger.WarnContext(ctx, "search content indexing failed after asset upload", "contentId", content.ID, "error", err.Error())
		}
		if s.redisClient != nil {
			cacheSetJSON(ctx, s.redisClient, contentCacheKey(content.ID), content, contentRedisTTL, "content", "contentId", content.ID)
		}
	}

	if uploadErr != nil {
		return uploadedPaths, uploadErr
	}

	return uploadedPaths, nil
}

func (s *ContentService) DeleteAssetKey(ctx context.Context, contentID string, assetType model.AssetType, keyID string) error {
	content, err := s.repo.GetAdmin(ctx, contentID)
	if err != nil {
		return err
	}
	if content == nil {
		return errs.ErrContentNotFound
	}

	var keyValue string
	assetIdx := -1
	keyIdx := -1
	for i, asset := range content.Assets {
		if asset.Type == assetType {
			for j, k := range asset.Keys {
				if k.ID == keyID {
					keyValue = k.Value
					assetIdx = i
					keyIdx = j
					break
				}
			}
			break
		}
	}
	if assetIdx == -1 || keyIdx == -1 {
		return errs.ErrContentNotFound
	}

	content.Assets[assetIdx].Keys = append(
		content.Assets[assetIdx].Keys[:keyIdx],
		content.Assets[assetIdx].Keys[keyIdx+1:]...,
	)
	if len(content.Assets[assetIdx].Keys) == 0 {
		content.Assets = append(content.Assets[:assetIdx], content.Assets[assetIdx+1:]...)
	}

	if err := s.repo.Update(ctx, *content); err != nil {
		return fmt.Errorf("failed to update content: %w", err)
	}

	if strings.TrimSpace(keyValue) != "" {
		if err := s.fileUploader.Delete(ctx, strings.TrimPrefix(keyValue, "/")); err != nil {
			logger.WarnContext(ctx, "failed to delete asset key from storage", "contentId", contentID, "keyId", keyID, "key", keyValue, "error", err.Error())
		}
	}

	if s.redisClient != nil {
		cacheSetJSON(ctx, s.redisClient, contentCacheKey(content.ID), content, contentRedisTTL, "content", "contentId", content.ID)
	}
	if err := s.searchService.IndexContent(ctx, *content); err != nil {
		logger.WarnContext(ctx, "search content indexing failed after asset key deletion", "contentId", content.ID, "error", err.Error())
	}

	return nil
}

func contentCacheKey(id string) string {
	return "bi8s:content:" + id
}

func bannerPoolCacheKey(contentTypeFilter string) string {
	return "bi8s:banner-pool:" + contentTypeFilter
}

func (s *ContentService) uploadSingleAsset(
	ctx context.Context,
	fh *multipart.FileHeader,
	s3Path, fileName string,
) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("open %s: %w", fh.Filename, err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			logger.WarnContext(ctx, "failed to close asset file", "filename", fh.Filename, "error", err.Error())
		}
	}()

	contentType := videoContentTypeForFile(fh)

	s3Key, err := s.fileUploader.UploadFileStream(ctx, "", "", s3Path, fileName, contentType, f, fh.Size)
	if err != nil {
		return "", fmt.Errorf("upload %s: %w", fh.Filename, err)
	}
	return s3Key, nil
}

func videoContentTypeForFile(fh *multipart.FileHeader) string {
	if fh.Header != nil {
		if ct := strings.TrimSpace(fh.Header.Get("Content-Type")); ct != "" {
			if idx := strings.Index(ct, ";"); idx > 0 {
				ct = strings.TrimSpace(ct[:idx])
			}
			switch strings.ToLower(ct) {
			case "video/mp4", "video/quicktime", "video/webm", "video/x-matroska", "video/x-msvideo":
				return strings.ToLower(ct)
			}
		}
	}
	switch strings.ToLower(filepath.Ext(fh.Filename)) {
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	case ".mkv":
		return "video/x-matroska"
	case ".avi":
		return "video/x-msvideo"
	default:
		return "video/mp4"
	}
}

// GetPlayback fetches the finished encoder job for contentID, validates the
// contentType, and returns the raw playback block. Paths are S3 keys served
// via the Cloudflare Worker CDN — no presigning needed.
func (s *ContentService) GetPlayback(ctx context.Context, contentID, contentType string) (*model.PlaybackInfo, error) {
	if s.encoderRepo == nil {
		return nil, errs.NewNotFound("playback not available")
	}
	job, err := s.encoderRepo.GetFinishedByContentID(ctx, contentID)
	if err != nil {
		return nil, err
	}
	if !strings.EqualFold(job.ContentType, contentType) {
		return nil, errs.NewNotFound("playback not available for this content type")
	}
	pb := job.Playback
	return &pb, nil
}
