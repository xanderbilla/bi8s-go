//go:build integration

package integration

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestEncoder_GetJob_NotFound verifies the encoder GET-by-id route is wired
// (chi → URL validator → handler → repo → error mapper) and returns a
// structured 404 envelope for an unknown job ID. The job ID must satisfy
// the JobIDValidator pattern (^job_[A-Za-z0-9-]+$).
func TestEncoder_GetJob_NotFound(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 5 * time.Second}
	url := baseURL() + "/v1/a/encoder/job_does-not-exist-" + nowSuffix()
	resp, err := client.Get(url)
	if err != nil {
		t.Skipf("local stack not reachable at %s: %v (run `make docker-up`)", baseURL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %s: got status %d, want 404; body=%s", url, resp.StatusCode, body)
	}
	assertErrorEnvelope(t, resp)
}

// TestEncoder_CreateJob_ValidationError verifies the encoder write pipeline
// rejects empty multipart bodies with a structured 400 envelope, confirming
// the rate-limited write path is wired and surfaces validation errors
// before reaching the encoder service.
func TestEncoder_CreateJob_ValidationError(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 5 * time.Second}
	body := strings.NewReader("")
	req, err := http.NewRequest(http.MethodPost, baseURL()+"/v1/a/encoder/", body)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "multipart/form-data; boundary=---bi8s-test")

	resp, err := client.Do(req)
	if err != nil {
		t.Skipf("local stack not reachable at %s: %v (run `make docker-up`)", baseURL(), err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest && resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("POST /v1/a/encoder/: got status %d, want 400 or 422", resp.StatusCode)
	}
	assertErrorEnvelope(t, resp)
}
