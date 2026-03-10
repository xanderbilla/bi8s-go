package http

import (
	"net/http"

	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/repository"
)

// MovieHandler handles movie-related HTTP routes.
type MovieHandler struct {
	App *app.Application
}

// GetMovies handles GET /v1/movies.
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

func (h *MovieHandler) GetMovie(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
}

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

func (h *MovieHandler) DeleteMovie(w http.ResponseWriter, r *http.Request) {
	// TODO: implement
}
