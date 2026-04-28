package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestValidateID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "valid alphanumeric ID",
			id:      "movie123",
			wantErr: false,
		},
		{
			name:    "valid with underscore",
			id:      "movie_123",
			wantErr: false,
		},
		{
			name:    "valid with hyphen",
			id:      "movie-123",
			wantErr: false,
		},
		{
			name:    "empty ID",
			id:      "",
			wantErr: true,
		},
		{
			name:    "too long",
			id:      string(make([]byte, 101)),
			wantErr: true,
		},
		{
			name:    "path traversal attempt",
			id:      "../../../etc/passwd",
			wantErr: true,
		},
		{
			name:    "SQL injection attempt",
			id:      "movie'; DROP TABLE movies;--",
			wantErr: true,
		},
		{
			name:    "contains slash",
			id:      "movie/123",
			wantErr: true,
		},
		{
			name:    "contains backslash",
			id:      "movie\\123",
			wantErr: true,
		},
		{
			name:    "contains dot dot",
			id:      "movie..123",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateID() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        string
		wantValid   bool
	}{
		{
			name:        "movie lowercase",
			contentType: "movie",
			want:        "movie",
			wantValid:   true,
		},
		{
			name:        "movie uppercase",
			contentType: "MOVIE",
			want:        "movie",
			wantValid:   true,
		},
		{
			name:        "movies plural",
			contentType: "movies",
			want:        "movie",
			wantValid:   true,
		},
		{
			name:        "person lowercase",
			contentType: "person",
			want:        "person",
			wantValid:   true,
		},
		{
			name:        "persons plural",
			contentType: "persons",
			want:        "person",
			wantValid:   true,
		},
		{
			name:        "people",
			contentType: "people",
			want:        "person",
			wantValid:   true,
		},
		{
			name:        "invalid type",
			contentType: "invalid",
			want:        "",
			wantValid:   false,
		},
		{
			name:        "empty",
			contentType: "",
			want:        "",
			wantValid:   false,
		},
		{
			name:        "injection attempt",
			contentType: "movie'; DROP TABLE",
			want:        "",
			wantValid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, valid := ValidateContentType(tt.contentType)
			if got != tt.want {
				t.Errorf("ValidateContentType() got = %v, want %v", got, tt.want)
			}
			if valid != tt.wantValid {
				t.Errorf("ValidateContentType() valid = %v, want %v", valid, tt.wantValid)
			}
		})
	}
}

func TestValidateURLParams_Required(t *testing.T) {
	validator := URLParamValidator{
		ParamName: "id",
		Required:  true,
		ErrorMsg:  "ID is required",
	}

	middleware := ValidateURLParams(validator)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rctx := chi.NewRouteContext()
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestValidateURLParams_MaxLength(t *testing.T) {
	validator := URLParamValidator{
		ParamName: "id",
		MaxLength: 10,
		ErrorMsg:  "ID too long",
	}

	middleware := ValidateURLParams(validator)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "this_is_way_too_long")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status %d, got %d", http.StatusBadRequest, w.Code)
	}
}

func TestValidateURLParams_Pattern(t *testing.T) {
	validator := URLParamValidator{
		ParamName: "id",
		Pattern:   AlphanumericPattern,
		ErrorMsg:  "ID format invalid",
	}

	middleware := ValidateURLParams(validator)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		paramValue string
		wantStatus int
	}{
		{
			name:       "valid alphanumeric",
			paramValue: "movie123",
			wantStatus: http.StatusOK,
		},
		{
			name:       "invalid with special chars",
			paramValue: "movie@123",
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "path traversal",
			paramValue: "../../../etc",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", tt.paramValue)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestValidateURLParams_AllowedValues(t *testing.T) {
	validator := URLParamValidator{
		ParamName:   "contentType",
		AllowedVals: []string{"movie", "tv"},
		ErrorMsg:    "Invalid content type",
	}

	middleware := ValidateURLParams(validator)
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	tests := []struct {
		name       string
		paramValue string
		wantStatus int
	}{
		{
			name:       "allowed movie",
			paramValue: "movie",
			wantStatus: http.StatusOK,
		},
		{
			name:       "allowed tv",
			paramValue: "tv",
			wantStatus: http.StatusOK,
		},
		{
			name:       "not allowed",
			paramValue: "invalid",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("contentType", tt.paramValue)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("Expected status %d, got %d", tt.wantStatus, w.Code)
			}
		})
	}
}

func TestCommonValidators(t *testing.T) {

	if MovieIDValidator.ParamName != "movieId" {
		t.Error("MovieIDValidator should validate 'movieId' parameter")
	}

	if !MovieIDValidator.Required {
		t.Error("MovieIDValidator should be required")
	}

	if JobIDValidator.Pattern == nil {
		t.Error("JobIDValidator should have a pattern")
	}

	if len(ContentTypeValidator.AllowedVals) != 2 {
		t.Error("ContentTypeValidator should have 2 allowed values")
	}
}
