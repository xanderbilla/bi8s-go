package ratelimit

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestLimiterAllow(t *testing.T) {
	l := NewLimiter(2, 1)
	if !l.Allow() {
		t.Fatalf("first request should be allowed")
	}
	if !l.Allow() {
		t.Fatalf("second request should be allowed (burst)")
	}
	if l.Allow() {
		t.Fatalf("third request should be denied (bucket empty)")
	}
}

func TestRateLimiterCleanupEvictsIdle(t *testing.T) {
	rl := NewRateLimiter(10, 1)
	defer rl.Close()

	rl.mu.Lock()
	old := NewLimiter(10, 1)
	old.lastRefillTime = time.Now().Add(-time.Hour)
	rl.limiters["old"] = old
	rl.limiters["fresh"] = NewLimiter(10, 1)
	rl.mu.Unlock()

	rl.mu.Lock()
	rl.cleanup()
	rl.mu.Unlock()

	rl.mu.RLock()
	defer rl.mu.RUnlock()
	if _, ok := rl.limiters["old"]; ok {
		t.Fatalf("expected idle limiter to be evicted")
	}
	if _, ok := rl.limiters["fresh"]; !ok {
		t.Fatalf("expected fresh limiter to be retained")
	}
}

func TestRateLimiterCloseIdempotent(t *testing.T) {
	rl := NewRateLimiter(1, 1)
	rl.Close()
	rl.Close()
}

func TestGetClientIP_IgnoresUntrustedXForwardedFor(t *testing.T) {
	if err := SetTrustedProxies(nil); err != nil {
		t.Fatalf("reset trusted proxies: %v", err)
	}
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "203.0.113.5:55555"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.Header.Set("X-Real-IP", "5.6.7.8")
	if got := GetClientIP(r); got != "203.0.113.5" {
		t.Fatalf("expected RemoteAddr (header ignored), got %q", got)
	}
}

func TestGetClientIP_TrustedProxyHonored(t *testing.T) {
	if err := SetTrustedProxies([]string{"10.0.0.0/8"}); err != nil {
		t.Fatalf("set trusted proxies: %v", err)
	}
	defer SetTrustedProxies(nil)
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.7:443"
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	if got := GetClientIP(r); got != "1.2.3.4" {
		t.Fatalf("expected real client IP from XFF, got %q", got)
	}
}

func TestGetClientIP_ChainedProxiesSkipTrusted(t *testing.T) {
	if err := SetTrustedProxies([]string{"10.0.0.0/8", "192.168.0.0/16"}); err != nil {
		t.Fatalf("set trusted proxies: %v", err)
	}
	defer SetTrustedProxies(nil)
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "10.0.0.1:443"

	r.Header.Set("X-Forwarded-For", "9.9.9.9, 1.2.3.4, 192.168.1.1, 10.0.0.2")
	if got := GetClientIP(r); got != "1.2.3.4" {
		t.Fatalf("expected first non-trusted hop walking right-to-left, got %q", got)
	}
}

var _ = time.Second
