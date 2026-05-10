package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xanderbilla/bi8s-go/internal/app"
)

func BenchmarkRouterDispatch(b *testing.B) {
	r, cleanup := buildRouter(&app.Application{})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/c/content/bench", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
		w.Body.Reset()
	}
}

func BenchmarkRouterLivez(b *testing.B) {
	r, cleanup := buildRouter(&app.Application{})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/livez", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
		w.Body.Reset()
	}
}

func BenchmarkRouter404(b *testing.B) {
	r, cleanup := buildRouter(&app.Application{})
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/v1/does/not/exist", nil)
	w := httptest.NewRecorder()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		r.ServeHTTP(w, req)
		w.Body.Reset()
	}
}
