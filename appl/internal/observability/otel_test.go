package observability

import (
	"context"
	"testing"
	"time"
)

func TestInit_Disabled_ReturnsNoopProvider(t *testing.T) {
	cfg := Config{Enabled: false}
	p, err := Init(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Init(disabled) returned error: %v", err)
	}
	if p == nil {
		t.Fatal("Init(disabled) returned nil provider")
	}
	if len(p.shutdown) != 0 {
		t.Fatalf("expected zero shutdown hooks for disabled provider, got %d", len(p.shutdown))
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown returned error: %v", err)
	}
}

func TestShutdown_Idempotent(t *testing.T) {
	p := &Provider{}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("first Shutdown: %v", err)
	}
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("second Shutdown: %v", err)
	}
}

func TestShutdown_NilProvider(t *testing.T) {
	var p *Provider
	if err := p.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown on nil provider: %v", err)
	}
}

func TestInit_EnabledUnreachableEndpoint_DoesNotBlock(t *testing.T) {
	// gRPC dial is non-blocking by default, so Init must succeed even if the
	// collector is unreachable. Spans/metrics will simply fail to export.
	cfg := Config{
		Enabled:              true,
		ServiceName:          "test-service",
		ServiceVersion:       "0.0.0",
		Environment:          "test",
		Endpoint:             "127.0.0.1:1", // intentionally unreachable
		Insecure:             true,
		TracesEnabled:        true,
		MetricsEnabled:       true,
		TraceSampleRatio:     1.0,
		MetricExportInterval: time.Second,
		ShutdownTimeout:      time.Second,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	p, err := Init(ctx, cfg)
	if err != nil {
		t.Fatalf("Init returned error for unreachable endpoint: %v", err)
	}
	t.Cleanup(func() {
		shutCtx, shutCancel := context.WithTimeout(context.Background(), time.Second)
		defer shutCancel()
		_ = p.Shutdown(shutCtx)
	})
	if p.tp == nil {
		t.Error("expected tracer provider when traces enabled")
	}
	if p.mp == nil {
		t.Error("expected meter provider when metrics enabled")
	}
}
