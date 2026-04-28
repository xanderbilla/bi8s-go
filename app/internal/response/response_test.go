package response

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestSuccess(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/things", nil)
	rec := httptest.NewRecorder()
	if err := Success(rec, req, 200, "ok", map[string]string{"a": "b"}); err != nil {
		t.Fatalf("Success returned error: %v", err)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content-type = %q, want application/json", got)
	}
	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if !env.Success || env.Status != 200 || env.Message != "ok" {
		t.Errorf("unexpected envelope: %+v", env)
	}
	if env.Path != "/v1/things" {
		t.Errorf("path = %q", env.Path)
	}
	if env.Timestamp == "" {
		t.Error("timestamp must be set")
	}
}

func TestError(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/things", nil)
	rec := httptest.NewRecorder()
	if err := Error(rec, req, 400, "BAD_REQUEST", "bad", []string{"x"}); err != nil {
		t.Fatalf("Error returned error: %v", err)
	}
	if rec.Code != 400 {
		t.Errorf("status = %d, want 400", rec.Code)
	}
	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if env.Success || env.Code != "BAD_REQUEST" || env.Message != "bad" || env.Path != "/v1/things" {
		t.Errorf("unexpected envelope: %+v", env)
	}
	if env.Details == nil {
		t.Error("details must be present")
	}
}

func TestCreated_SetsLocation(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/things", nil)
	rec := httptest.NewRecorder()
	if err := Created(rec, req, "/v1/things/abc", "ok", map[string]string{"id": "abc"}); err != nil {
		t.Fatalf("Created: %v", err)
	}
	if rec.Code != 201 {
		t.Errorf("status = %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/v1/things/abc" {
		t.Errorf("location = %q", loc)
	}
	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Path != "/v1/things" {
		t.Errorf("path = %q", env.Path)
	}
}

func TestAccepted_SetsLocation(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/jobs", nil)
	rec := httptest.NewRecorder()
	if err := Accepted(rec, req, "/v1/jobs/job-1", "queued", nil); err != nil {
		t.Fatalf("Accepted: %v", err)
	}
	if rec.Code != 202 {
		t.Errorf("status = %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/v1/jobs/job-1" {
		t.Errorf("location = %q", loc)
	}
}

func TestSuccess_NilRequest(t *testing.T) {
	rec := httptest.NewRecorder()
	if err := Success(rec, nil, 200, "ok", nil); err != nil {
		t.Fatalf("Success: %v", err)
	}
	var env Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatal(err)
	}
	if env.Path != "" {
		t.Errorf("path should be empty for nil request, got %q", env.Path)
	}
}
