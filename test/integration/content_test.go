//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// TestContent_GetAdmin_NotFound verifies that the admin GET-by-id route is
// wired end-to-end (chi → URL validator → handler → repo → error mapper)
// and returns a structured 404 envelope for an unknown content ID.
func TestContent_GetAdmin_NotFound(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 5 * time.Second}
	url := baseURL() + "/v1/a/content/does-not-exist-" + nowSuffix()
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

// TestContent_CreateAdmin_ValidationError verifies the create pipeline rejects
// empty multipart forms with a structured 400 (handler → parser →
// validation → error envelope), confirming the write path is wired and
// rate-limited middleware does not short-circuit valid traffic.
func TestContent_CreateAdmin_ValidationError(t *testing.T) {
	t.Parallel()

	client := &http.Client{Timeout: 5 * time.Second}
	body := strings.NewReader("")
	req, err := http.NewRequest(http.MethodPost, baseURL()+"/v1/a/content/", body)
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
		t.Fatalf("POST /v1/a/content/: got status %d, want 400 or 422", resp.StatusCode)
	}
	assertErrorEnvelope(t, resp)
}

// assertErrorEnvelope validates the response is a JSON error envelope with
// success=false, a non-empty error.code, and a non-empty error.title.
func assertErrorEnvelope(t *testing.T, resp *http.Response) {
	t.Helper()

	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("content-type = %q, want application/json", ct)
	}

	var env struct {
		Success bool `json:"success"`
		Status  int  `json:"status"`
		Error   *struct {
			Type  string `json:"type"`
			Code  string `json:"code"`
			Title string `json:"title"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	if env.Success {
		t.Fatalf("envelope.success = true, want false")
	}
	if env.Error == nil {
		t.Fatalf("envelope.error is nil")
	}
	if env.Error.Code == "" || env.Error.Title == "" {
		t.Fatalf("envelope.error missing code/title: %+v", env.Error)
	}
}

func nowSuffix() string {
	return time.Now().UTC().Format("20060102150405.000")
}
