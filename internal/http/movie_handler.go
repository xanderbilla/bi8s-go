package http

import (
	"errors"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/repository"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

// MovieHandler handles movie-related HTTP routes.
// It keeps handler code focused on request/response flow, while shared packages
// handle validation and error formatting consistently.
type MovieHandler struct {
	App *app.Application
}

// GetAllMovies handles GET /v1/movies and returns all movies in the database.
// It uses a DynamoDB Scan under the hood, which reads the whole table.
// This is fine for small datasets but will slow down and get expensive as the table grows.
// See docs/todo.md (Scalability section) for the plan to fix this.
func (h *MovieHandler) GetAllMovies(w http.ResponseWriter, r *http.Request) {

	// Passing r.Context() means the DB call will be automatically cancelled
	// if the client disconnects or the 60s middleware timeout fires.
	movies, err := h.App.MovieService.GetAll(r.Context())
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

	movie, err := h.App.MovieService.Get(r.Context(), id)
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
// Flow: decode JSON -> validate payload -> create movie.
// Duplicate IDs are mapped to 409 Conflict for a clearer client contract.
func (h *MovieHandler) CreateMovie(w http.ResponseWriter, r *http.Request) {
	var payload repository.Movie

	if err := Decode(w, r, &payload); err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	if err := validation.ValidateStruct(payload); err != nil {
		errs.BadRequestError(w, r, err)
		return
	}

	movie, err := h.App.MovieService.Create(r.Context(), payload)
	if err != nil {
		if isConditionalCheckFailed(err) {
			errs.ConflictError(w, r, err)
			return
		}

		errs.InternalServerError(w, r, err)
		return
	}

	Success(w, http.StatusCreated, "movie created", movie)

}

// DeleteMovie handles DELETE /v1/movies/{movieId}.
// If the movie does not exist, it returns 404; unexpected failures return 500.
func (h *MovieHandler) DeleteMovie(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "movieId")

	if err := h.App.MovieService.Delete(r.Context(), id); err != nil {
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
