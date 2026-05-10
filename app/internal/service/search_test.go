package service

import (
	"context"
	"errors"
	"testing"

	"github.com/xanderbilla/bi8s-go/internal/model"
)

type fakeSearchProvider struct {
	contentItems []model.MoviePublicList
	contentTotal int
	contentErr   error

	peopleItems []model.SearchPersonResult
	peopleTotal int
	peopleErr   error
}

func (f fakeSearchProvider) EnsureIndexes(ctx context.Context) error { return nil }

func (f fakeSearchProvider) SearchContent(ctx context.Context, query string, contentType string, sort string, page, pageSize int) ([]model.MoviePublicList, int, error) {
	return f.contentItems, f.contentTotal, f.contentErr
}

func (f fakeSearchProvider) SearchPeople(ctx context.Context, query string, sort string, page, pageSize int) ([]model.SearchPersonResult, int, error) {
	return f.peopleItems, f.peopleTotal, f.peopleErr
}

func (f fakeSearchProvider) IndexContent(ctx context.Context, movie model.Movie) error { return nil }

func (f fakeSearchProvider) DeleteContent(ctx context.Context, id string) error { return nil }

func (f fakeSearchProvider) IndexPerson(ctx context.Context, person model.Person) error { return nil }

func (f fakeSearchProvider) DeletePerson(ctx context.Context, id string) error { return nil }
func (f fakeSearchProvider) DocCount(ctx context.Context) (int64, error)       { return 0, nil }
func (f fakeSearchProvider) MoreLikeThis(ctx context.Context, contentID string, page, pageSize int) ([]model.MoviePublicList, int, error) {
	return []model.MoviePublicList{}, 0, nil
}

func TestSearchGuardrails_PageExceedsMaximum(t *testing.T) {
	t.Parallel()

	svc := NewSearchService(fakeSearchProvider{}, true)
	_, err := svc.Search(context.Background(), SearchInput{Query: "jolie", In: SearchInAll, ContentPage: 501, ContentPageSize: 20, PeoplePage: 1, PeoplePageSize: 20})
	if err == nil {
		t.Fatal("expected page guardrail error")
	}
	if got := err.Error(); got != "content page exceeds maximum limit of 500" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestSearchGuardrails_DeepWindowExceeded(t *testing.T) {
	t.Parallel()

	svc := NewSearchService(fakeSearchProvider{}, true)
	_, err := svc.Search(context.Background(), SearchInput{Query: "jolie", In: SearchInAll, ContentPage: 200, ContentPageSize: 100, PeoplePage: 1, PeoplePageSize: 20})
	if err == nil {
		t.Fatal("expected deep window guardrail error")
	}
	if got := err.Error(); got != "content requested page window is too deep (max from=10000)" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestSearchGuardrails_FromPlusSizeOverflow(t *testing.T) {
	t.Parallel()

	svc := NewSearchService(fakeSearchProvider{}, true)
	_, err := svc.Search(context.Background(), SearchInput{Query: "jolie", In: SearchInAll, ContentPage: 101, ContentPageSize: 100, PeoplePage: 1, PeoplePageSize: 20})
	if err == nil {
		t.Fatal("expected from+size overflow guardrail error")
	}
	if got := err.Error(); got != "content requested page window is too deep (max from=10000)" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestSearchInAll_PartialResultWithWarning(t *testing.T) {
	t.Parallel()

	svc := NewSearchService(fakeSearchProvider{
		contentItems: []model.MoviePublicList{{ID: "m1", Title: "Salt"}},
		contentTotal: 1,
		peopleErr:    errors.New("backend timeout"),
	}, true)

	resp, err := svc.Search(context.Background(), SearchInput{Query: "jolie", In: SearchInAll, ContentPage: 1, ContentPageSize: 20, PeoplePage: 1, PeoplePageSize: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content.Count != 1 {
		t.Fatalf("expected content count 1, got %d", resp.Content.Count)
	}
	if resp.People.Count != 0 {
		t.Fatalf("expected people count 0, got %d", resp.People.Count)
	}
	if len(resp.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(resp.Warnings))
	}
	if resp.Warnings[0].Scope != warningScopePeople {
		t.Fatalf("expected warning scope %q, got %q", warningScopePeople, resp.Warnings[0].Scope)
	}
	if resp.Warnings[0].Code != warningCodePartialSearch {
		t.Fatalf("expected warning code %q, got %q", warningCodePartialSearch, resp.Warnings[0].Code)
	}
}

func TestSearchInAll_BothBackendsFail(t *testing.T) {
	t.Parallel()

	svc := NewSearchService(fakeSearchProvider{
		contentErr: errors.New("content down"),
		peopleErr:  errors.New("people down"),
	}, true)

	_, err := svc.Search(context.Background(), SearchInput{Query: "jolie", In: SearchInAll, ContentPage: 1, ContentPageSize: 20, PeoplePage: 1, PeoplePageSize: 20})
	if err == nil {
		t.Fatal("expected combined backend error")
	}
}

func TestSearchRejectsInvalidSort(t *testing.T) {
	t.Parallel()

	svc := NewSearchService(fakeSearchProvider{}, true)
	_, err := svc.Search(context.Background(), SearchInput{Query: "jolie", In: SearchInAll, Sort: "random", ContentPage: 1, ContentPageSize: 20, PeoplePage: 1, PeoplePageSize: 20})
	if err == nil {
		t.Fatal("expected invalid sort error")
	}
	if got := err.Error(); got != "invalid 'sort' value" {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestSearchRejectsCombinedSort(t *testing.T) {
	t.Parallel()

	svc := NewSearchService(fakeSearchProvider{}, true)
	_, err := svc.Search(context.Background(), SearchInput{Query: "jolie", In: SearchInMovie, Sort: "recent,alpha_asc", ContentPage: 1, ContentPageSize: 20})
	if err == nil {
		t.Fatal("expected invalid sort error")
	}
	if got := err.Error(); got != "invalid 'sort' value" {
		t.Fatalf("unexpected error: %s", got)
	}
}
