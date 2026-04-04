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
	personHandler := NewPersonHandler(app.PersonService)
	attributeHandler := NewAttributeHandler(app.AttributeService)
	encoderHandler := NewEncoderHandler(app.EncoderService)

	r.Route("/v1", func(r chi.Router) {
		// GET /v1/health — liveness check, returns the current environment name.
		r.Get("/health", healthHandler.HealthCheck)

		// Content routes
		r.Route("/c", func(r chi.Router) {
			// GET /v1/c/content?type=all — returns recent content sorted by creation date
			r.Get("/content", movieHandler.GetRecentContent)
			// GET /v1/c/content/{contentId} — returns single content by ID
			r.Get("/content/{contentId}", movieHandler.GetMovie)
			// GET /v1/c/people/{peopleId} — returns single person by ID
			r.Get("/people/{peopleId}", personHandler.GetPerson)
			// GET /v1/c/people/{peopleId}/content?type=all — returns content by person ID
			r.Get("/people/{peopleId}/content", movieHandler.GetContentByPersonId)
			// GET /v1/c/banner?type=all — returns random banner
			r.Get("/banner", movieHandler.GetBanner)
			// GET /v1/c/attributes/{id}?content=all — returns content by attribute ID
			r.Get("/attributes/{id}", movieHandler.GetContentByAttributeId)
			// GET /v1/c/discover?type=latest&content=all — discover content (latest, popular, trending)
			r.Get("/discover", movieHandler.GetDiscoverContent)
			// GET /v1/c/play/{contentType}/{contentId} — get playback information
			r.Get("/play/{contentType}/{contentId}", movieHandler.GetPlayback)
			
			// Encoder routes
			r.Route("/encoder", func(r chi.Router) {
				// POST /v1/c/encoder/new — create new video encoding job
				r.Post("/new", encoderHandler.CreateEncodingJob)
				// GET /v1/c/encoder/{jobId} — get encoding job details
				r.Get("/{jobId}", encoderHandler.GetEncodingJob)
			})
		})

		// Admin routes (no filtering, returns all fields including stats and audit)
		r.Route("/a", func(r chi.Router) {
			// Content asset upload
			r.Post("/content/{contentId}", movieHandler.UploadAssets)
			
			// Movie admin routes
			r.Route("/movies", func(r chi.Router) {
				r.Get("/", movieHandler.GetAllMoviesAdmin)
				r.Get("/{movieId}", movieHandler.GetMovieAdmin)
				r.Post("/", movieHandler.CreateMovie)
				r.Delete("/{movieId}", movieHandler.DeleteMovie)
			})

			// People admin routes
			r.Route("/people", func(r chi.Router) {
				r.Get("/", personHandler.GetAllPeople)
				r.Get("/{peopleId}", personHandler.GetPerson)
				r.Post("/", personHandler.CreatePerson)
				r.Delete("/{peopleId}", personHandler.DeletePerson)
				// Get content by person ID with type filter
				r.Get("/{peopleId}/content", movieHandler.GetContentByPersonIdAdmin)
			})

			// Attribute admin routes
			r.Route("/attributes", func(r chi.Router) {
				r.Get("/", attributeHandler.GetAllAttributes)
				r.Get("/{attributeId}", attributeHandler.GetAttribute)
				r.Post("/", attributeHandler.CreateAttribute)
				r.Delete("/{attributeId}", attributeHandler.DeleteAttribute)
			})
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
