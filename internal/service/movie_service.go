package service

import (
	"context"
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
	fileUploader  storage.FileUploader
}

// NewMovieService creates a new MovieService and wires it up with the given repository.
// Call this once at startup and pass the result into your handlers.
func NewMovieService(repo repository.MovieRepository, personRepo repository.PersonRepository, attributeRepo repository.AttributeRepository, fileUploader storage.FileUploader) *MovieService {
	return &MovieService{
		repo:          repo,
		personRepo:    personRepo,
		attributeRepo: attributeRepo,
		fileUploader:  fileUploader,
	}
}

// GetAllAdmin returns every movie without filtering (for admin use).
func (s *MovieService) GetAllAdmin(ctx context.Context) ([]model.Movie, error) {
	return s.repo.GetAllAdmin(ctx)
}

// GetRecentContent returns content filtered by type and sorted by creation date (most recent first).
func (s *MovieService) GetRecentContent(ctx context.Context, contentTypeFilter string) ([]model.Movie, error) {
	return s.repo.GetRecentContent(ctx, contentTypeFilter)
}

// Get returns a single movie by its ID.
// Returns nil (with no error) if no movie with that ID exists.
func (s *MovieService) Get(ctx context.Context, id string) (*model.Movie, error) {
	return s.repo.Get(ctx, id)
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
	return s.repo.GetMoviesByPersonId(ctx, personId)
}

// GetContentByPersonId returns all content where the person is in the cast, filtered by content type.
func (s *MovieService) GetContentByPersonId(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error) {
	return s.repo.GetContentByPersonId(ctx, personId, contentTypeFilter)
}

// GetContentByPersonIdAdmin returns all content where the person is in the cast, filtered by content type (admin - no visibility/status filtering).
func (s *MovieService) GetContentByPersonIdAdmin(ctx context.Context, personId string, contentTypeFilter string) ([]model.Movie, error) {
	return s.repo.GetContentByPersonIdAdmin(ctx, personId, contentTypeFilter)
}

// GetMoviesByAttributeId returns all movies that have the specified attribute.
func (s *MovieService) GetMoviesByAttributeId(ctx context.Context, attributeId string, contentTypeFilter string) ([]model.Movie, error) {
	return s.repo.GetMoviesByAttributeId(ctx, attributeId, contentTypeFilter)
}

// GetBanner returns a random banner content with optional contentType filter.
func (s *MovieService) GetBanner(ctx context.Context, contentTypeFilter string) (*model.Movie, error) {
	return s.repo.GetBanner(ctx, contentTypeFilter)
}

// GetDiscoverContent returns content for discovery based on type and content filter.
func (s *MovieService) GetDiscoverContent(ctx context.Context, discoverType string, contentTypeFilter string) ([]model.Movie, error) {
	return s.repo.GetDiscoverContent(ctx, discoverType, contentTypeFilter)
}
