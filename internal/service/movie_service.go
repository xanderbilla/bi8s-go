package service

import (
	"context"

	"github.com/xanderbilla/bi8s-go/internal/repository"
)

// MovieService sits between the HTTP handlers and the repository.
// Business logic (validation, transformation, authorization) belongs here —
// not in the handler and not in the repository.
type MovieService struct {
	repo repository.MovieRepository
}

// NewMovieService creates a MovieService backed by the given repository.
func NewMovieService(repo repository.MovieRepository) *MovieService {
	return &MovieService{repo: repo}
}

// GetAll retrieves every movie. Add filtering, sorting, or pagination logic here
// when needed, without touching the handler or the repository.
func (s *MovieService) GetAll(ctx context.Context) ([]repository.Movie, error) {
	return s.repo.GetAll(ctx)
}

// Get retrieves a single movie by ID.
func (s *MovieService) Get(ctx context.Context, id string) (*repository.Movie, error) {
	return s.repo.Get(ctx, id)
}

// Create validates and stores a new movie.
func (s *MovieService) Create(ctx context.Context, movie repository.Movie) error {
	return s.repo.Create(ctx, movie)
}

// Delete removes a movie by ID.
func (s *MovieService) Delete(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}
