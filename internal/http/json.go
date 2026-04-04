package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"

	"github.com/xanderbilla/bi8s-go/internal/domain"
	"github.com/xanderbilla/bi8s-go/internal/response"
	"github.com/xanderbilla/bi8s-go/internal/validation"
)

// Cap incoming request bodies at 1MB to protect against memory exhaustion attacks.

// Multipart form-data size limits
const (
	maxRequestBodySize      = 1_048_576     // 1MB
	maxMultipartBodySize    = 12_582_912    // 12MB total multipart body limit
	maxMultipartFileSize    = 10_485_760    // 10MB per file limit
	maxVideoMultipartBody   = 10_737_418_240 // 10GB for video uploads
	maxVideoFileSize        = 10_737_418_240 // 10GB per video file
)

// Response keeps backward compatibility for callers in this package while
// delegating the actual envelope definition to internal/response.
type Response = response.Envelope

// JSON is the low-level helper that serializes any payload to JSON and writes it
// to the response writer with the given HTTP status code.
func JSON(w http.ResponseWriter, status int, payload interface{}) error {
	return response.JSON(w, status, payload)
}

// Success wraps a successful result in the standard Response envelope and sends it.
// Use this whenever a handler completes without errors.
func Success(w http.ResponseWriter, status int, msg string, data interface{}) error {
	return response.Success(w, status, msg, data)
}

// Error wraps an error message in the standard Response envelope and sends it.
// The Message field is always "request failed" so clients have a stable string to check.
func Error(w http.ResponseWriter, status int, msg string) error {
	return response.Error(w, status, msg)
}

// Decode safely reads and parses a JSON request body into the given struct.
// It enforces a 1MB size limit, rejects unknown fields (typo protection),
// and makes sure the body contains exactly one JSON object — nothing more.
func Decode(w http.ResponseWriter, r *http.Request, payload interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	decode := json.NewDecoder(r.Body)
	decode.DisallowUnknownFields()

	if err := decode.Decode(payload); err != nil {
		return err
	}

	// Reject requests that try to sneak in extra JSON objects after the first one.
	if decode.More() {
		return errors.New("body must contain only one JSON object")
	}

	return nil
}

// ParseMultipartForm parses a multipart/form-data request.
// Returns the raw form values so the caller can extract domain models and files.
func ParseMultipartForm(r *http.Request) (url.Values, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxMultipartBodySize)

	if err := r.ParseMultipartForm(maxMultipartBodySize); err != nil {
		return nil, errors.New("body must be multipart/form-data")
	}

	return r.Form, nil
}

// ExtractFile simplifies extracting a file using the package's max file size limit.
func ExtractFile(r *http.Request, fieldName string) (*domain.FileUploadInput, error) {
	return validation.ExtractFile(r, fieldName, maxMultipartFileSize)
}

// ParseMultipartFormForVideo parses a multipart/form-data request for video uploads (up to 10GB).
func ParseMultipartFormForVideo(r *http.Request) (url.Values, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, maxVideoMultipartBody)

	if err := r.ParseMultipartForm(maxVideoMultipartBody); err != nil {
		return nil, errors.New("body must be multipart/form-data")
	}

	return r.Form, nil
}

// ExtractVideoFile extracts a video file with 10GB size limit.
func ExtractVideoFile(r *http.Request, fieldName string) (*domain.FileUploadInput, error) {
	return validation.ExtractFile(r, fieldName, maxVideoFileSize)
}

