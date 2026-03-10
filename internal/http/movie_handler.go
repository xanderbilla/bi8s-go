package http

import (
	"net/http"

	"github.com/xanderbilla/bi8s-go/internal/repository"
)

// GetMoviesHandler fetches all movies from the repository and returns them as JSON.
// The repo is injected as a dependency so this handler stays testable and decoupled from DynamoDB.
func GetMoviesHandler(repo repository.MovieRepository) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Pass the request context so the DB call respects timeouts and cancellations.
		movies, err := repo.GetAll(r.Context())
		if err != nil {
			Error(w, http.StatusInternalServerError, err.Error())
			return
		}

		Success(w, http.StatusOK, "movies fetched", movies)
	}
}