package service

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/domain"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/model"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/storage"
	"github.com/xanderbilla/bi8s-go/internal/utils"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

// MovieService sits between the HTTP handlers and the database layer.
// Any business logic — like validation, transformations, or access rules —
// should live here, keeping handlers thin and the database layer focused on data only.
type MovieService struct {
	repo          repository.MovieRepository
	personRepo    repository.PersonRepository
	attributeRepo repository.AttributeRepository
	encoderRepo   repository.EncoderRepository
	fileUploader  storage.FileUploader
}

// NewMovieService creates a new MovieService and wires it up with the given repository.
// Call this once at startup and pass the result into your handlers.
func NewMovieService(repo repository.MovieRepository, personRepo repository.PersonRepository, attributeRepo repository.AttributeRepository, encoderRepo repository.EncoderRepository, fileUploader storage.FileUploader) *MovieService {
	return &MovieService{
		repo:          repo,
		personRepo:    personRepo,
		attributeRepo: attributeRepo,
		encoderRepo:   encoderRepo,
		fileUploader:  fileUploader,
	}
}

// GetAllAdmin returns every movie without filtering (for admin use).
func (s *MovieService) GetAllAdmin(ctx context.Context) ([]model.Movie, error) {
	return s.repo.GetAllAdmin(ctx)
}

// GetRecentContent returns content filtered by type and sorted by creation date (most recent first).
func (s *MovieService) GetRecentContent(ctx context.Context, contentTypeFilter string) ([]model.Movie, error) {
	movies, err := s.repo.GetRecentContent(ctx, contentTypeFilter)
	if err != nil {
		return nil, err
	}
	
	// Enrich assets for all movies
	return s.enrichAssetsForList(ctx, movies), nil
}

// Get returns a single movie by its ID.
// Returns nil (with no error) if no movie with that ID exists.
func (s *MovieService) Get(ctx context.Context, id string) (*model.Movie, error) {
	movie, err := s.repo.Get(ctx, id)
	if err != nil || movie == nil {
		return movie, err
	}
	
	// Enrich assets from encoder if needed
	s.enrichAssetsFromEncoder(ctx, movie)
	
	return movie, nil
}

// GetAdmin returns a single movie by ID without filtering (for admin use).
func (s *MovieService) GetAdmin(ctx context.Context, id string) (*model.Movie, error) {
	return s.repo.GetAdmin(ctx, id)
}

// Create saves a new movie to the database with optional poster and cover uploads.
// If file inputs are provided, they will be uploaded to storage before saving.
func (s *MovieService) Create(ctx context.Context, movie model.Movie, posterInput, coverInput *domain.FileUploadInput) (model.Movie, error) {
	if movie.ID == "" {
		movie.ID = utils.GenerateID()
	}

	// Validate that all cast members exist in person table and populate names
	validatedCasts, err := validation.ValidateAndPopulateCasts(ctx, movie.Casts, s.personRepo)
	if err != nil {
		return model.Movie{}, err
	}
	movie.Casts = validatedCasts

	// Validate and populate genres (must have GENRE attribute type)
	validatedGenres, err := validation.ValidateAndPopulateAttributes(ctx, movie.Genres, model.AttributeTypeGenre, s.attributeRepo)
	if err != nil {
		return model.Movie{}, err
	}
	movie.Genres = validatedGenres

	// Validate and populate tags (must have TAG attribute type)
	validatedTags, err := validation.ValidateAndPopulateAttributes(ctx, movie.Tags, model.AttributeTypeTag, s.attributeRepo)
	if err != nil {
		return model.Movie{}, err
	}
	movie.Tags = validatedTags

	// Validate and populate mood tags (must have MOOD attribute type)
	validatedMoodTags, err := validation.ValidateAndPopulateAttributes(ctx, movie.MoodTags, model.AttributeTypeMood, s.attributeRepo)
	if err != nil {
		return model.Movie{}, err
	}
	movie.MoodTags = validatedMoodTags

	// Validate and populate studios (must have STUDIO attribute type)
	validatedStudios, err := validation.ValidateAndPopulateAttributes(ctx, movie.Studios, model.AttributeTypeStudio, s.attributeRepo)
	if err != nil {
		return model.Movie{}, err
	}
	movie.Studios = validatedStudios

	// Populate castIds for querying
	castIds := make([]string, len(movie.Casts))
	for i, cast := range movie.Casts {
		castIds[i] = cast.ID
	}
	movie.CastIds = castIds

	// Populate attributeIds for querying (combine all attribute types)
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

	// Set default visibility to PUBLIC
	if movie.Visibility == "" {
		movie.Visibility = model.VisibilityPublic
	}

	// Set audit timestamps
	now := time.Now()
	movie.Audit = model.Audit{
		CreatedAt: now,
		IsDeleted: false,
		Version:   1,
	}

	// Initialize stats with zero values
	movie.Stats = model.ContentStats{
		TotalViews:    0,
		TotalLikes:    0,
		AverageRating: 0.0,
	}

	// Upload movie poster
	if posterInput != nil {
		posterKey, err := s.uploadFileToStorage(ctx, movie.ID, "poster", posterInput)
		if err != nil {
			return model.Movie{}, err
		}
		movie.PosterPath = posterKey
	}

	// Upload movie cover/backdrop
	if coverInput != nil {
		coverKey, err := s.uploadFileToStorage(ctx, movie.ID, "cover", coverInput)
		if err != nil {
			return model.Movie{}, err
		}
		movie.BackdropPath = coverKey
	}

	if err := s.repo.Create(ctx, movie); err != nil {
		return model.Movie{}, err
	}

	return movie, nil
}

