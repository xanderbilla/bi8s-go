package search

import (
	"context"

	"github.com/xanderbilla/bi8s-go/internal/model"
)

type Provider interface {
	EnsureIndexes(ctx context.Context) error
	SearchContent(ctx context.Context, query string, contentType string, sort string, page, pageSize int) ([]model.MoviePublicList, int, error)
	SearchPeople(ctx context.Context, query string, sort string, page, pageSize int) ([]model.SearchPersonResult, int, error)
	IndexContent(ctx context.Context, movie model.Movie) error
	DeleteContent(ctx context.Context, id string) error
	IndexPerson(ctx context.Context, person model.Person) error
	DeletePerson(ctx context.Context, id string) error

	DocCount(ctx context.Context) (int64, error)

	MoreLikeThis(ctx context.Context, contentID string, page, pageSize int) ([]model.MoviePublicList, int, error)
}

type NoopProvider struct{}

const (
	SortRecent    = "recent"
	SortLatest    = "latest"
	SortAlphaAsc  = "alpha_asc"
	SortAlphaDesc = "alpha_desc"
)

func (NoopProvider) EnsureIndexes(ctx context.Context) error { return nil }

func (NoopProvider) SearchContent(ctx context.Context, query string, contentType string, sort string, page, pageSize int) ([]model.MoviePublicList, int, error) {
	return []model.MoviePublicList{}, 0, nil
}

func (NoopProvider) SearchPeople(ctx context.Context, query string, sort string, page, pageSize int) ([]model.SearchPersonResult, int, error) {
	return []model.SearchPersonResult{}, 0, nil
}

func (NoopProvider) IndexContent(ctx context.Context, movie model.Movie) error { return nil }

func (NoopProvider) DeleteContent(ctx context.Context, id string) error { return nil }

func (NoopProvider) IndexPerson(ctx context.Context, person model.Person) error { return nil }

func (NoopProvider) DeletePerson(ctx context.Context, id string) error { return nil }

func (NoopProvider) DocCount(ctx context.Context) (int64, error) { return 0, nil }

func (NoopProvider) MoreLikeThis(ctx context.Context, contentID string, page, pageSize int) ([]model.MoviePublicList, int, error) {
	return []model.MoviePublicList{}, 0, nil
}
