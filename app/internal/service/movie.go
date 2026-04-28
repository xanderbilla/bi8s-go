package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/storage"
	"github.com/xanderbilla/bi8s-go/internal/utils"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

type MovieService struct {
	repo          repository.MovieRepository
	personRepo    repository.PersonRepository
	attributeRepo repository.AttributeRepository
	encoderRepo   repository.EncoderRepository
	fileUploader  storage.FileUploader
}

func NewMovieService(repo repository.MovieRepository, personRepo repository.PersonRepository, attributeRepo repository.AttributeRepository, encoderRepo repository.EncoderRepository, fileUploader storage.FileUploader) *MovieService {
	return &MovieService{
		repo:          repo,
		personRepo:    personRepo,
		attributeRepo: attributeRepo,
		encoderRepo:   encoderRepo,
		fileUploader:  fileUploader,
	}
}

func (s *MovieService) GetAllAdmin(ctx context.Context) ([]model.Movie, error) {
	return s.repo.GetAllAdmin(ctx)
}

func (s *MovieService) GetRecentContent(ctx context.Context, contentTypeFilter string) ([]model.Movie, error) {
	return s.repo.GetRecentContent(ctx, contentTypeFilter)
}

func (s *MovieService) Get(ctx context.Context, id string) (*model.Movie, error) {
	m, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errs.ErrContentNotFound
	}
	return m, nil
}

func (s *MovieService) GetAdmin(ctx context.Context, id string) (*model.Movie, error) {
	m, err := s.repo.GetAdmin(ctx, id)
	if err != nil {
		return nil, err
	}
	if m == nil {
		return nil, errs.ErrContentNotFound
	}
	return m, nil
}

func (s *MovieService) Create(ctx context.Context, movie model.Movie, posterInput, coverInput *model.FileUploadInput) (model.Movie, error) {
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

	if err := s.repo.Create(ctx, movie); err != nil {
		s.cleanupUploadedKeys(ctx, uploadedKeys)
		return model.Movie{}, err
	}

	return movie, nil
}

func (s *MovieService) cleanupUploadedKeys(ctx context.Context, keys []string) {
	cleanupUploadedKeys(ctx, s.fileUploader, keys)
}

func (s *MovieService) uploadFileToStorage(ctx context.Context, movieID, purpose string, input *model.FileUploadInput) (string, error) {
	return uploadInputToStorage(ctx, s.fileUploader, "movies", movieID, purpose, input)
}

func (s *MovieService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

func (s *MovieService) GetMoviesByPersonId(ctx context.Context, personId string) ([]model.Movie, error) {
	return s.repo.GetMoviesByPersonId(ctx, personId)
}

func (s *MovieService) GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error) {
	return s.repo.GetContentByPersonId(ctx, personId, contentTypeFilter)
}

func (s *MovieService) GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error) {
	return s.repo.GetContentByPersonIdAdmin(ctx, personId, contentTypeFilter)
}

func (s *MovieService) GetMoviesByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string) ([]model.Movie, error) {
	return s.repo.GetMoviesByAttributeId(ctx, attributeId, contentTypeFilter)
}

func (s *MovieService) GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error) {
	return s.repo.GetBanner(ctx, contentTypeFilter)
}

func (s *MovieService) GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string) ([]model.Movie, error) {
	return s.repo.GetDiscoverContent(ctx, discoverType, contentTypeFilter)
}

func (s *MovieService) UploadAssets(ctx context.Context, contentID string, assetType model.AssetType, files []*multipart.FileHeader) ([]string, error) {

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
	}

	if uploadErr != nil {
		return uploadedPaths, uploadErr
	}

	return uploadedPaths, nil
}

func (s *MovieService) uploadSingleAsset(
	ctx context.Context,
	fh *multipart.FileHeader,
	s3Path, fileName string,
) (string, error) {
	f, err := fh.Open()
	if err != nil {
		return "", fmt.Errorf("open %s: %w", fh.Filename, err)
	}
	defer f.Close()

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
