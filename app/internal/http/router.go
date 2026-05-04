package http

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/env"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/http/middleware/ratelimit"
)

func Mount(application *app.Application) (http.Handler, func()) {
	r := chi.NewRouter()

	r.NotFound(func(w http.ResponseWriter, r *http.Request) {
		errs.Write(w, r, errs.NewNotFound(""))
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		errs.Write(w, r, &errs.APIError{
			Status:  http.StatusMethodNotAllowed,
			Code:    "METHOD_NOT_ALLOWED",
			Message: "Method not allowed for this endpoint",
		})
	})

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   application.Config.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "Access-Control-Request-Private-Network"},
		ExposedHeaders:   []string{"Link", "X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	if application.Config.CORSAllowPrivateNetwork {
		r.Use(allowPrivateNetworkPreflight)
	}

	r.Use(RequestIDMiddleware)
	if application.HTTPMetrics != nil {
		r.Use(application.HTTPMetrics.Middleware)
	}
	r.Use(RequestLogger)
	r.Use(middleware.Recoverer)
	r.Use(SecureHeaders)

	r.Use(MaxBytesJSON(int64(env.GetInt("HTTP_MAX_JSON_BYTES", 1<<20))))

	globalBurst := env.GetInt("RATELIMIT_GLOBAL_BURST", 100)
	globalRPM := env.GetInt("RATELIMIT_GLOBAL_PER_MIN", 100)
	encoderBurst := env.GetInt("RATELIMIT_ENCODER_WRITE_BURST", 5)
	encoderRPM := env.GetInt("RATELIMIT_ENCODER_WRITE_PER_MIN", 5)
	movieBurst := env.GetInt("RATELIMIT_MOVIE_WRITE_BURST", 20)
	movieRPM := env.GetInt("RATELIMIT_MOVIE_WRITE_PER_MIN", 20)
	personBurst := env.GetInt("RATELIMIT_PERSON_WRITE_BURST", 20)
	personRPM := env.GetInt("RATELIMIT_PERSON_WRITE_PER_MIN", 20)

	factory := application.RateLimitFactory
	if factory == nil {
		factory = ratelimit.MemoryFactory{}
	}
	globalBackend := factory.NewBackend("global", float64(globalBurst), float64(globalRPM)/60.0)
	encoderBackend := factory.NewBackend("encoder_write", float64(encoderBurst), float64(encoderRPM)/60.0)
	movieBackend := factory.NewBackend("movie_write", float64(movieBurst), float64(movieRPM)/60.0)
	personBackend := factory.NewBackend("person_write", float64(personBurst), float64(personRPM)/60.0)

	r.Use(ratelimit.Middleware(globalBackend, float64(globalBurst), float64(globalRPM)/60.0, ratelimit.Options{}))

	encoderWriteMW := ratelimit.Middleware(encoderBackend, float64(encoderBurst), float64(encoderRPM)/60.0, ratelimit.Options{})
	movieWriteMW := ratelimit.Middleware(movieBackend, float64(movieBurst), float64(movieRPM)/60.0, ratelimit.Options{})
	personWriteMW := ratelimit.Middleware(personBackend, float64(personBurst), float64(personRPM)/60.0, ratelimit.Options{})

	stdTimeout := middleware.Timeout(time.Duration(env.GetInt("ROUTER_TIMEOUT_SECONDS", 60)) * time.Second)

	healthHandler := &HealthHandler{env: application.Config.Env, healthChecks: application.HealthChecks}
	contentHandler := NewContentHandler(application.MovieService)
	personHandler := NewPersonHandler(application.PersonService)
	attrHandler := NewAttributeHandler(application.AttributeService)
	encoderHandler := NewEncoderHandler(application.EncoderService)

	r.Route("/v1", func(r chi.Router) {

		r.Get("/health", healthHandler.HealthCheck)

		r.Get("/livez", healthHandler.Liveness)

		r.Get("/readyz", healthHandler.Readiness)

		r.Get("/openapi.yaml", ServeOpenAPISpec)
		r.Get("/docs", ServeSwaggerUI)

		r.Route("/c", func(r chi.Router) {
			r.Use(stdTimeout)
			r.Get("/content", contentHandler.GetRecentContent)
			r.With(ValidateURLParams(ContentIDValidator)).Get("/content/{contentId}", contentHandler.GetContent)
			r.With(ValidateURLParams(PersonIDValidator)).Get("/people/{peopleId}", personHandler.GetPerson)
			r.With(ValidateURLParams(PersonIDValidator)).Get("/people/{peopleId}/content", contentHandler.GetContentByPersonId)
			r.Get("/banner", contentHandler.GetBanner)
			r.With(ValidateURLParams(ConsumerAttributeIDValidator)).Get("/attributes/{id}", contentHandler.GetContentByAttributeId)
			r.Get("/discover", contentHandler.GetDiscoverContent)
			r.With(ValidateURLParams(ContentTypeValidator, ContentIDValidator)).Get("/play/{contentType}/{contentId}", contentHandler.GetPlayback)
		})

		r.Route("/a", func(r chi.Router) {

			r.With(ValidateURLParams(ContentIDValidator)).Post("/content/{contentId}", contentHandler.UploadAssets)

			r.Route("/encoder", func(r chi.Router) {

				r.With(encoderWriteMW).Post("/", encoderHandler.CreateEncodingJob)
				r.With(stdTimeout, ValidateURLParams(JobIDValidator)).Get("/{jobId}", encoderHandler.GetEncodingJob)
			})

			r.Group(func(r chi.Router) {
				r.Use(stdTimeout)

				r.Route("/content", func(r chi.Router) {
					r.Get("/", contentHandler.GetAllContentAdmin)
					r.With(ValidateURLParams(MovieIDValidator)).Get("/{contentId}", contentHandler.GetContentAdmin)
					r.With(movieWriteMW).Post("/", contentHandler.CreateContent)
					r.With(ValidateURLParams(MovieIDValidator)).Delete("/{contentId}", contentHandler.DeleteContent)
				})

				r.Route("/people", func(r chi.Router) {
					r.Get("/", personHandler.GetAllPeople)
					r.With(ValidateURLParams(PersonIDValidator)).Get("/{peopleId}", personHandler.GetPerson)
					r.With(personWriteMW).Post("/", personHandler.CreatePerson)
					r.With(ValidateURLParams(PersonIDValidator)).Delete("/{peopleId}", personHandler.DeletePerson)
					r.With(ValidateURLParams(PersonIDValidator)).Get("/{peopleId}/content", contentHandler.GetContentByPersonIdAdmin)
				})

				r.Route("/attributes", func(r chi.Router) {
					r.Get("/", attrHandler.GetAllAttributes)
					r.With(ValidateURLParams(AttributeIDValidator)).Get("/{attributeId}", attrHandler.GetAttribute)
					r.Post("/", attrHandler.CreateAttribute)
					r.With(ValidateURLParams(AttributeIDValidator)).Delete("/{attributeId}", attrHandler.DeleteAttribute)
				})
			})
		})
	})

	cleanup := func() {
		_ = globalBackend.Close()
		_ = encoderBackend.Close()
		_ = movieBackend.Close()
		_ = personBackend.Close()
	}

	handler := otelhttp.NewHandler(r, "bi8s-api",
		otelhttp.WithSpanNameFormatter(func(_ string, req *http.Request) string {
			if rctx := chi.RouteContext(req.Context()); rctx != nil {
				if pattern := rctx.RoutePattern(); pattern != "" {
					return req.Method + " " + pattern
				}
			}
			return req.Method + " " + req.URL.Path
		}),
	)
	return handler, cleanup
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
