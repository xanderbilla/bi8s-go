package ratelimit

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMiddlewareReturnsEnvelopeOn429(t *testing.T) {
	t.Parallel()

	rl := NewRateLimiter(1, 0)
	defer rl.Close()

	h := rl.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/v1/health", nil)
	req.RemoteAddr = "127.0.0.1:1234"

	rec1 := httptest.NewRecorder()
	h.ServeHTTP(rec1, req)
	if rec1.Code != http.StatusOK {
		t.Fatalf("first request status = %d, want 200", rec1.Code)
	}

	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request status = %d, want 429", rec2.Code)
	}
	if rec2.Header().Get("Retry-After") == "" {
		t.Error("Retry-After header missing")
	}
	if got := rec2.Header().Get("X-RateLimit-Limit"); got != "1" {
		t.Errorf("X-RateLimit-Limit = %q, want %q", got, "1")
	}

	var env map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env["success"] != false {
		t.Errorf("success = %v, want false", env["success"])
	}
	if env["code"] != "RATE_LIMITED" {
		t.Errorf("code = %v, want RATE_LIMITED", env["code"])
	}
	if env["status"].(float64) != float64(http.StatusTooManyRequests) {
		t.Errorf("status field = %v, want 429", env["status"])
	}
}
