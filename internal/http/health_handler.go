package http

import (
	"net/http"
)

// HealthHandler handles health-related routes.
type HealthHandler struct {
	env string
}

// HealthCheck returns a simple 200 OK response with the current environment name.
// It's mainly used by load balancers, uptime monitors, or just to confirm the server is alive.
func (h *HealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	// Include the environment (e.g. "prod", "dev") so you know which instance you're hitting.
	data := map[string]string{
		"version": h.env,
	}

	Success(w, http.StatusOK, "Health check passed!", data)
}
