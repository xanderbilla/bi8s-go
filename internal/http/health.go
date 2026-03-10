package http

import (
	"net/http"

	"github.com/xanderbilla/bi8s-go/internal/app"
)

// HealthCheckHandler returns a simple 200 OK response with the current environment name.
// It's mainly used by load balancers, uptime monitors, or just to confirm the server is alive.
func HealthCheckHandler(app *app.Application) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		// Include the environment (e.g. "prod", "dev") so you know which instance you're hitting.
		data := map[string]string{
			"version": app.Config.Env,
		}

		Success(w, http.StatusOK, "Health check passed!", data)
	}
}