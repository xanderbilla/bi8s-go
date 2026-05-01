package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/xanderbilla/bi8s-go/internal/errs"
)

func TestSecureHeaders(t *testing.T) {
	t.Parallel()
	h := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	want := map[string]string{
		"X-Content-Type-Options":       "nosniff",
		"X-Frame-Options":              "DENY",
		"Referrer-Policy":              "no-referrer",
		"Cross-Origin-Opener-Policy":   "same-origin",
		"Cross-Origin-Resource-Policy": "same-origin",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("header %s = %q, want %q", k, got, v)
		}
	}
	pp := rec.Header().Get("Permissions-Policy")
	for _, frag := range []string{"camera=()", "microphone=()", "geolocation=()", "payment=()"} {
		if !strings.Contains(pp, frag) {
			t.Errorf("Permissions-Policy missing %q (got %q)", frag, pp)
		}
	}
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Errorf("HSTS should not be set on plain HTTP requests")
	}
}

func TestSecureHeaders_HSTSOnHTTPSForwardedProto(t *testing.T) {
	t.Parallel()
	h := SecureHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-Proto", "https")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if got := rec.Header().Get("Strict-Transport-Security"); got == "" {
		t.Error("expected HSTS header on https request")
	}
}

func TestNotFoundEnvelope(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		errs.Write(w, r, errs.NewNotFound(""))
	})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env["success"] != false {
		t.Errorf("success = %v, want false", env["success"])
	}
	if env["code"] != "NOT_FOUND" {
		t.Errorf("code = %v, want NOT_FOUND", env["code"])
	}
}

func TestMethodNotAllowedEnvelope(t *testing.T) {
	t.Parallel()
	r := chi.NewRouter()
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		errs.Write(w, r, &errs.APIError{
			Status:  http.StatusMethodNotAllowed,
			Code:    "METHOD_NOT_ALLOWED",
			Message: "Method not allowed for this endpoint",
		})
	})
	r.Get("/x", func(w http.ResponseWriter, _ *http.Request) {})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/x", nil))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", rec.Code)
	}
	var env map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env["code"] != "METHOD_NOT_ALLOWED" {
		t.Errorf("code = %v", env["code"])
	}
}
