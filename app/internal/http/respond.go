package http

import (
	"net/http"

	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/response"
)

func writeOK(w http.ResponseWriter, r *http.Request, status int, msg string, data any) {
	if err := response.Success(w, r, status, msg, data); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}
