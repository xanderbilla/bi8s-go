package authstub

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequireAuthIsPassThrough(t *testing.T) {
	called := false
	h := RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	}))

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/x", nil))

	if !called {
		t.Fatal("expected next handler to be invoked")
	}
	if rr.Code != http.StatusTeapot {
		t.Fatalf("expected status %d, got %d", http.StatusTeapot, rr.Code)
	}
	if rr.Header().Get("WWW-Authenticate") != "" {
		t.Fatal("authstub must not set WWW-Authenticate")
	}
}
