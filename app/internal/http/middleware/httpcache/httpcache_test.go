package httpcache

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestMiddleware_HitsAfterFirstCall(t *testing.T) {
	var calls int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		_, _ = w.Write([]byte("body"))
	})
	mw := Middleware(NewMemoryStore(8), Options{TTL: time.Minute})

	for i := 0; i < 3; i++ {
		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		if rec.Code != 200 || rec.Body.String() != "body" {
			t.Fatalf("call %d: got %d %q", i, rec.Code, rec.Body.String())
		}
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Fatalf("expected 1 underlying call, got %d", got)
	}
}

func TestMiddleware_BypassesNonGet(t *testing.T) {
	var calls int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(200)
	})
	mw := Middleware(NewMemoryStore(8), Options{TTL: time.Minute})

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", nil))
		_ = rec
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("expected POST to bypass cache; got %d calls", got)
	}
}

func TestMiddleware_HonoursNoStoreRequest(t *testing.T) {
	var calls int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(200)
	})
	mw := Middleware(NewMemoryStore(8), Options{TTL: time.Minute})

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("Cache-Control", "no-store")
		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, req)
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("no-store should bypass; got %d calls", got)
	}
}

func TestMiddleware_DoesNotCacheNon2xx(t *testing.T) {
	var calls int32
	handler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(500)
	})
	mw := Middleware(NewMemoryStore(8), Options{TTL: time.Minute})

	for i := 0; i < 2; i++ {
		rec := httptest.NewRecorder()
		mw(handler).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	}
	if got := atomic.LoadInt32(&calls); got != 2 {
		t.Fatalf("5xx should not be cached; got %d calls", got)
	}
}

func TestMemoryStore_TTL(t *testing.T) {
	store := NewMemoryStore(8)
	store.Set("k", Entry{Status: 200, Body: []byte("x"), StoredAt: time.Now().Add(-time.Hour), TTL: time.Minute})
	if _, ok := store.Get("k"); ok {
		t.Fatal("expected expired entry to miss")
	}
}
