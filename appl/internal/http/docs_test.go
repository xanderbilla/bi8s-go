package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeOpenAPISpec(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	ServeOpenAPISpec(rec, httptest.NewRequest(http.MethodGet, "/v1/openapi.yaml", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/yaml") {
		t.Errorf("Content-Type = %q", ct)
	}
	if !strings.Contains(rec.Body.String(), "openapi: 3.0") {
		t.Error("body missing OpenAPI version line")
	}
}

func TestServeSwaggerUI(t *testing.T) {
	t.Parallel()
	rec := httptest.NewRecorder()
	ServeSwaggerUI(rec, httptest.NewRequest(http.MethodGet, "/v1/docs", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "swagger-ui") {
		t.Error("HTML missing swagger-ui marker")
	}
	if !strings.Contains(rec.Body.String(), "/v1/openapi.yaml") {
		t.Error("HTML must reference /v1/openapi.yaml")
	}
}
