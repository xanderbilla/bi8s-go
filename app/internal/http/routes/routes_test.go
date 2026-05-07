package routes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

func okHandler(body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}
}

func passMW(_ Route) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler { return next }
}

func TestLoadAndValidate_Valid(t *testing.T) {
	src := `{
	  "version": 1,
	  "defaults": {"middlewares": ["auth"]},
	  "routes": [
	    {"id": "r1", "method": "GET", "route": "/x", "handler": "h.x", "middlewares": ["+cache"]}
	  ]
	}`
	cfg, err := Load(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	handlers := HandlerRegistry{"h.x": okHandler("ok")}
	mws := MiddlewareRegistry{"auth": passMW, "cache": passMW}
	if err := Validate(cfg, handlers, mws); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

func TestValidate_FailsOnUnknownHandler(t *testing.T) {
	cfg := Config{
		Version: 1,
		Routes: []Route{
			{ID: "r1", Method: "GET", Route: "/x", Handler: "missing"},
		},
	}
	err := Validate(cfg, HandlerRegistry{}, MiddlewareRegistry{})
	if err == nil || !strings.Contains(err.Error(), "handler \"missing\" is not registered") {
		t.Fatalf("expected unknown-handler error, got %v", err)
	}
}

func TestValidate_FailsOnDuplicateRoute(t *testing.T) {
	cfg := Config{
		Version: 1,
		Routes: []Route{
			{ID: "a", Method: "GET", Route: "/x", Handler: "h"},
			{ID: "b", Method: "GET", Route: "/x", Handler: "h"},
		},
	}
	err := Validate(cfg, HandlerRegistry{"h": okHandler("")}, MiddlewareRegistry{})
	if err == nil || !strings.Contains(err.Error(), "duplicate (method, route)") {
		t.Fatalf("expected duplicate route error, got %v", err)
	}
}

func TestValidate_FailsOnBadCIDR(t *testing.T) {
	cfg := Config{
		Version: 1,
		Routes: []Route{
			{ID: "a", Method: "GET", Route: "/x", Handler: "h", AllowedIPs: []string{"not-an-ip"}},
		},
	}
	err := Validate(cfg, HandlerRegistry{"h": okHandler("")}, MiddlewareRegistry{})
	if err == nil || !strings.Contains(err.Error(), "invalid IP") {
		t.Fatalf("expected CIDR error, got %v", err)
	}
}

func TestMount_AppliesRouteAndDisabledRoute(t *testing.T) {
	enabled := true
	disabled := false
	cfg := Config{
		Version: 1,
		Routes: []Route{
			{ID: "live", Method: "GET", Route: "/live", Handler: "h.live", Enabled: &enabled},
			{ID: "off", Method: "GET", Route: "/off", Handler: "h.live", Enabled: &disabled},
		},
	}
	handlers := HandlerRegistry{"h.live": okHandler("alive")}
	mws := MiddlewareRegistry{}
	if err := Validate(cfg, handlers, mws); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	r := chi.NewRouter()
	if err := Mount(r, cfg, handlers, mws); err != nil {
		t.Fatalf("Mount: %v", err)
	}

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/live", nil))
	if rec.Code != 200 || rec.Body.String() != "alive" {
		t.Fatalf("live route: got %d %q", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/off", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("disabled route: expected 404, got %d", rec.Code)
	}
}
