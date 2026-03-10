package http

import (
	"encoding/json"
	"errors"
	"net/http"
)

const maxRequestBodySize = 1_048_576 // 1MB

type Response struct {
	Status  int         `json:"status"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func JSON(w http.ResponseWriter, status int, payload interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	return json.NewEncoder(w).Encode(payload)
}

func Success(w http.ResponseWriter, status int, msg string, data interface{}) error {
	res := Response{
		Status:  status,
		Message: msg,
		Data:    data,
	}

	return JSON(w, status, res)
}

func Error(w http.ResponseWriter, status int, msg string) error {
	res := Response{
		Status:  status,
		Message: "request failed",
		Error:   msg,
	}

	return JSON(w, status, res)
}

func Decode(w http.ResponseWriter, r *http.Request, payload interface{}) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodySize)

	decode := json.NewDecoder(r.Body)
	decode.DisallowUnknownFields()

	if err := decode.Decode(payload); err != nil {
		return err
	}

	if decode.More() {
		return errors.New("body must contain only one JSON object")
	}

	return nil
}
