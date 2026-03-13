package response

import (
	"encoding/json"
	"net/http"
)

// Envelope is the shared API response shape used across the service.
// Both success and error responses use this format for predictable clients.
type Envelope struct {
	Status  int         `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// JSON writes any payload as JSON with the given HTTP status code.
// It is the lowest-level writer used by Success and Error.
func JSON(w http.ResponseWriter, status int, payload interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	return json.NewEncoder(w).Encode(payload)
}

// Success writes a successful response using the shared envelope.
func Success(w http.ResponseWriter, status int, msg string, data interface{}) error {
	res := Envelope{
		Status:  status,
		Message: msg,
		Data:    data,
	}

	return JSON(w, status, res)
}

// Error writes a failed response using the shared envelope.
// The message is intentionally stable so clients can handle errors consistently.
func Error(w http.ResponseWriter, status int, msg string) error {
	res := Envelope{
		Status:  status,
		Message: "request failed",
		Error:   msg,
	}

	return JSON(w, status, res)
}
