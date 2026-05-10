package ratelimit

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/response"
)

type Backend interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
	Close() error
}

type Factory interface {
	NewBackend(name string, burst, refillPerSec float64) Backend
}

type Options struct {
	LimitHeaderValue string

	DefaultRetryAfterSeconds int
}

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
