// Package http contains all exterior service boundaries connecting web routing logic, request decoding validations, and dependency injection wiring.
// HTTP handlers strictly relay formatted parameters down into domain services ensuring scalable separation of concerns generically.
package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/xanderbilla/bi8s-go/internal/app"
)

// Mount wires up all routes and middleware, then returns the finished HTTP handler.
// Think of this as the front door of the API — every request comes through here.
func Mount(app *app.Application) http.Handler {

	r := chi.NewRouter()

	// Return JSON for unmatched routes and unsupported methods so clients
	// always get the same response shape, even before hitting a handler.
	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		Error(w, http.StatusNotFound, "The requested resource was not found")
	})

	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		Error(w, http.StatusMethodNotAllowed, "Method not allowed for this endpoint")
	})

	// --- Middleware stack (runs on every request, in order) ---

	// Apply official chi CORS middleware using environment-driven allowed origins.
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   app.Config.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Access-Control-Request-Private-Network"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	if app.Config.CORSAllowPrivateNetwork {
		r.Use(allowPrivateNetworkPreflight)
	}

	// Attaches a unique X-Request-Id to each request so you can trace it through logs.
	r.Use(middleware.RequestID)

	// Reads X-Forwarded-For / X-Real-IP so r.RemoteAddr reflects the real client IP,
	// not the load balancer or proxy sitting in front.
	r.Use(middleware.RealIP)

	// Logs every request: method, path, status, latency, and the request ID.
	r.Use(middleware.Logger)

	// Catches any panic inside a handler, logs the stack trace, and returns a 500
	// instead of letting the whole server crash.
	r.Use(middleware.Recoverer)

	// Cancels the request context after 60 seconds. Prevents slow handlers from
	// holding onto connections forever.
	r.Use(middleware.Timeout(60 * time.Second))

	// --- Routes ---

	// Each handler is a struct that holds a pointer to the app, giving it access
	// to config, DB, and any other shared dependencies without using globals.
	healthHandler := &HealthHandler{env: app.Config.Env}
	movieHandler := &MovieHandler{movieService: app.MovieService}

	r.Route("/v1", func(r chi.Router) {
		// GET /v1/health — liveness check, returns the current environment name.
		r.Get("/health", healthHandler.HealthCheck)

		// GET /v1/movies — returns all movies from DynamoDB.
		r.Route("/movies", func(r chi.Router) {
			r.Get("/", movieHandler.GetAllMovies)
			r.Get("/{movieId}", movieHandler.GetMovie)
			r.Post("/", movieHandler.CreateMovie)
			r.Delete("/{movieId}", movieHandler.DeleteMovie)
		})
	})

	return r
}

func allowPrivateNetworkPreflight(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Private-Network") == "true" {
			w.Header().Set("Access-Control-Allow-Private-Network", "true")
			w.Header().Add("Vary", "Access-Control-Request-Private-Network")
		}
		next.ServeHTTP(w, r)
	})
}
