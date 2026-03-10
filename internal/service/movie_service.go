package service

import (
	"context"

	"github.com/xanderbilla/bi8s-go/internal/repository"
)

// MovieService sits between the HTTP handlers and the database layer.
// Any business logic — like validation, transformations, or access rules —
// should live here, keeping handlers thin and the database layer focused on data only.
type MovieService struct {
	repo repository.MovieRepository
}

// NewMovieService creates a new MovieService and wires it up with the given repository.
// Call this once at startup and pass the result into your handlers.
func NewMovieService(repo repository.MovieRepository) *MovieService {
	return &MovieService{repo: repo}
}

// GetAll returns every movie in the database.
// If you ever need filtering, sorting, or pagination, add that logic here
// rather than in the handler or the repository.
func (s *MovieService) GetAll(ctx context.Context) ([]repository.Movie, error) {
	return s.repo.GetAll(ctx)
}

// Get returns a single movie by its ID.
// Returns nil (with no error) if no movie with that ID exists.
func (s *MovieService) Get(ctx context.Context, id string) (*repository.Movie, error) {
	return s.repo.Get(ctx, id)
}

// Create saves a new movie to the database.
// Add any validation or default-value logic here before passing it down to the repository.
func (s *MovieService) Create(ctx context.Context, movie repository.Movie) error {
	return s.repo.Create(ctx, movie)
}

// Delete removes a movie from the database by its ID.
func (s *MovieService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