// uploadFileToStorage ensures identical business constraints apply to all S3 interactions seamlessly.
func (s *MovieService) uploadFileToStorage(ctx context.Context, movieID, purpose string, input *domain.FileUploadInput) (string, error) {
	if s.fileUploader == nil {
		return "", errs.ErrFileUploaderNotConfigured
	}

	return s.fileUploader.UploadFile(
		ctx,
		"movies",
		movieID,
		purpose,
		input.FileName,
		input.ContentType,
		input.Data,
	)
}

// Delete removes a movie from the database by its ID.
func (s *MovieService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// GetMoviesByPersonId returns all movies where the person is in the cast.
func (s *MovieService) GetMoviesByPersonId(ctx context.Context, personId string) ([]model.Movie, error) {
	movies, err := s.repo.GetMoviesByPersonId(ctx, personId)
	if err != nil {
		return nil, err
	}
	return s.enrichAssetsForList(ctx, movies), nil
}

// GetContentByPersonId returns all content where the person is in the cast, filtered by content type.
func (s *MovieService) GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error) {
	movies, err := s.repo.GetContentByPersonId(ctx, personId, contentTypeFilter)
	if err != nil {
		return nil, err
	}
	return s.enrichAssetsForList(ctx, movies), nil
}

// GetContentByPersonIdAdmin returns all content where the person is in the cast, filtered by content type (admin - no visibility/status filtering).
func (s *MovieService) GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error) {
	return s.repo.GetContentByPersonIdAdmin(ctx, personId, contentTypeFilter)
}

// GetMoviesByAttributeId returns all movies that have the specified attribute.
func (s *MovieService) GetMoviesByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string) ([]model.Movie, error) {
	movies, err := s.repo.GetMoviesByAttributeId(ctx, attributeId, contentTypeFilter)
	if err != nil {
		return nil, err
	}
	return s.enrichAssetsForList(ctx, movies), nil
}

// GetBanner returns a random banner content with optional contentType filter.
func (s *MovieService) GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error) {
	banner, err := s.repo.GetBanner(ctx, contentTypeFilter)
	if err != nil || banner == nil {
		return banner, err
	}
	
	// Enrich assets from encoder if needed
	s.enrichAssetsFromEncoder(ctx, banner)
	
	return banner, nil
}

// GetDiscoverContent returns content for discovery based on type and content filter.
func (s *MovieService) GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string) ([]model.Movie, error) {
	movies, err := s.repo.GetDiscoverContent(ctx, discoverType, contentTypeFilter)
	if err != nil {
		return nil, err
	}
	return s.enrichAssetsForList(ctx, movies), nil
}


// enrichAssetsFromEncoder enriches content assets by fetching preview from encoder if assets are empty
func (s *MovieService) enrichAssetsFromEncoder(ctx context.Context, content *model.Content) {
	// If assets already exist, no need to fetch from encoder
	if len(content.Assets) > 0 {
		return
	}
	
	// Query encoder table for this content ID to get the latest job
	// For now, we'll implement a simple approach - you may want to add a method to get latest job by contentId
	// This is a placeholder - you'll need to implement GetLatestByContentId in encoder repository
	jobs, err := s.encoderRepo.GetByContentId(ctx, content.ID)
	if err != nil || len(jobs) == 0 {
		return
	}
	
	// Get the most recent completed job
	var latestJob *model.EncoderJob
	for i := range jobs {
		if jobs[i].Status == model.EncoderStatusCompleted || jobs[i].Status == model.EncoderStatusCompletedWithWarnings {
			if latestJob == nil || (jobs[i].Meta.CompletedAt != nil && latestJob.Meta.CompletedAt != nil && jobs[i].Meta.CompletedAt.After(*latestJob.Meta.CompletedAt)) {
				latestJob = &jobs[i]
			}
		}
	}
	
	// If we found a completed job with a preview, add it as a TRAILER asset
	if latestJob != nil && latestJob.Output.Preview.File != nil && *latestJob.Output.Preview.File != "" {
		content.Assets = []model.Asset{
			{
				Type: model.AssetTypeTrailer,
				Keys: []string{*latestJob.Output.Preview.File},
			},
		}
	}
}

