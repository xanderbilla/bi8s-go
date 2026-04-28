package http

import (
	"expvar"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

var (
	httpRequestsTotal     = expvar.NewMap("http_requests_total")
	httpRequestsByStatus  = expvar.NewMap("http_requests_by_status")
	httpInFlight          = expvar.NewInt("http_in_flight")
	httpRequestDurationMs = expvar.NewMap("http_request_duration_ms_total")
)

var inFlightGauge atomic.Int64

func MetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		httpInFlight.Set(inFlightGauge.Add(1))
		defer func() {
			httpInFlight.Set(inFlightGauge.Add(-1))
		}()

		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		class := statusClass(status)
		dur := time.Since(start).Milliseconds()

		httpRequestsTotal.Add("all", 1)
		httpRequestsTotal.Add(class, 1)
		httpRequestsByStatus.Add(strconv.Itoa(status), 1)
		httpRequestDurationMs.Add("all", dur)
		httpRequestDurationMs.Add(class, dur)
	})
}

func statusClass(code int) string {
	switch {
	case code >= 500:
		return "5xx"
	case code >= 400:
		return "4xx"
	case code >= 300:
		return "3xx"
	case code >= 200:
		return "2xx"
	default:
		return "1xx"
	}
}
