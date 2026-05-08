package observability

import (
	"net/http"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/go-chi/chi/v5"

	"github.com/xanderbilla/bi8s-go/internal/http/respwriter"
)

const meterName = "github.com/xanderbilla/bi8s-go/internal/observability"

type HTTPMetrics struct {
	requests   metric.Int64Counter
	duration   metric.Float64Histogram
	inFlight   metric.Int64UpDownCounter
	respBytes  metric.Int64Counter
	inFlightMu atomic.Int64
}

func NewHTTPMetrics() (*HTTPMetrics, error) {
	meter := otel.Meter(meterName)

	var firstErr error
	capture := func(err error) {
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	requests, err := meter.Int64Counter("http.server.requests",
		metric.WithDescription("Total number of HTTP requests received"),
		metric.WithUnit("{request}"),
	)
	capture(err)
	duration, err := meter.Float64Histogram("http.server.duration",
		metric.WithDescription("Duration of HTTP server requests"),
		metric.WithUnit("ms"),
	)
	capture(err)
	inFlight, err := meter.Int64UpDownCounter("http.server.active_requests",
		metric.WithDescription("Number of in-flight HTTP requests"),
		metric.WithUnit("{request}"),
	)
	capture(err)
	respBytes, err := meter.Int64Counter("http.server.response.body.size",
		metric.WithDescription("Bytes written in HTTP responses"),
		metric.WithUnit("By"),
	)
	capture(err)

	if firstErr != nil {
		return nil, firstErr
	}
	return &HTTPMetrics{
		requests:  requests,
		duration:  duration,
		inFlight:  inFlight,
		respBytes: respBytes,
	}, nil
}

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

		rec := respwriter.New(w)
		next.ServeHTTP(rec, r)

		status := rec.Status
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
		if rec.Bytes > 0 {
			m.respBytes.Add(ctx, int64(rec.Bytes), attrs)
		}
	})
}

func routeTemplate(r *http.Request) string {
	if rctx := chi.RouteContext(r.Context()); rctx != nil {
		if pattern := rctx.RoutePattern(); pattern != "" {
			return pattern
		}
	}
	return "unmatched"
}
