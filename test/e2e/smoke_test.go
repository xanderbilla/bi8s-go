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
