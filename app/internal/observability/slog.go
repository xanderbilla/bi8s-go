package observability

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// SlogHandler wraps an slog.Handler and adds trace_id/span_id attributes drawn
// from the active span context whenever a record is emitted. It is the only
// integration point between the OTel trace context and structured logs, so all
// log lines emitted via slog.*Context (or our internal/logger) are
// automatically correlated with traces.
type SlogHandler struct {
	inner slog.Handler
}

// NewSlogHandler returns a handler that delegates to inner and injects
// trace_id/span_id when the request context carries an active span.
func NewSlogHandler(inner slog.Handler) *SlogHandler {
	return &SlogHandler{inner: inner}
}

func (h *SlogHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *SlogHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.inner.Handle(ctx, r)
}

func (h *SlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SlogHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *SlogHandler) WithGroup(name string) slog.Handler {
	return &SlogHandler{inner: h.inner.WithGroup(name)}
}
