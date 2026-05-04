package ratelimit

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	goredis "github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) (*goredis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	t.Cleanup(mr.Close)
	c := goredis.NewClient(&goredis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = c.Close() })
	return c, mr
}

func TestRedisBackend_AllowsBurstThenDenies(t *testing.T) {
	client, _ := newTestRedis(t)
	b, err := NewRedisBackend(RedisBackendOptions{
		Client: client, Name: "test", Burst: 2, RefillPerSec: 1, Timeout: time.Second,
	})
	if err != nil {
		t.Fatalf("NewRedisBackend: %v", err)
	}
	ctx := context.Background()
	for i := 0; i < 2; i++ {
		ok, _, err := b.Allow(ctx, "k1")
		if err != nil || !ok {
			t.Fatalf("burst hit %d: ok=%v err=%v", i, ok, err)
		}
	}
	ok, retry, err := b.Allow(ctx, "k1")
	if err != nil {
		t.Fatalf("third call err: %v", err)
	}
	if ok {
		t.Fatalf("expected deny after burst")
	}
	if retry <= 0 || retry > 2*time.Second {
		t.Fatalf("unexpected retryAfter: %v", retry)
	}
}

func TestRedisBackend_RefillsOverTime(t *testing.T) {
	client, _ := newTestRedis(t)
	// Use a very high refill rate so a short real-time sleep yields a token.
	b, _ := NewRedisBackend(RedisBackendOptions{
		Client: client, Name: "rf", Burst: 1, RefillPerSec: 100, Timeout: time.Second,
	})
	ctx := context.Background()
	if ok, _, _ := b.Allow(ctx, "k"); !ok {
		t.Fatalf("first allow failed")
	}
	// Bucket is now empty. Wait ~50ms; with rate=100/s that yields ~5 tokens.
	time.Sleep(50 * time.Millisecond)
	if ok, _, _ := b.Allow(ctx, "k"); !ok {
		t.Fatalf("expected allow after refill")
	}
}

func TestRedisBackend_KeysAreScopedByName(t *testing.T) {
	client, _ := newTestRedis(t)
	a, _ := NewRedisBackend(RedisBackendOptions{Client: client, Name: "a", Burst: 1, RefillPerSec: 0, Timeout: time.Second})
	bb, _ := NewRedisBackend(RedisBackendOptions{Client: client, Name: "b", Burst: 1, RefillPerSec: 0, Timeout: time.Second})
	ctx := context.Background()
	if ok, _, _ := a.Allow(ctx, "shared"); !ok {
		t.Fatalf("a first should allow")
	}
	// Different name -> different bucket; should still allow.
	if ok, _, _ := bb.Allow(ctx, "shared"); !ok {
		t.Fatalf("b first should allow (separate namespace)")
	}
	// Re-hitting a's bucket must be denied.
	if ok, _, _ := a.Allow(ctx, "shared"); ok {
		t.Fatalf("a second should deny")
	}
}

func TestRedisBackend_FailModes(t *testing.T) {
	client, mr := newTestRedis(t)
	b, _ := NewRedisBackend(RedisBackendOptions{Client: client, Name: "fo", Burst: 5, RefillPerSec: 1, Timeout: 100 * time.Millisecond, FailMode: FailOpen})
	mr.Close() // simulate outage
	ok, _, err := b.Allow(context.Background(), "k")
	if err == nil {
		t.Fatalf("expected error when redis is down")
	}
	if !ok {
		t.Fatalf("FailOpen should still allow")
	}

	client2, mr2 := newTestRedis(t)
	bc, _ := NewRedisBackend(RedisBackendOptions{Client: client2, Name: "fc", Burst: 5, RefillPerSec: 1, Timeout: 100 * time.Millisecond, FailMode: FailClosed})
	mr2.Close()
	ok2, _, err2 := bc.Allow(context.Background(), "k")
	if err2 == nil {
		t.Fatalf("expected error when redis is down")
	}
	if ok2 {
		t.Fatalf("FailClosed should deny")
	}
}

func TestMiddleware_AllowsAndDeniesCorrectly(t *testing.T) {
	b := NewMemoryBackend(1, 0)
	mw := Middleware(b, 1, 0, Options{})
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rec1 := httptest.NewRecorder()
	req1 := httptest.NewRequest("GET", "/", nil)
	req1.RemoteAddr = "1.2.3.4:1"
	handler.ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first call status=%d", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/", nil)
	req2.RemoteAddr = "1.2.3.4:1"
	handler.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second call status=%d", rec2.Code)
	}
	if rec2.Header().Get("Retry-After") == "" {
		t.Fatalf("missing Retry-After header")
	}
}

func TestParseFailMode(t *testing.T) {
	for in, want := range map[string]FailMode{
		"":            FailOpen,
		"fail-open":   FailOpen,
		"fail-closed": FailClosed,
		"closed":      FailClosed,
		"deny":        FailClosed,
		"garbage":     FailOpen,
	} {
		if got := ParseFailMode(in); got != want {
			t.Errorf("ParseFailMode(%q)=%v want %v", in, got, want)
		}
	}
}

func TestMemoryFactory_BuildsIsolated(t *testing.T) {
	f := MemoryFactory{}
	a := f.NewBackend("a", 1, 0)
	b := f.NewBackend("b", 1, 0)
	defer a.Close()
	defer b.Close()
	ctx := context.Background()
	if ok, _, _ := a.Allow(ctx, "k"); !ok {
		t.Fatalf("a first should allow")
	}
	if ok, _, _ := a.Allow(ctx, "k"); ok {
		t.Fatalf("a second should deny")
	}
	if ok, _, _ := b.Allow(ctx, "k"); !ok {
		t.Fatalf("b first should allow (separate factory result)")
	}
}
