package http

import (
	"context"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/errs"
	"github.com/xanderbilla/bi8s-go/internal/logger"
	"github.com/xanderbilla/bi8s-go/internal/response"
)

var readyFlag atomic.Bool

func SetReady(ready bool) { readyFlag.Store(ready) }

func IsReady() bool { return readyFlag.Load() }

type HealthHandler struct {
	env          string
	healthChecks map[string]app.HealthCheck
}

func (h *HealthHandler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	type result struct {
		name string
		ok   bool
	}

	results := make(chan result, len(h.healthChecks))
	var wg sync.WaitGroup
	for name, check := range h.healthChecks {
		wg.Add(1)
		go func(name string, check app.HealthCheck) {
			defer wg.Done()
			results <- result{name: name, ok: check(ctx) == nil}
		}(name, check)
	}
	wg.Wait()
	close(results)

	checks := make(map[string]string, len(h.healthChecks))
	allOK := true
	for r := range results {
		if r.ok {
			checks[r.name] = "up"
			continue
		}
		checks[r.name] = "down"
		allOK = false
	}

	if !allOK {
		errs.Write(w, r, &errs.APIError{
			Status:  http.StatusServiceUnavailable,
			Code:    "SERVICE_UNAVAILABLE",
			Message: "one or more dependencies are unhealthy",
			Details: map[string]any{"checks": checks},
		})
		return
	}

	if err := response.Success(w, r, http.StatusOK, "health check passed", map[string]any{
		"env":    h.env,
		"checks": checks,
	}); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	if err := response.Success(w, r, http.StatusOK, "alive", nil); err != nil {
		logger.ErrorContext(r.Context(), "failed to write response", "error", err)
	}
}

func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	if !readyFlag.Load() {
		errs.Write(w, r, &errs.APIError{
			Status:  http.StatusServiceUnavailable,
			Code:    "NOT_READY",
			Message: "server is not ready to accept traffic",
		})
		return
	}
	h.HealthCheck(w, r)
}
