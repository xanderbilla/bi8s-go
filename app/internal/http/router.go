package http

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/http/middleware/ratelimit"
	"github.com/xanderbilla/bi8s-go/internal/http/routes"
)

func Mount(application *app.Application) (http.Handler, func()) {
	r, cleanup := buildRouter(application)
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

func buildRouter(application *app.Application) (*chi.Mux, func()) {
	r := chi.NewRouter()
	allowOrigin := newCORSOriginMatcher(application.Config.CORSAllowedOrigins)

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
		AllowOriginFunc:  allowOrigin,
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

	r.Use(MaxBytesJSON(int64(application.Config.HTTPMaxJSONBytes)))
	r.Use(MaxBytesMultipart(application.Config.HTTPMaxMultipartBytes))

	factory := application.RateLimitFactory
	if factory == nil {
		factory = ratelimit.MemoryFactory{}
	}

	buildRateLimit := func(name string, burst, perMin int) (ratelimit.Backend, func(http.Handler) http.Handler) {
		b := float64(burst)
		rps := float64(perMin) / 60.0
		backend := factory.NewBackend(name, b, rps)
		return backend, ratelimit.Middleware(backend, b, rps, ratelimit.Options{})
	}

	globalBackend, globalMW := buildRateLimit("global", application.Config.RateLimitGlobalBurst, application.Config.RateLimitGlobalPerMin)
	encoderBackend, encoderWriteMW := buildRateLimit("encoder_write", application.Config.RateLimitEncoderBurst, application.Config.RateLimitEncoderPerMin)
	movieBackend, movieWriteMW := buildRateLimit("movie_write", application.Config.RateLimitMovieBurst, application.Config.RateLimitMoviePerMin)
	personBackend, personWriteMW := buildRateLimit("person_write", application.Config.RateLimitPersonBurst, application.Config.RateLimitPersonPerMin)

	r.Use(globalMW)

	stdTimeout := middleware.Timeout(time.Duration(application.Config.RouterTimeoutSecond) * time.Second)

	healthHandler := &HealthHandler{env: application.Config.Env, healthChecks: application.HealthChecks}
	contentHandler := NewContentHandler(application.ContentService)
	personHandler := NewPersonHandler(application.PersonService)
	searchHandler := NewSearchHandler(application.SearchService)
	attrHandler := NewAttributeHandler(application.AttributeService)
	encoderHandler := NewEncoderHandler(application.EncoderService)

	handlers := routes.HandlerRegistry{
		"health.HealthCheck":          healthHandler.HealthCheck,
		"health.Liveness":             healthHandler.Liveness,
		"health.Readiness":            healthHandler.Readiness,
		"docs.OpenAPISpec":            ServeOpenAPISpec,
		"docs.SwaggerUI":              ServeSwaggerUI,
		"search.Search":               searchHandler.Search,
		"search.MoreLikeThis":         searchHandler.MoreLikeThis,
		"attribute.ListConsumer":      attrHandler.GetConsumerAttributes,
		"attribute.ListAdmin":         attrHandler.GetAllAttributes,
		"attribute.GetAdmin":          attrHandler.GetAttribute,
		"attribute.Create":            attrHandler.CreateAttribute,
		"attribute.Delete":            attrHandler.DeleteAttribute,
		"content.GetConsumer":         contentHandler.GetContent,
		"content.GetByPersonConsumer": contentHandler.GetContentByPersonId,
		"content.GetBanner":           contentHandler.GetBanner,
		"content.GetByAttribute":      contentHandler.GetContentByAttributeId,
		"content.GetDiscover":         contentHandler.GetDiscoverContent,
		"content.GetPlayback":         contentHandler.GetPlayback,
		"content.UploadAssets":        contentHandler.UploadAssets,
		"content.ListAdmin":           contentHandler.GetAllContentAdmin,
		"content.GetAdmin":            contentHandler.GetContentAdmin,
		"content.Create":              contentHandler.CreateContent,
		"content.Delete":              contentHandler.DeleteContent,
		"content.GetByPersonAdmin":    contentHandler.GetContentByPersonIdAdmin,
		"person.GetConsumer":          personHandler.GetPerson,
		"person.ListAdmin":            personHandler.GetAllPeople,
		"person.GetAdmin":             personHandler.GetPerson,
		"person.Create":               personHandler.CreatePerson,
		"person.Delete":               personHandler.DeletePerson,
		"encoder.Create":              encoderHandler.CreateEncodingJob,
		"encoder.Get":                 encoderHandler.GetEncodingJob,
	}

	staticMW := func(mw func(http.Handler) http.Handler) routes.MiddlewareConstructor {
		return func(_ routes.Route) func(http.Handler) http.Handler { return mw }
	}
	mws := routes.MiddlewareRegistry{
		"timeout":                      staticMW(stdTimeout),
		"ratelimit.encoder":            staticMW(encoderWriteMW),
		"ratelimit.movie":              staticMW(movieWriteMW),
		"ratelimit.person":             staticMW(personWriteMW),
		"validate.contentId":           staticMW(ValidateURLParams(ContentIDValidator)),
		"validate.personId":            staticMW(ValidateURLParams(PersonIDValidator)),
		"validate.movieId":             staticMW(ValidateURLParams(MovieIDValidator)),
		"validate.attributeId":         staticMW(ValidateURLParams(AttributeIDValidator)),
		"validate.consumerAttributeId": staticMW(ValidateURLParams(ConsumerAttributeIDValidator)),
		"validate.jobId":               staticMW(ValidateURLParams(JobIDValidator)),
		"validate.contentTypeAndId":    staticMW(ValidateURLParams(ContentTypeValidator, ContentIDValidator)),
	}

	cfg, err := loadRoutesConfig()
	if err != nil {
		panic(fmt.Errorf("router: load routes config: %w", err))
	}
	if err := routes.Validate(cfg, handlers, mws); err != nil {
		panic(fmt.Errorf("router: validate routes config: %w", err))
	}
	if err := routes.Mount(r, cfg, handlers, mws); err != nil {
		panic(fmt.Errorf("router: mount routes: %w", err))
	}

	cleanup := func() {
		_ = globalBackend.Close()
		_ = encoderBackend.Close()
		_ = movieBackend.Close()
		_ = personBackend.Close()
	}

	return r, cleanup
}

func loadRoutesConfig() (routes.Config, error) {
	if path := os.Getenv("ROUTES_CONFIG_PATH"); path != "" {
		return routes.LoadFile(path)
	}
	return routes.Load(bytes.NewReader(routes.DefaultV1JSON()))
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

func newCORSOriginMatcher(allowedOrigins []string) func(*http.Request, string) bool {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		if normalized, ok := normalizeOrigin(origin); ok {
			allowed[normalized] = struct{}{}
		}
	}

	return func(_ *http.Request, origin string) bool {
		normalized, ok := normalizeOrigin(origin)
		if !ok {
			return false
		}
		_, exists := allowed[normalized]
		return exists
	}
}

func normalizeOrigin(origin string) (string, bool) {
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return "", false
	}

	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return "", false
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", false
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", false
	}
	if path := strings.Trim(u.EscapedPath(), "/"); path != "" {
		return "", false
	}

	host := strings.ToLower(u.Hostname())
	if host == "" {
		return "", false
	}

	port := u.Port()
	switch {
	case u.Scheme == "https" && port == "443":
		port = ""
	case u.Scheme == "http" && port == "80":
		port = ""
	}

	if port != "" {
		return u.Scheme + "://" + host + ":" + port, true
	}
	return u.Scheme + "://" + host, true
}
