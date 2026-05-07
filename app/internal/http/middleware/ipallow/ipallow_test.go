package ipallow

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

func TestAllow_PermitsMatchingIP(t *testing.T) {
	mw := Allow([]string{"10.0.0.0/8"})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "10.1.2.3:1234"
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestAllow_BlocksNonMatchingIP(t *testing.T) {
	mw := Allow([]string{"10.0.0.0/8"})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestAllow_EmptyListIsPassThrough(t *testing.T) {
	mw := Allow(nil)
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "203.0.113.5:443"
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for empty allow-list, got %d", rec.Code)
	}
}

func TestAllow_BareIPTreatedAsHostPrefix(t *testing.T) {
	mw := Allow([]string{"203.0.113.5"})
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.RemoteAddr = "203.0.113.5:443"
	rec := httptest.NewRecorder()
	mw(okHandler()).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