// enrichAssetsForList enriches a list of content with assets from encoder
func (s *MovieService) enrichAssetsForList(ctx context.Context, contents []model.Content) []model.Content {
	for i := range contents {
		s.enrichAssetsFromEncoder(ctx, &contents[i])
	}
	return contents
}


// UploadAssets uploads video assets for a content item
func (s *MovieService) UploadAssets(ctx context.Context, contentID string, assetType model.AssetType, files []*multipart.FileHeader) ([]string, error) {
	// Get the content to determine content type
	content, err := s.repo.Get(ctx, contentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, errs.ErrContentNotFound
	}
	
	// Determine content type path using ToPath() method
	contentTypePath := content.ContentType.ToPath()
	
	// Get existing assets for this type to determine starting count
	existingCount := 0
	for _, asset := range content.Assets {
		if asset.Type == assetType {
			existingCount = len(asset.Keys)
			break
		}
	}
	
	// Upload each file
	uploadedPaths := []string{}
	assetTypeLower := strings.ToLower(string(assetType))
	
	for i, fileHeader := range files {
		// Open the file
		file, err := fileHeader.Open()
		if err != nil {
			return uploadedPaths, fmt.Errorf("failed to open file %s: %w", fileHeader.Filename, err)
		}
		defer file.Close()
		
		// Read file data
		data, err := io.ReadAll(file)
		if err != nil {
			return uploadedPaths, fmt.Errorf("failed to read file %s: %w", fileHeader.Filename, err)
		}
		
		// Determine file extension
		ext := filepath.Ext(fileHeader.Filename)
		if ext == "" {
			ext = ".mp4"
		}
		
		// Generate S3 path
		count := existingCount + i
		fileName := fmt.Sprintf("%s_%d%s", assetTypeLower, count, ext)
		s3Path := fmt.Sprintf("%s/%s/assets/%s", contentTypePath, contentID, fileName)
		
		// Upload to S3
		_, err = s.fileUploader.UploadFile(ctx, "", "", s3Path, fileName, "video/mp4", data)
		if err != nil {
			return uploadedPaths, fmt.Errorf("failed to upload file %s: %w", fileHeader.Filename, err)
		}
		
		uploadedPaths = append(uploadedPaths, "/"+s3Path)
	}
	
	// Update content with new assets
	assetFound := false
	for i := range content.Assets {
		if content.Assets[i].Type == assetType {
			// Append to existing keys
			content.Assets[i].Keys = append(content.Assets[i].Keys, uploadedPaths...)
			assetFound = true
			break
		}
	}
	
	// If asset type doesn't exist, create new entry
	if !assetFound {
		content.Assets = append(content.Assets, model.Asset{
			Type: assetType,
			Keys: uploadedPaths,
		})
	}
	
	// Update content in database
	if err := s.repo.Update(ctx, *content); err != nil {
		return uploadedPaths, fmt.Errorf("failed to update content: %w", err)
	}
	
	return uploadedPaths, nil
}


// GetPlaybackInfo retrieves playback information from the encoder table
func (s *MovieService) GetPlaybackInfo(ctx context.Context, contentID string) (map[string]interface{}, error) {
	// Get the content to verify it exists
	content, err := s.repo.Get(ctx, contentID)
	if err != nil {
		return nil, err
	}
	if content == nil {
		return nil, errs.ErrContentNotFound
	}
	
	// Get encoder jobs for this content
	jobs, err := s.encoderRepo.GetByContentId(ctx, contentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get encoder jobs: %w", err)
	}
	
	if len(jobs) == 0 {
		return nil, errs.ErrNoEncodingFound
	}
	
	// Find the latest completed job
	var latestJob *model.EncoderJob
	for i := range jobs {
		if jobs[i].Status == model.EncoderStatusCompleted || jobs[i].Status == model.EncoderStatusCompletedWithWarnings {
			if latestJob == nil || (jobs[i].Meta.CompletedAt != nil && latestJob.Meta.CompletedAt != nil && jobs[i].Meta.CompletedAt.After(*latestJob.Meta.CompletedAt)) {
				latestJob = &jobs[i]
			}
		}
	}
	
	if latestJob == nil {
		return nil, errs.ErrNoCompletedEncoding
	}
	
	// Return the pre-built playback object with contentId, contentType, and info
	if latestJob.Playback != nil {
		return map[string]interface{}{
			"contentId":   contentID,
			"contentType": content.ContentType,
			"info": map[string]interface{}{
				"title":    content.Title,
				"overview": content.Overview,
				"casts":    content.Casts,
			},
			"playback": latestJob.Playback,
		}, nil
	}
	
	// Fallback: if playback field doesn't exist (old jobs), return error
	return nil, errs.ErrPlaybackNotAvailable
}
