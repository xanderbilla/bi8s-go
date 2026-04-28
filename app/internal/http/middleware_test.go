package http

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMaxBytesJSON_RejectsLarge(t *testing.T) {
	mw := MaxBytesJSON(16)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Error("expected MaxBytesReader error reading oversized body")
		}
		w.WriteHeader(http.StatusOK)
	}))

	body := bytes.NewReader([]byte(strings.Repeat("a", 1024)))
	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
}

func TestMaxBytesJSON_AllowsSmallAndMultipart(t *testing.T) {
	mw := MaxBytesJSON(16)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected read err: %v", err)
		}
		if len(b) == 0 {
			t.Error("body empty")
		}
		w.WriteHeader(http.StatusOK)
	}))

	big := bytes.NewReader([]byte(strings.Repeat("a", 1024)))
	req := httptest.NewRequest("POST", "/", big)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("multipart blocked: status=%d", rec.Code)
	}
}

// TestRequestLogger_DoesNotLeakAuthorization asserts that the access log
// produced by RequestLogger does not include the value of the Authorization
// header (or any other request header). This locks the redaction invariant
// so that future changes that add header logging must explicitly redact
// sensitive headers.
func TestRequestLogger_DoesNotLeakAuthorization(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	const secret = "Bearer super-secret-token-do-not-log"
	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/anything", nil)
	req.Header.Set("Authorization", secret)
	req.Header.Set("Cookie", "session=do-not-log-cookie")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	out := buf.String()
	if strings.Contains(out, "super-secret-token-do-not-log") {
		t.Fatalf("Authorization value leaked into access log: %s", out)
	}
	if strings.Contains(out, "do-not-log-cookie") {
		t.Fatalf("Cookie value leaked into access log: %s", out)
	}
}
