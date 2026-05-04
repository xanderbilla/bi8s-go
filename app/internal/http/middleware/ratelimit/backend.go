package ratelimit

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/response"
)

// Backend computes whether a single request bound to key is allowed under
// the configured token-bucket parameters. Allow MUST be safe for concurrent
// use across many requests and many keys. Implementations may consult
// shared state (e.g. Redis) but should treat ctx as a hard deadline.
//
// retryAfter is a hint emitted to clients via the Retry-After header when
// allowed is false; implementations may return zero to fall back to the
// middleware default.
type Backend interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
	Close() error
}

// Factory builds a Backend bound to a named bucket configuration. Each
// route family (global, encoder write, …) calls NewBackend exactly once
// during router construction; the returned Backend is reused for the life
// of the process and Close()d on shutdown.
type Factory interface {
	// NewBackend returns a backend keyed by name with the given burst
	// (max tokens) and steady-state refill rate (tokens per second).
	NewBackend(name string, burst, refillPerSec float64) Backend
}

// Options drive Middleware behaviour. Most callers can leave them at the
// zero value.
type Options struct {
	// LimitHeaderValue is the verbatim string sent in X-RateLimit-Limit
	// when a request is denied. Empty means derive from burst.
	LimitHeaderValue string
	// DefaultRetryAfterSeconds is used when the backend reports zero
	// retryAfter. Must be > 0.
	DefaultRetryAfterSeconds int
}

// Middleware returns an HTTP middleware that consults backend on every
// request keyed by the trusted-proxy-aware client IP. Burst and refillPerSec
// are used only to render headers; the backend itself owns the policy.
func Middleware(backend Backend, burst, refillPerSec float64, opts Options) func(http.Handler) http.Handler {
	limitHeader := opts.LimitHeaderValue
	if limitHeader == "" {
		limitHeader = strconv.FormatFloat(burst, 'f', -1, 64)
	}
	defaultRetry := opts.DefaultRetryAfterSeconds
	if defaultRetry <= 0 {
		if refillPerSec > 0 {
			defaultRetry = int(1.0/refillPerSec) + 1
		} else {
			defaultRetry = 60
		}
	}
	defaultRetryStr := strconv.Itoa(defaultRetry)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := GetClientIP(r)
			allowed, retryAfter, err := backend.Allow(r.Context(), key)
			// Backend errors are surfaced via the backend's own decision
			// (fail-open or fail-closed reflected in `allowed`); the err
			// value is intentionally ignored here.
			_ = err
			if !allowed {
				w.Header().Set("X-RateLimit-Limit", limitHeader)
				w.Header().Set("X-RateLimit-Remaining", "0")
				if retryAfter > 0 {
					w.Header().Set("Retry-After", strconv.Itoa(int(retryAfter.Seconds())+1))
				} else {
					w.Header().Set("Retry-After", defaultRetryStr)
				}
				_ = response.Error(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Rate limit exceeded. Please try again later.", nil)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
