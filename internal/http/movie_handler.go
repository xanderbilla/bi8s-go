package http

import (
	"errors"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/domain"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/service"
)

// MovieHandler handles movie-related HTTP routes.
// It keeps handler code focused on request/response flow, while shared packages
// handle validation and error formatting consistently.
type MovieHandler struct {
	movieService *service.MovieService
}

// GetAllMovies handles GET /v1/movies and returns all movies in the database.
// It uses a DynamoDB Scan under the hood, which reads the whole table.
// This is fine for small datasets but will slow down and get expensive as the table grows.
func (h *MovieHandler) GetAllMovies(w http.ResponseWriter, r *http.Request) {
	// Passing r.Context() means the DB call will be automatically cancelled
	// if the client disconnects or the 60s middleware timeout fires.
	movies, err := h.movieService.GetAll(r.Context())
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "movies fetched", movies)
}

// GetMovie handles GET /v1/movies/{movieId}.
// If the service returns nil data, we treat that as "not found" and return 404.
func (h *MovieHandler) GetMovie(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "movieId")

	movie, err := h.movieService.Get(r.Context(), id)
	if err != nil {
		errs.InternalServerError(w, r, err)
		return
	}

	if movie == nil {
		errs.NotFoundError(w, r, errors.New("movie not found"))
		return
	}

	Success(w, http.StatusOK, "movie fetched", movie)
}

// CreateMovie handles POST /v1/movies.
// Flow: parse multipart form -> build movie -> map files via helper -> save to storage -> create database record.
// Duplicate IDs are mapped to 409 Conflict for a clearer client contract.
func (h *MovieHandler) CreateMovie(w http.ResponseWriter, r *http.Request) {
	formValues, err := ParseMultipartForm(r)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	movie, err := ParseMovieFromForm(formValues)
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	posterInput, err := ExtractFile(r, "poster")
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	coverInput, err := ExtractFile(r, "cover")
	if err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	// Extract cast images (cast_image_{id})
	castImages := make(map[string]*domain.FileUploadInput)
	for _, cast := range movie.Casts {
		fieldName := "cast_image_" + cast.ID
		if img, err := ExtractFile(r, fieldName); err == nil && img != nil {
			castImages[cast.ID] = img
		}
	}

	// Extract company images (company_image_{id})
	companyImages := make(map[string]*domain.FileUploadInput)
	for _, company := range movie.ProductionCompanies {
		fieldName := "company_image_" + company.ID
		if img, err := ExtractFile(r, fieldName); err == nil && img != nil {
			companyImages[company.ID] = img
		}
	}

	newMovie, err := h.movieService.Create(r.Context(), movie, posterInput, coverInput, castImages, companyImages)
	if err != nil {
		if errors.Is(err, errs.ErrFileUploaderNotConfigured) {
			errs.InternalServerError(w, r, err)
			return
		}

		if isConditionalCheckFailed(err) {
			errs.ConflictError(w, r, err)
			return
		}

		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusCreated, "movie created", newMovie)
}

// DeleteMovie handles DELETE /v1/movies/{movieId}.
// If the movie does not exist, it returns 404; unexpected failures return 500.
func (h *MovieHandler) DeleteMovie(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "movieId")

	if err := h.movieService.Delete(r.Context(), id); err != nil {
		if isConditionalCheckFailed(err) {
			errs.NotFoundError(w, r, err)
			return
		}

		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusOK, "movie deleted", nil)
}



// isConditionalCheckFailed identifies DynamoDB conditional-write failures.
// We use this to map domain-level conflicts/missing-resource cases to 409/404
// instead of returning a generic 500.
func isConditionalCheckFailed(err error) bool {
	var conditionErr *types.ConditionalCheckFailedException
	return errors.As(err, &conditionErr)
}
