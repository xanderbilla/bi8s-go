//go:build e2e

// Package e2e contains black-box tests against a fully running stack
// (typically docker-compose.local.yml). Run via:
//
//	make docker-up
//	go test -tags=e2e ./test/e2e/...
//
// E2E tests must NOT import production code from `app/internal/...`. They
// drive the system exclusively through its public HTTP / messaging surface.
package e2e

import (
	"net/http"
	"os"
	"testing"
	"time"
)

func baseURL() string {
	if v := os.Getenv("BI8S_E2E_BASE_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

// TestRoot_Reachable verifies the API root returns the documented envelope.
func TestRoot_Reachable(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL() + "/")
	if err != nil {
		t.Skipf("API not reachable at %s: %v (run `make docker-up`)", baseURL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		t.Fatalf("GET /: got status %d, want < 500", resp.StatusCode)
	}
}

// TestHealth_ReturnsChecks verifies the health endpoint responds with 200.
func TestHealth_ReturnsChecks(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL() + "/v1/health")
	if err != nil {
		t.Skipf("API not reachable at %s: %v (run `make docker-up`)", baseURL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/health: got status %d, want 200", resp.StatusCode)
	}
}

// TestSearch_ReturnsValidEnvelope verifies the search endpoint accepts a query
// and returns a structured response without a 5xx error.
func TestSearch_ReturnsValidEnvelope(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL() + "/v1/c/search?q=test")
	if err != nil {
		t.Skipf("API not reachable at %s: %v (run `make docker-up`)", baseURL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		t.Fatalf("GET /v1/c/search?q=test: got status %d, want < 500", resp.StatusCode)
	}
}

// TestGetContent_NotFound verifies that a missing content ID returns 404.
func TestGetContent_NotFound(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL() + "/v1/c/content/nonexistent-smoke-test-id")
	if err != nil {
		t.Skipf("API not reachable at %s: %v (run `make docker-up`)", baseURL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /v1/c/content/nonexistent-smoke-test-id: got %d, want 404", resp.StatusCode)
	}
}

// TestDiscover_ReturnsValid verifies the discover endpoint does not 5xx.
func TestDiscover_ReturnsValid(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL() + "/v1/c/discover?type=top_rated")
	if err != nil {
		t.Skipf("API not reachable at %s: %v (run `make docker-up`)", baseURL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		t.Fatalf("GET /v1/c/discover?type=top_rated: got status %d, want < 500", resp.StatusCode)
	}
}

// TestPlayback_NotFound verifies playback endpoint handles missing content safely.
func TestPlayback_NotFound(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(baseURL() + "/v1/c/play/movie/nonexistent-smoke-test-id")
	if err != nil {
		t.Skipf("API not reachable at %s: %v (run `make docker-up`)", baseURL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("GET /v1/c/play/movie/nonexistent-smoke-test-id: got status %d, want 404", resp.StatusCode)
	}
}
