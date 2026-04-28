package response

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"
)

type Envelope struct {
	Success   bool   `json:"success"`
	Status    int    `json:"status"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	Data      any    `json:"data,omitempty"`
	Details   any    `json:"details,omitempty"`
	Path      string `json:"path,omitempty"`
	RequestID string `json:"request_id,omitempty"`
	Timestamp string `json:"timestamp"`
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

func Error(w http.ResponseWriter, r *http.Request, status int, code, msg string, details any) error {
	return JSON(w, status, Envelope{
		Success:   false,
		Status:    status,
		Code:      code,
		Message:   msg,
		Details:   details,
		Path:      reqPath(r),
		RequestID: w.Header().Get("X-Request-ID"),
		Timestamp: nowRFC3339(),
	})
}

func reqPath(r *http.Request) string {
	if r == nil || r.URL == nil {
		return ""
	}
	return r.URL.Path
}

func nowRFC3339() string { return time.Now().UTC().Format(time.RFC3339Nano) }
