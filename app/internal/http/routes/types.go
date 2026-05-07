package routes

import (
	"encoding/json"
	"net/http"
)

var supportedMethods = map[string]struct{}{
	http.MethodGet:     {},
	http.MethodPost:    {},
	http.MethodPut:     {},
	http.MethodPatch:   {},
	http.MethodDelete:  {},
	http.MethodHead:    {},
	http.MethodOptions: {},
}

type RateLimit struct {
	Enabled       *bool `json:"enabled,omitempty"`
	Requests      int   `json:"requests,omitempty"`
	WindowSeconds int   `json:"windowSeconds,omitempty"`
}

type Cache struct {
	Enabled    *bool `json:"enabled,omitempty"`
	TTLSeconds int   `json:"ttlSeconds,omitempty"`
}

type DisabledResponse string

const (
	DisabledNotFound  DisabledResponse = "notfound"
	DisabledForbidden DisabledResponse = "forbidden"
)

type Route struct {
	ID               string           `json:"id"`
	Route            string           `json:"route"`
	Method           string           `json:"method"`
	Enabled          *bool            `json:"enabled,omitempty"`
	DisabledResponse DisabledResponse `json:"disabledResponse,omitempty"`

	Auth         bool        `json:"auth,omitempty"`
	Caching      bool        `json:"caching,omitempty"`
	Cache        *Cache      `json:"cache,omitempty"`
	CacheTTLSec  int         `json:"cacheTTLSeconds,omitempty"`
	AllowedIPs   []string    `json:"allowedIPs,omitempty"`
	RateLimit    *RateLimit  `json:"rateLimit,omitempty"`
	Middlewares  []string    `json:"middlewares,omitempty"`
	Handler      string      `json:"handler"`
	HandlerExtra interface{} `json:"handlerExtra,omitempty"`
}

type Defaults struct {
	Middlewares []string   `json:"middlewares,omitempty"`
	RateLimit   *RateLimit `json:"rateLimit,omitempty"`
	Cache       *Cache     `json:"cache,omitempty"`
	AllowedIPs  []string   `json:"allowedIPs,omitempty"`
}

type Config struct {
	Version  int      `json:"version"`
	Defaults Defaults `json:"defaults"`
	Routes   []Route  `json:"routes"`
}

type HandlerRegistry map[string]http.HandlerFunc

type MiddlewareRegistry map[string]MiddlewareConstructor

type MiddlewareConstructor func(r Route) func(http.Handler) http.Handler

func (c Config) MarshalIndent() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}
