package http

import (
	"testing"

	"github.com/xanderbilla/bi8s-go/internal/model"
)

func TestSortMoviesAlphabeticallyAsc(t *testing.T) {
	t.Parallel()

	items := []model.Movie{{ID: "2", Title: "Zulu"}, {ID: "1", Title: "Alpha"}, {ID: "3", Title: "Bravo"}}
	sortMoviesAlphabetically(items, false)

	if items[0].Title != "Alpha" || items[1].Title != "Bravo" || items[2].Title != "Zulu" {
		t.Fatalf("unexpected order: %+v", items)
	}
}

func TestSortMoviesAlphabeticallyDesc(t *testing.T) {
	t.Parallel()

	items := []model.Movie{{ID: "1", Title: "Alpha"}, {ID: "2", Title: "Zulu"}, {ID: "3", Title: "Bravo"}}
	sortMoviesAlphabetically(items, true)

	if items[0].Title != "Zulu" || items[1].Title != "Bravo" || items[2].Title != "Alpha" {
		t.Fatalf("unexpected order: %+v", items)
	}
}

func TestPaginateDiscoverMovies(t *testing.T) {
	t.Parallel()

	items := []model.Movie{{ID: "1"}, {ID: "2"}, {ID: "3"}}
	page, next := paginateDiscoverMovies(items, 1, 1)
	if len(page) != 1 || page[0].ID != "2" {
		t.Fatalf("unexpected page: %+v", page)
	}
	if next != "2" {
		t.Fatalf("unexpected next cursor: %q", next)
	}
}
