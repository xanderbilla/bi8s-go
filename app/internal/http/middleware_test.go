package http

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func TestMaxBytesMultipart_RejectsLarge(t *testing.T) {
	mw := MaxBytesMultipart(16)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err == nil {
			t.Error("expected MaxBytesReader error reading oversized multipart body")
		}
		w.WriteHeader(http.StatusOK)
	}))
	body := bytes.NewReader([]byte(strings.Repeat("a", 1024)))
	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=x")
	handler.ServeHTTP(httptest.NewRecorder(), req)
}

func TestMaxBytesMultipart_IgnoresJSON(t *testing.T) {
	mw := MaxBytesMultipart(16)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected read err: %v", err)
		}
		if len(b) != 1024 {
			t.Errorf("expected 1024 bytes, got %d", len(b))
		}
		w.WriteHeader(http.StatusOK)
	}))
	body := bytes.NewReader([]byte(strings.Repeat("a", 1024)))
	req := httptest.NewRequest("POST", "/", body)
	req.Header.Set("Content-Type", "application/json")
	handler.ServeHTTP(httptest.NewRecorder(), req)
}

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

func TestRequestLogger_RedactsSensitiveQueryValues(t *testing.T) {
	prev := slog.Default()
	defer slog.SetDefault(prev)

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))

	handler := RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/v1/search?query=narcos&access_token=super-secret&apikey=123", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	out := buf.String()
	if strings.Contains(out, "super-secret") || strings.Contains(out, "123") {
		t.Fatalf("sensitive query value leaked into access log: %s", out)
	}
	if !strings.Contains(out, "%5BREDACTED%5D") {
		t.Fatalf("expected redaction marker in logs, got: %s", out)
	}
}

func TestSanitizeRequestID(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "valid id", in: "req-123_abc:1", want: "req-123_abc:1"},
		{name: "trim spaces", in: "  req-1  ", want: "req-1"},
		{name: "reject control char", in: "req\n1", want: ""},
		{name: "reject non-ascii", in: "réq-1", want: ""},
		{name: "reject too long", in: strings.Repeat("a", 129), want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := sanitizeRequestID(tt.in); got != tt.want {
				t.Fatalf("sanitizeRequestID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRedactQuery(t *testing.T) {
	in := url.Values{
		"query":        {"avatar"},
		"token":        {"secret"},
		"access_token": {"another-secret"},
	}
	out := redactQuery(in)

	if out.Get("query") != "avatar" {
		t.Fatalf("query should not be redacted, got %q", out.Get("query"))
	}
	if out.Get("token") != "[REDACTED]" {
		t.Fatalf("token should be redacted, got %q", out.Get("token"))
	}
	if out.Get("access_token") != "[REDACTED]" {
		t.Fatalf("access_token should be redacted, got %q", out.Get("access_token"))
	}
}
