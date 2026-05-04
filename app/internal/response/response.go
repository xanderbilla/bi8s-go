package response

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

// ErrorPayload is the structured error body embedded in the response envelope.
// Type groups errors at a high level (VALIDATION_ERROR, CLIENT_ERROR, ...);
// Code is the specific machine-readable code (NOT_FOUND, RATE_LIMITED, ...).
type ErrorPayload struct {
	Type        string `json:"type"`
	Code        string `json:"code"`
	Title       string `json:"title"`
	Detail      string `json:"detail"`
	UserMessage string `json:"userMessage,omitempty"`
	Context     any    `json:"context,omitempty"`
}

// Envelope is the single response shape used by every endpoint. Data is null
// on errors; Error is null on success. Both fields are always present so
// clients can rely on the schema.
type Envelope struct {
	Success   bool          `json:"success"`
	Status    int           `json:"status"`
	Message   string        `json:"message"`
	Data      any           `json:"data"`
	Error     *ErrorPayload `json:"error"`
	Path      string        `json:"path,omitempty"`
	RequestID string        `json:"requestId,omitempty"`
	Timestamp string        `json:"timestamp"`
}

func JSON(w http.ResponseWriter, status int, payload any) error {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err := w.Write(buf.Bytes())
	return err
}

func Success(w http.ResponseWriter, r *http.Request, status int, msg string, data any) error {
	return JSON(w, status, Envelope{
		Success:   true,
		Status:    status,
		Message:   msg,
		Data:      data,
		Error:     nil,
		Path:      reqPath(r),
		RequestID: w.Header().Get("X-Request-ID"),
		Timestamp: nowRFC3339(),
	})
}

func Created(w http.ResponseWriter, r *http.Request, location, msg string, data any) error {
	if location != "" {
		w.Header().Set("Location", location)
	}
	return Success(w, r, http.StatusCreated, msg, data)
}

func Accepted(w http.ResponseWriter, r *http.Request, location, msg string, data any) error {
	if location != "" {
		w.Header().Set("Location", location)
	}
	return Success(w, r, http.StatusAccepted, msg, data)
}

// Error writes a structured error envelope. msg becomes the top-level
// message and the error title/detail/userMessage; details (if non-nil)
// becomes ErrorPayload.Context.
func Error(w http.ResponseWriter, r *http.Request, status int, code, msg string, details any) error {
	return ErrorWith(w, r, status, &ErrorPayload{
		Type:        deriveType(status),
		Code:        code,
		Title:       msg,
		Detail:      msg,
		UserMessage: msg,
		Context:     details,
	})
}

// ErrorWith writes an error envelope with a fully-specified ErrorPayload.
// The top-level Message mirrors payload.Title for client convenience.
func ErrorWith(w http.ResponseWriter, r *http.Request, status int, payload *ErrorPayload) error {
	if payload == nil {
		payload = &ErrorPayload{Type: deriveType(status), Code: "UNKNOWN", Title: "unknown error", Detail: "unknown error"}
	}
	return JSON(w, status, Envelope{
		Success:   false,
		Status:    status,
		Message:   payload.Title,
		Data:      nil,
		Error:     payload,
		Path:      reqPath(r),
		RequestID: w.Header().Get("X-Request-ID"),
		Timestamp: nowRFC3339(),
	})
}

func deriveType(status int) string {
	switch {
	case status == http.StatusUnprocessableEntity || status == http.StatusBadRequest:
		return "VALIDATION_ERROR"
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return "AUTH_ERROR"
	case status == http.StatusNotFound:
		return "NOT_FOUND_ERROR"
	case status == http.StatusConflict:
		return "CONFLICT_ERROR"
	case status == http.StatusTooManyRequests:
		return "RATE_LIMIT_ERROR"
	case status >= 500:
		return "SERVER_ERROR"
	case status >= 400:
		return "CLIENT_ERROR"
	default:
		return "ERROR"
	}
}

func reqPath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	return r.URL.Path
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339Nano) }
