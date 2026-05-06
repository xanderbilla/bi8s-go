package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
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
	repo           repository.ContentRepository
	personRepo     repository.PersonRepository
	attributeRepo  repository.AttributeRepository
	encoderRepo    repository.EncoderRepository
	fileUploader   storage.FileUploader
	searchService  *SearchService
	bannerCache    sync.Map
	redisClient    *goredis.Client
	playbackURLTTL time.Duration
}

func NewContentService(repo repository.ContentRepository, personRepo repository.PersonRepository, attributeRepo repository.AttributeRepository, encoderRepo repository.EncoderRepository, fileUploader storage.FileUploader) *ContentService {
	return &ContentService{
		repo:           repo,
		personRepo:     personRepo,
		attributeRepo:  attributeRepo,
		encoderRepo:    encoderRepo,
		fileUploader:   fileUploader,
		searchService:  NewSearchService(nil, false),
		playbackURLTTL: 20 * time.Minute,
	}
}

const bannerRedisTTL = 5 * time.Minute

const bannerLocalTTL = 5 * time.Second
const contentRedisTTL = 60 * time.Second

func (s *ContentService) SetRedisClient(client *goredis.Client) {
	s.redisClient = client
}

func (s *ContentService) SetPlaybackURLTTL(ttl time.Duration) {
	if ttl > 0 {
		s.playbackURLTTL = ttl
	}
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

func (s *ContentService) cleanupUploadedKeys(ctx context.Context, keys []string) {
	cleanupUploadedKeys(ctx, s.fileUploader, keys)
}

func (s *ContentService) uploadFileToStorage(ctx context.Context, movieID, purpose string, input *model.FileUploadInput) (string, error) {
	return uploadInputToStorage(ctx, s.fileUploader, "movies", movieID, purpose, input)
}

func (s *ContentService) Delete(ctx context.Context, id string) error {
	movie, err := s.repo.GetAdmin(ctx, id)
	if err != nil {
		return err
	}
	if movie == nil {
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
			if strings.TrimSpace(key) == "" {
				continue
			}
			cleanupKeys = append(cleanupKeys, key)
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
	return s.repo.GetContentByPersonId(ctx, personId, contentTypeFilter, limit, startKey)
}

func (s *ContentService) GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	return s.repo.GetContentByPersonIdAdmin(ctx, personId, contentTypeFilter, limit, startKey)
}

func (s *ContentService) GetContentByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	return s.repo.GetContentByAttributeId(ctx, attributeId, contentTypeFilter, limit, startKey)
}

func (s *ContentService) GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error) {
	if s.redisClient != nil {
		key := bannerCacheKey(contentTypeFilter)
		if cached, ok := cacheGetJSON[model.Movie](ctx, s.redisClient, key); ok {
			return cached, nil
		}

		movie, err := s.repo.GetBanner(ctx, contentTypeFilter)
		if err != nil {
			return nil, err
		}
		if movie != nil {
			cacheSetJSON(ctx, s.redisClient, key, movie, bannerRedisTTL, "banner", "key", key)
		}
		return movie, nil
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

func (s *ContentService) GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string, limit int32, startKey map[string]types.AttributeValue) ([]model.Movie, map[string]types.AttributeValue, error) {
	return s.repo.GetDiscoverContent(ctx, discoverType, contentTypeFilter, limit, startKey)
}

func (s *ContentService) UploadAssets(ctx context.Context, contentID string, assetType model.AssetType, files []*multipart.FileHeader) ([]string, error) {

	content, err := s.repo.Get(ctx, contentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, errs.ErrContentNotFound
	}

	contentTypePath := content.ContentType.ToPath()

	existingCount := 0
	for _, asset := range content.Assets {
		if asset.Type == assetType {
			existingCount = len(asset.Keys)
			break
		}
	}

	uploadedPaths := make([]string, 0, len(files))
	assetTypeLower := strings.ToLower(string(assetType))
	var uploadErr error

	for i, fileHeader := range files {
		ext := filepath.Ext(fileHeader.Filename)
		if ext == "" {
			ext = ".mp4"
		}
		count := existingCount + i
		fileName := fmt.Sprintf("%s_%d%s", assetTypeLower, count, ext)
		s3Path := fmt.Sprintf("%s/%s/assets/%s", contentTypePath, contentID, fileName)

		s3Key, err := s.uploadSingleAsset(ctx, fileHeader, s3Path, fileName)
		if err != nil {
			uploadErr = fmt.Errorf("upload file %d: %w", i+1, err)
			break
		}
		uploadedPaths = append(uploadedPaths, "/"+s3Key)
	}

	if len(uploadedPaths) > 0 {
		assetFound := false
		for i := range content.Assets {
			if content.Assets[i].Type == assetType {
				content.Assets[i].Keys = append(content.Assets[i].Keys, uploadedPaths...)
				assetFound = true
				break
			}
		}

		if !assetFound {
			content.Assets = append(content.Assets, model.Asset{
				Type: assetType,
				Keys: uploadedPaths,
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

func contentCacheKey(id string) string {
	return "bi8s:content:" + id
}

func bannerCacheKey(contentTypeFilter string) string {
	return "bi8s:banner:" + contentTypeFilter
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
