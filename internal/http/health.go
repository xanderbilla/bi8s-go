package http

import (
	"net/http"

	"github.com/xanderbilla/bi8s-go/internal/app"
)

func HealthCheckHandler(app *app.Application) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {

		data := map[string]string{
			"version": app.Config.Env,
		}

		Success(w, http.StatusOK, "Health check passed!", data)
	}
}