package observability

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// Provider holds the live OTel SDK providers and exposes a single Shutdown
// to flush traces/metrics on graceful termination.
type Provider struct {
	cfg      Config
	tp       *sdktrace.TracerProvider
	mp       *metric.MeterProvider
	shutdown []func(context.Context) error
	closeMu  sync.Mutex
	closed   bool
}

// Init configures global tracer/meter/propagator. When cfg.Enabled is false it
// installs no-op providers and returns a Provider whose Shutdown is a noop.
// All exporters use OTLP/gRPC. Telemetry MUST flow only to the OTel collector.
func Init(ctx context.Context, cfg Config) (*Provider, error) {
	p := &Provider{cfg: cfg}

	if !cfg.Enabled {
		return p, nil
	}

	res, err := buildResource(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("otel: build resource: %w", err)
	}

	dialOpts := []grpc.DialOption{}
	if cfg.Insecure {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	} else {
		dialOpts = append(dialOpts, grpc.WithTransportCredentials(credentials.NewTLS(nil)))
	}

	if cfg.TracesEnabled {
		traceExp, err := otlptrace.New(ctx, otlptracegrpc.NewClient(
			otlptracegrpc.WithEndpoint(cfg.Endpoint),
			otlptracegrpc.WithDialOption(dialOpts...),
		))
		if err != nil {
			return nil, fmt.Errorf("otel: trace exporter: %w", err)
		}
		p.tp = sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(traceExp),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.TraceSampleRatio))),
		)
		otel.SetTracerProvider(p.tp)
		p.shutdown = append(p.shutdown, p.tp.Shutdown)
	}

	if cfg.MetricsEnabled {
		metricExp, err := otlpmetricgrpc.New(ctx,
			otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
			otlpmetricgrpc.WithDialOption(dialOpts...),
		)
		if err != nil {
			return nil, fmt.Errorf("otel: metric exporter: %w", err)
		}
		p.mp = metric.NewMeterProvider(
			metric.WithReader(metric.NewPeriodicReader(metricExp,
				metric.WithInterval(cfg.MetricExportInterval),
			)),
			metric.WithResource(res),
		)
		otel.SetMeterProvider(p.mp)
		p.shutdown = append(p.shutdown, p.mp.Shutdown)
	}

	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return p, nil
}

func buildResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	return resource.New(ctx,
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithOS(),
		resource.WithContainer(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceName(cfg.ServiceName),
			semconv.ServiceVersion(cfg.ServiceVersion),
			semconv.DeploymentEnvironment(cfg.Environment),
		),
	)
}

// Shutdown flushes pending telemetry and closes exporters. Safe to call more
// than once; subsequent calls are noops.
func (p *Provider) Shutdown(ctx context.Context) error {
	if p == nil {
		return nil
	}
	p.closeMu.Lock()
	defer p.closeMu.Unlock()
	if p.closed {
		return nil
	}
	p.closed = true

	var errs []error
	for _, fn := range p.shutdown {
		if err := fn(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
