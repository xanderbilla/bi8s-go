package errs

import (
	"log"
	"net/http"

	"github.com/xanderbilla/bi8s-go/internal/response"
)

// writeError sends a standardized JSON error envelope.
// Keeping this in one place prevents response-shape drift between handlers.
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
// Returning the validation/parsing message helps clients fix their request quickly.
func BadRequestError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Bad Request Error: %s path: %s error: %s", r.Method, r.URL.Path, err)
	writeError(w, http.StatusBadRequest, err.Error())
}

// NotFoundError is used when a requested resource does not exist.
// We log internal details for debugging but return a generic client message.
func NotFoundError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Not Found Error: %s path: %s error: %s", r.Method, r.URL.Path, err)
	writeError(w, http.StatusNotFound, "The requested resource was not found")
}

// ConflictError is used when a request cannot be completed because of
// a resource state conflict (for example, creating a movie with an existing ID).
func ConflictError(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("Conflict Error: %s path: %s error: %s", r.Method, r.URL.Path, err)
	writeError(w, http.StatusConflict, "The resource already exists")
}
