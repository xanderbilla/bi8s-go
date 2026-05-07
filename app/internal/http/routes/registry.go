package routes

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/xanderbilla/bi8s-go/internal/errs"
)

func Mount(r chi.Router, cfg Config, handlers HandlerRegistry, mws MiddlewareRegistry) error {
	for _, route := range cfg.Routes {
		method := strings.ToUpper(route.Method)

		if route.Enabled != nil && !*route.Enabled {
			r.MethodFunc(method, route.Route, disabledResponder(route))
			continue
		}

		handler, ok := handlers[route.Handler]
		if !ok {
			return fmt.Errorf("routes: mount %s: unknown handler %q", route.ID, route.Handler)
		}

		chain := resolveMiddlewares(route, cfg.Defaults, mws)
		final := http.Handler(handler)

		for i := len(chain) - 1; i >= 0; i-- {
			final = chain[i](final)
		}
		r.MethodFunc(method, route.Route, final.ServeHTTP)
	}
	return nil
}

func resolveMiddlewares(route Route, defs Defaults, mws MiddlewareRegistry) []func(http.Handler) http.Handler {
	var names []string

	hasAppend := false
	for _, n := range route.Middlewares {
		if strings.HasPrefix(n, "+") {
			hasAppend = true
			break
		}
	}

	switch {
	case len(route.Middlewares) == 0:
		names = append(names, defs.Middlewares...)
	case hasAppend:
		names = append(names, defs.Middlewares...)
		for _, n := range route.Middlewares {
			names = append(names, strings.TrimPrefix(n, "+"))
		}
	default:
		names = append(names, route.Middlewares...)
	}

	out := make([]func(http.Handler) http.Handler, 0, len(names))
	for _, name := range names {
		ctor, ok := mws[name]
		if !ok {

			continue
		}
		if mw := ctor(route); mw != nil {
			out = append(out, mw)
		}
	}
	return out
}

func disabledResponder(route Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch route.DisabledResponse {
		case DisabledForbidden:
			errs.Write(w, r, &errs.APIError{
				Status:  http.StatusForbidden,
				Code:    "ROUTE_DISABLED",
				Message: "This endpoint is currently disabled",
			})
		default:
			errs.Write(w, r, errs.NewNotFound(""))
		}
	}
}
