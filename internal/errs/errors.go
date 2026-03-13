package errs

import (
	"log"
	"net/http"

	"github.com/xanderbilla/bi8s-go/internal/response"
)

// writeError uses the shared response package so error payloads stay
// consistent across both handlers and centralized error helpers.
func writeError(w http.ResponseWriter, status int, msg string) {
	_ = response.Error(w, status, msg)
}

// InternalServerError logs the underlying failure details and returns
// a safe message to the client without leaking internal information.
func InternalServerError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Internal Server Error: %s path: %s error: %s", r.Method, r.URL.Path, err)
	writeError(w, http.StatusInternalServerError, "The server encountered a problem")
}

// BadRequestError is used when client input is invalid (for example malformed JSON).
// It logs full context and returns a client-actionable error message.
func BadRequestError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Bad Request Error: %s path: %s error: %s", r.Method, r.URL.Path, err)
	writeError(w, http.StatusBadRequest, err.Error())
}

// NotFoundError is used when a requested resource does not exist.
// It keeps the response format consistent with the rest of the API.
func NotFoundError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Not Found Error: %s path: %s error: %s", r.Method, r.URL.Path, err)
	writeError(w, http.StatusNotFound, err.Error())
}
