package errs

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/xanderbilla/bi8s-go/internal/response"
)

func decode(t *testing.T, rec *httptest.ResponseRecorder) response.Envelope {
	t.Helper()
	var env response.Envelope
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	return env
}

func TestWrite_APIError(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/x", nil)
	rec := httptest.NewRecorder()
	Write(rec, req, NewNotFound("movie"))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d", rec.Code)
	}
	env := decode(t, rec)
	if env.Error == nil || env.Error.Code != CodeNotFound || env.Path != "/v1/x" {
		t.Errorf("envelope = %+v", env)
	}
}

func TestWrite_UnknownErrorDoesNotLeak(t *testing.T) {
	rec := httptest.NewRecorder()
	Write(rec, httptest.NewRequest("GET", "/y", nil), errors.New("internal db secret table=xxx"))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d", rec.Code)
	}
	env := decode(t, rec)
	if env.Error == nil || env.Error.Code != CodeInternal {
		t.Errorf("code = %+v", env.Error)
	}
	if env.Message == "internal db secret table=xxx" {
		t.Error("internal error message must not be leaked to client")
	}
}

func TestNewValidation(t *testing.T) {
	rec := httptest.NewRecorder()
	details := []map[string]string{{"field": "title", "code": "required"}}
	Write(rec, httptest.NewRequest("POST", "/m", nil), NewValidation(details))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
	env := decode(t, rec)
	if env.Error == nil || env.Error.Code != CodeValidationFailed || env.Error.Context == nil {
		t.Errorf("envelope = %+v", env)
	}
}

func TestBadRequestError_PassesThroughAPIError(t *testing.T) {
	rec := httptest.NewRecorder()
	BadRequestError(rec, httptest.NewRequest("POST", "/m", nil), NewValidation([]string{"name required"}))
	env := decode(t, rec)
	if env.Error == nil || env.Error.Code != CodeValidationFailed {
		t.Errorf("expected validation passthrough, got %+v", env)
	}
}

func TestBadRequestError_DoesNotLeakUnknown(t *testing.T) {
	rec := httptest.NewRecorder()
	BadRequestError(rec, httptest.NewRequest("POST", "/m", nil), errors.New("internal sql syntax: SELECT * FROM secrets"))
	env := decode(t, rec)
	if env.Error == nil || env.Error.Code != CodeBadRequest {
		t.Fatalf("code = %+v", env.Error)
	}
	if env.Message == "" || env.Message == "internal sql syntax: SELECT * FROM secrets" {
		t.Errorf("internal cause leaked into message: %q", env.Message)
	}
}

func TestNotFoundError_DoesNotLeakUnknown(t *testing.T) {
	rec := httptest.NewRecorder()
	NotFoundError(rec, httptest.NewRequest("GET", "/m", nil), errors.New("dynamodb internal: stack=foo table=bar"))
	env := decode(t, rec)
	if env.Error == nil || env.Error.Code != CodeNotFound {
		t.Fatalf("code = %+v", env.Error)
	}
	if env.Message != "" && (env.Message == "dynamodb internal: stack=foo table=bar") {
		t.Errorf("internal cause leaked: %q", env.Message)
	}
}

func TestConflictError_DoesNotLeakUnknown(t *testing.T) {
	rec := httptest.NewRecorder()
	ConflictError(rec, httptest.NewRequest("PUT", "/m", nil), errors.New("ConditionalCheckFailedException: detail item exists pk=foo sk=bar"))
	env := decode(t, rec)
	if env.Error == nil || env.Error.Code != CodeConflict {
		t.Fatalf("code = %+v", env.Error)
	}
	if env.Message == "ConditionalCheckFailedException: detail item exists pk=foo sk=bar" {
		t.Errorf("internal cause leaked into message: %q", env.Message)
	}
}

func TestSafeMessage_KnownTypesPassThrough(t *testing.T) {
	rec := httptest.NewRecorder()
	BadRequestError(rec, httptest.NewRequest("POST", "/m", nil), &PerformerNotFoundError{ID: "abc"})
	env := decode(t, rec)
	if env.Message == "" || env.Error == nil || env.Error.Code != CodeBadRequest {
		t.Errorf("expected typed err to pass through, got %+v", env)
	}
}

func TestFrom_Mapping(t *testing.T) {
	cases := []struct {
		name string
		in   error
		code string
	}{
		{"nil", nil, ""},
		{"content not found", ErrContentNotFound, CodeNotFound},
		{"no encoding", ErrNoEncodingFound, CodeNotFound},
		{"no completed encoding", ErrNoCompletedEncoding, CodeNotFound},
		{"playback not available", ErrPlaybackNotAvailable, CodeNotFound},
		{"attribute name taken", ErrAttributeNameTaken, CodeConflict},
		{"file empty", ErrFileEmpty, CodeBadRequest},
		{"result too large", ErrResultTooLarge, CodeBadRequest},
		{"performer not found", &PerformerNotFoundError{ID: "x"}, CodeBadRequest},
		{"attribute not found", &AttributeNotFoundError{ID: "x"}, CodeBadRequest},
		{"unknown -> internal", errors.New("anything goes"), CodeInternal},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := From(tc.in)
			if tc.in == nil {
				if got != nil {
					t.Fatalf("From(nil) should be nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("From(%v) returned nil", tc.in)
			}
			if got.Code != tc.code {
				t.Errorf("From(%v) code = %q, want %q", tc.in, got.Code, tc.code)
			}
		})
	}
}

func TestFrom_PassesThroughAPIError(t *testing.T) {
	in := NewConflict("dup")
	got := From(in)
	if got != in {
		t.Errorf("From should return the same APIError pointer, got %+v", got)
	}
}
