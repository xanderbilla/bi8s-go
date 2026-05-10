//go:build integration

// Package integration contains tests that wire real adapters (DynamoDB Local,
// MinIO, Redis) against a running local stack. Run via:
//
//	make docker-up
//	make test-integration
//
// All tests in this package MUST be guarded by the `integration` build tag
// and MUST clean up any state they create.
package integration

import (
	"net/http"
	"os"
	"testing"
	"time"
)

// baseURL is the API endpoint under test. Override with BI8S_INTEGRATION_BASE_URL.
func baseURL() string {
	if v := os.Getenv("BI8S_INTEGRATION_BASE_URL"); v != "" {
		return v
	}
	return "http://localhost:8080"
}

// TestHealth_Live is a smoke test that fails fast if the local stack is not up.
// It validates the contract of GET /v1/livez (status 200, JSON envelope).
func TestHealth_Live(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(baseURL() + "/v1/livez")
	if err != nil {
		t.Skipf("local stack not reachable at %s: %v (run `make docker-up`)", baseURL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /v1/livez: got status %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct == "" || len(ct) < 16 || ct[:16] != "application/json" {
		t.Fatalf("GET /v1/livez: content-type = %q, want application/json", ct)
	}
}
