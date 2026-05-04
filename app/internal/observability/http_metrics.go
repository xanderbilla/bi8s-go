package observability

import (
	"net/http"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/go-chi/chi/v5"
)

const meterName = "github.com/xanderbilla/bi8s-go/internal/observability"

// HTTPMetrics holds the OTel instruments emitted by the HTTP middleware.
// Names follow OpenTelemetry semantic conventions for HTTP server metrics.
type HTTPMetrics struct {
	requests   metric.Int64Counter
	duration   metric.Float64Histogram
	inFlight   metric.Int64UpDownCounter
	respBytes  metric.Int64Counter
	inFlightMu atomic.Int64
}

// NewHTTPMetrics constructs the HTTP server instruments. It returns an error
// only if instrument creation fails (it never does for the in-memory meter
// when OTel is disabled, in which case the global noop meter is used).
func NewHTTPMetrics() (*HTTPMetrics, error) {
	meter := otel.Meter(meterName)

	requests, err := meter.Int64Counter("http.server.requests",
		metric.WithDescription("Total number of HTTP requests received"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}
	duration, err := meter.Float64Histogram("http.server.duration",
		metric.WithDescription("Duration of HTTP server requests"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}
	inFlight, err := meter.Int64UpDownCounter("http.server.active_requests",
		metric.WithDescription("Number of in-flight HTTP requests"),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return nil, err
	}
	respBytes, err := meter.Int64Counter("http.server.response.body.size",
		metric.WithDescription("Bytes written in HTTP responses"),
		metric.WithUnit("By"),
	)
	if err != nil {
		return nil, err
	}
	return &HTTPMetrics{
		requests:  requests,
		duration:  duration,
		inFlight:  inFlight,
		respBytes: respBytes,
	}, nil
}

// Middleware records request count, duration, in-flight gauge and response
// size for every HTTP request. It is meant to wrap the chi router AFTER the
// otelhttp tracing handler so the active span context is already available.
func (m *HTTPMetrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := r.Context()

		m.inFlightMu.Add(1)
		m.inFlight.Add(ctx, 1)
		defer func() {
			m.inFlightMu.Add(-1)
			m.inFlight.Add(ctx, -1)
		}()

		rec := &statusBytesRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}

		route := routeTemplate(r)
		attrs := metric.WithAttributes(
			attribute.String("http.request.method", r.Method),
			attribute.Int("http.response.status_code", status),
			attribute.String("http.route", route),
		)

		m.requests.Add(ctx, 1, attrs)
		m.duration.Record(ctx, float64(time.Since(start).Milliseconds()), attrs)
		if rec.bytes > 0 {
			m.respBytes.Add(ctx, int64(rec.bytes), attrs)
		}
	})
}

// routeTemplate returns the matched chi route pattern (e.g. "/v1/c/content/{contentId}")
// or falls back to "unmatched" so cardinality stays bounded for unknown URLs.
func routeTemplate(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if pattern := rctx.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return "unmatched"
}

type statusBytesRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (s *statusBytesRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (s *statusBytesRecorder) Write(b []byte) (int, error) {
	if s.status == 0 {
		s.status = http.StatusOK
	}
	n, err := s.ResponseWriter.Write(b)
	s.bytes += n
	return n, err
}
