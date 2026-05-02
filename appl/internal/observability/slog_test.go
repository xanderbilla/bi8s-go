package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

func TestSlogHandler_InjectsTraceAndSpanID(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := NewSlogHandler(inner)
	logger := slog.New(h)

	traceID, _ := trace.TraceIDFromHex("0102030405060708090a0b0c0d0e0f10")
	spanID, _ := trace.SpanIDFromHex("0102030405060708")
	sc := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     false,
	})
	if !sc.IsValid() {
		t.Fatal("constructed span context is not valid")
	}
	ctx := trace.ContextWithSpanContext(context.Background(), sc)

	logger.InfoContext(ctx, "hello")

	var got map[string]any
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &got); err != nil {
		t.Fatalf("unmarshal log: %v\noutput: %s", err, buf.String())
	}
	if got["trace_id"] != traceID.String() {
		t.Errorf("trace_id = %v, want %s", got["trace_id"], traceID.String())
	}
	if got["span_id"] != spanID.String() {
		t.Errorf("span_id = %v, want %s", got["span_id"], spanID.String())
	}
}

func TestSlogHandler_NoSpanContext_OmitsAttrs(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	logger := slog.New(NewSlogHandler(inner))

	logger.InfoContext(context.Background(), "hello")

	out := buf.String()
	if strings.Contains(out, "trace_id") {
		t.Errorf("did not expect trace_id in output without span context: %s", out)
	}
	if strings.Contains(out, "span_id") {
		t.Errorf("did not expect span_id in output without span context: %s", out)
	}
}

func TestSlogHandler_WithAttrsAndGroupDelegate(t *testing.T) {
	var buf bytes.Buffer
	inner := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	h := NewSlogHandler(inner)

	withAttrs := h.WithAttrs([]slog.Attr{slog.String("svc", "bi8s")})
	if _, ok := withAttrs.(*SlogHandler); !ok {
		t.Fatalf("WithAttrs returned %T, want *SlogHandler", withAttrs)
	}

	withGroup := h.WithGroup("grp")
	if _, ok := withGroup.(*SlogHandler); !ok {
		t.Fatalf("WithGroup returned %T, want *SlogHandler", withGroup)
	}
}
