package http

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/repository"
)

// MovieHandler handles movie-related HTTP routes.
type MovieHandler struct {
	App *app.Application
}

// GetAllMovies handles GET /v1/movies and returns all movies in the database.
// It delegates to the service layer, which owns any business logic before hitting the DB.
func (h *MovieHandler) GetAllMovies(w http.ResponseWriter, r *http.Request) {

	// Pass the request context so the service (and underlying DB call)
	// respects timeouts and cancellations set by the middleware.
	movies, err := h.App.MovieService.GetAll(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, http.StatusOK, "movies fetched", movies)
}

// GetMovie handles GET /v1/movies/{movieId} and returns a single movie by its ID.
func (h *MovieHandler) GetMovie(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "movieId")

	movies, err := h.App.MovieService.Get(r.Context(), id)
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, http.StatusOK, "movie fetched", movies)

}

// CreateMovie handles POST /v1/movies.
// It reads the request body, builds a Movie struct, and saves it via the service layer.
func (h *MovieHandler) CreateMovie(w http.ResponseWriter, r *http.Request) {
	var payload repository.Movie

	if err := Decode(w, r, &payload); err != nil {
		Error(w, http.StatusBadRequest, err.Error())
		return
	}

	movie := &repository.Movie{
		ID:    payload.ID,
		Title: payload.Title,
		Year:  payload.Year,
	}

	if err := h.App.MovieService.Create(r.Context(), *movie); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, http.StatusCreated, "movie created", movie)

}

// DeleteMovie handles DELETE /v1/movies/{movieId} and removes a movie from the database.
func (h *MovieHandler) DeleteMovie(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "movieId")

	if err := h.App.MovieService.Delete(r.Context(), id); err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, http.StatusOK, "movie deleted", nil)

}
