package observability

import (
	"strconv"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/env"
)

// Config controls OpenTelemetry SDK initialization. All values are sourced
// from environment variables; nothing is hardcoded.
type Config struct {
	Enabled        bool
	ServiceName    string
	ServiceVersion string
	Environment    string

	Endpoint string // host:port for the OTLP gRPC collector.
	Insecure bool   // true → no TLS (collector on the same network).

	TracesEnabled  bool
	MetricsEnabled bool

	TraceSampleRatio float64

	MetricExportInterval time.Duration
	ShutdownTimeout      time.Duration
}

// LoadConfig reads OTel configuration from environment variables. Defaults are
// safe for local development against the observability stack defined in
// observability/docker-compose.local.yml.
func LoadConfig(serviceName, serviceVersion, environment string) Config {
	if v := strings.TrimSpace(env.GetString("OTEL_SERVICE_NAME", "")); v != "" {
		serviceName = v
	}
	return Config{
		Enabled:              env.GetBool("OTEL_ENABLED", true),
		ServiceName:          serviceName,
		ServiceVersion:       serviceVersion,
		Environment:          environment,
		Endpoint:             env.GetString("OTEL_EXPORTER_OTLP_ENDPOINT", "otel-collector:4317"),
		Insecure:             env.GetBool("OTEL_EXPORTER_OTLP_INSECURE", true),
		TracesEnabled:        env.GetBool("OTEL_TRACES_ENABLED", true),
		MetricsEnabled:       env.GetBool("OTEL_METRICS_ENABLED", true),
		TraceSampleRatio:     parseRatio(env.GetString("OTEL_TRACES_SAMPLER_ARG", "1.0"), 1.0),
		MetricExportInterval: time.Duration(env.GetInt("OTEL_METRIC_EXPORT_INTERVAL_SECONDS", 15)) * time.Second,
		ShutdownTimeout:      time.Duration(env.GetInt("OTEL_SHUTDOWN_TIMEOUT_SECONDS", 5)) * time.Second,
	}
}

func parseRatio(s string, fallback float64) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fallback
	}
	switch {
	case v < 0:
		return 0
	case v > 1:
		return 1
	}
	return v
}
