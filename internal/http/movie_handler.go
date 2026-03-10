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
// It uses a DynamoDB Scan under the hood, which reads the whole table.
// This is fine for small datasets but will slow down and get expensive as the table grows.
// See docs/todo.md (Scalability section) for the plan to fix this.
func (h *MovieHandler) GetAllMovies(w http.ResponseWriter, r *http.Request) {

	// Passing r.Context() means the DB call will be automatically cancelled
	// if the client disconnects or the 60s middleware timeout fires.
	movies, err := h.App.MovieService.GetAll(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, http.StatusOK, "movies fetched", movies)
}

// GetMovie handles GET /v1/movies/{movieId} and returns a single movie by its ID.
// Note: if the movie does not exist, this currently returns 200 with null data.
// It should return 404 — that fix is tracked in docs/todo.md.
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
// If the ID already exists in DynamoDB, the write will fail — duplicate IDs are rejected
// by a condition expression in the repository. See docs/todo.md for returning a proper 409 instead of 500.
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
		Error(w, http.StatusNotFound, err.Error())
		return
	}

	Success(w, http.StatusOK, "movie deleted", nil)

}
