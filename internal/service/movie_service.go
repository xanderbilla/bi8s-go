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
)

// MovieService sits between the HTTP handlers and the database layer.
// Any business logic — like validation, transformations, or access rules —
// should live here, keeping handlers thin and the database layer focused on data only.
type MovieService struct {
	repo         repository.MovieRepository
	fileUploader storage.FileUploader
}



// NewMovieService creates a new MovieService and wires it up with the given repository.
// Call this once at startup and pass the result into your handlers.
func NewMovieService(repo repository.MovieRepository, fileUploader storage.FileUploader) *MovieService {
	return &MovieService{repo: repo, fileUploader: fileUploader}
}

// GetAll returns every movie in the database.
// If you ever need filtering, sorting, or pagination, add that logic here
// rather than in the handler or the repository.
func (s *MovieService) GetAll(ctx context.Context) ([]model.Movie, error) {
	return s.repo.GetAll(ctx)
}

// Get returns a single movie by its ID.
// Returns nil (with no error) if no movie with that ID exists.
func (s *MovieService) Get(ctx context.Context, id string) (*model.Movie, error) {
	return s.repo.Get(ctx, id)
}

// Create saves a new movie to the database with optional poster, cover, cast, and company image uploads.
// If file inputs are provided, they will be uploaded to storage before saving.
func (s *MovieService) Create(ctx context.Context, movie model.Movie, posterInput, coverInput *domain.FileUploadInput, castImages map[string]*domain.FileUploadInput, companyImages map[string]*domain.FileUploadInput) (model.Movie, error) {
	if movie.ID == "" {
		movie.ID = utils.GenerateID()
	}

	// Set audit timestamps
	now := time.Now()
	movie.Audit = model.Audit{
		CreatedAt: now,
		IsDeleted: false,
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

	// Upload cast images
	if len(castImages) > 0 {
		for i := range movie.Casts {
			if img, ok := castImages[movie.Casts[i].ID]; ok && img != nil {
				key, err := s.uploadCastImage(ctx, movie.Casts[i].ID, img)
				if err != nil {
					return model.Movie{}, err
				}
				movie.Casts[i].CoverPicture = key
			}
		}
	}

	// Upload studio images
	if len(companyImages) > 0 {
		for i := range movie.Studios {
			if img, ok := companyImages[movie.Studios[i].ID]; ok && img != nil {
				key, err := s.uploadCompanyImage(ctx, movie.Studios[i].ID, img)
				if err != nil {
					return model.Movie{}, err
				}
				movie.Studios[i].CoverPicture = key
			}
		}
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

// uploadCastImage uploads a cast member's image to S3 with path: casts/{cast_id}/cover.{ext}
func (s *MovieService) uploadCastImage(ctx context.Context, castID string, input *domain.FileUploadInput) (string, error) {
	if s.fileUploader == nil {
		return "", errs.ErrFileUploaderNotConfigured
	}

	return s.fileUploader.UploadFile(
		ctx,
		"casts",
		castID,
		"cover",
		input.FileName,
		input.ContentType,
		input.Data,
	)
}

// uploadCompanyImage uploads a production company's image to S3 with path: studios/{company_id}/media/cover.{ext}
func (s *MovieService) uploadCompanyImage(ctx context.Context, companyID string, input *domain.FileUploadInput) (string, error) {
	if s.fileUploader == nil {
		return "", errs.ErrFileUploaderNotConfigured
	}

	return s.fileUploader.UploadFile(
		ctx,
		"studios",
		companyID,
		"media/cover",
		input.FileName,
		input.ContentType,
		input.Data,
	)
}

// Delete removes a movie from the database by its ID.
func (s *MovieService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
