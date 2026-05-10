package http

import (
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/xanderbilla/bi8s-go/internal/app"
)

func TestRouter_RouteSnapshot(t *testing.T) {
	t.Parallel()

	r, cleanup := buildRouter(&app.Application{})
	t.Cleanup(cleanup)

	var got []string
	walkFn := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		got = append(got, padMethod(method)+" "+route)
		return nil
	}
	if err := chi.Walk(r, walkFn); err != nil {
		t.Fatalf("chi.Walk: %v", err)
	}

	expected := []string{
		"GET    /v1/health",
		"GET    /v1/livez",
		"GET    /v1/readyz",
		"GET    /v1/openapi.yaml",
		"GET    /v1/docs",

		"GET    /v1/c/search",
		"GET    /v1/c/attributes",
		"GET    /v1/c/content/{contentId}",
		"GET    /v1/c/content/{contentId}/related",
		"GET    /v1/c/people/{peopleId}",
		"GET    /v1/c/people/{peopleId}/content",
		"GET    /v1/c/banner",
		"GET    /v1/c/attributes/{id}",
		"GET    /v1/c/discover",
		"GET    /v1/c/play/{contentType}/{contentId}",

		"POST   /v1/a/content/{contentId}",
		"POST   /v1/a/encoder/",
		"GET    /v1/a/encoder/{jobId}",

		"GET    /v1/a/content/",
		"GET    /v1/a/content/{contentId}",
		"POST   /v1/a/content/",
		"DELETE /v1/a/content/{contentId}",

		"GET    /v1/a/people/",
		"GET    /v1/a/people/{peopleId}",
		"POST   /v1/a/people/",
		"DELETE /v1/a/people/{peopleId}",
		"GET    /v1/a/people/{peopleId}/content",

		"GET    /v1/a/attributes/",
		"GET    /v1/a/attributes/{attributeId}",
		"POST   /v1/a/attributes/",
		"DELETE /v1/a/attributes/{attributeId}",
	}

	sort.Strings(got)
	sort.Strings(expected)

	if strings.Join(got, "\n") != strings.Join(expected, "\n") {
		t.Errorf("route table drift.\n got:\n%s\nwant:\n%s",
			strings.Join(got, "\n"), strings.Join(expected, "\n"))
	}
}

func padMethod(m string) string {
	for len(m) < 6 {
		m += " "
	}
	return m
}
