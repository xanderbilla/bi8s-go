package http

import (
	"net/http"

	"github.com/xanderbilla/bi8s-go/internal/app"
)

// MovieHandler handles movie-related HTTP routes.
type MovieHandler struct {
	App *app.Application
}

// GetMovies handles GET /v1/movies.
// It delegates to the service layer, which owns any business logic before hitting the DB.
func (h *MovieHandler) GetMovies(w http.ResponseWriter, r *http.Request) {

	// Pass the request context so the service (and underlying DB call)
	// respects timeouts and cancellations set by the middleware.
	movies, err := h.App.MovieService.GetAll(r.Context())
	if err != nil {
		Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	Success(w, http.StatusOK, "movies fetched", movies)
}
