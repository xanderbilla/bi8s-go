package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestReadinessUnreadyReturns503(t *testing.T) {
	SetReady(false)
	defer SetReady(false)

	h := &HealthHandler{env: "test"}
	rec := httptest.NewRecorder()
	h.Readiness(rec, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", rec.Code)
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if errCode(env) != "NOT_READY" {
		t.Errorf("code = %v, want NOT_READY", errCode(env))
	}
}

func TestReadinessReadyDelegatesToHealthCheck(t *testing.T) {
	SetReady(true)
	defer SetReady(false)

	h := &HealthHandler{env: "test", healthChecks: nil}
	rec := httptest.NewRecorder()
	h.Readiness(rec, httptest.NewRequest(http.MethodGet, "/v1/readyz", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (no checks registered)", rec.Code)
	}
}

func TestLivenessOmitsEnv(t *testing.T) {
	t.Parallel()
	h := &HealthHandler{env: "production"}
	rec := httptest.NewRecorder()
	h.Liveness(rec, httptest.NewRequest(http.MethodGet, "/v1/livez", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if data, ok := env["data"].(map[string]any); ok {
		if _, exists := data["env"]; exists {
			t.Errorf("liveness data must not include env (got %v)", data)
		}
	}
}
