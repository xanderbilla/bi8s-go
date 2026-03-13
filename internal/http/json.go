package http

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/xanderbilla/bi8s-go/internal/response"
)

// Cap incoming request bodies at 1MB to protect against memory exhaustion attacks.
const maxRequestBodySize = 1_048_576 // 1MB

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
