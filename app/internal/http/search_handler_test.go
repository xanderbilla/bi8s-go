package http

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xanderbilla/bi8s-go/internal/service"
)

func newEnabledSearchHandler() *SearchHandler {

	svc := service.NewSearchService(nil, true)
	return NewSearchHandler(svc)
}

func TestSearchHandlerMissingQuery(t *testing.T) {
	t.Parallel()
	h := &SearchHandler{searchService: service.NewSearchService(nil, false)}
	rec := httptest.NewRecorder()
	h.Search(rec, httptest.NewRequest(http.MethodGet, "/v1/c/search", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSearchHandlerInvalidIn(t *testing.T) {
	t.Parallel()
	h := newEnabledSearchHandler()
	rec := httptest.NewRecorder()
	h.Search(rec, httptest.NewRequest(http.MethodGet, "/v1/c/search?query=test&in=invalid", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSearchHandlerInvalidSort(t *testing.T) {
	t.Parallel()
	h := newEnabledSearchHandler()
	rec := httptest.NewRecorder()
	h.Search(rec, httptest.NewRequest(http.MethodGet, "/v1/c/search?query=test&sort=random", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSearchHandlerDisabledReturns500(t *testing.T) {
	t.Parallel()
	h := &SearchHandler{searchService: service.NewSearchService(nil, false)}
	rec := httptest.NewRecorder()
	h.Search(rec, httptest.NewRequest(http.MethodGet, "/v1/c/search?query=test", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestParseIntDefault(t *testing.T) {
	t.Parallel()
	cases := []struct {
		raw      string
		fallback int
		want     int
	}{
		{"", 1, 1},
		{"abc", 1, 1},
		{"-5", 10, 10},
		{"0", 5, 5},
		{"3", 1, 3},
		{"20", 1, 20},
	}
	for _, tc := range cases {
		got := parseIntDefault(tc.raw, tc.fallback)
		if got != tc.want {
			t.Errorf("parseIntDefault(%q, %d) = %d, want %d", tc.raw, tc.fallback, got, tc.want)
		}
	}
}

func TestIsSearchInputError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "invalid in", err: errors.New("invalid 'in' value"), want: true},
		{name: "invalid sort", err: errors.New("invalid 'sort' value"), want: true},
		{name: "page too high", err: errors.New("page exceeds maximum limit of 500"), want: true},
		{name: "window too deep", err: errors.New("requested page window is too deep (max from=10000)"), want: true},
		{name: "infra error", err: errors.New("search backend timeout"), want: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isSearchInputError(tc.err); got != tc.want {
				t.Fatalf("isSearchInputError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
