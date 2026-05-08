package observability

import (
	"strconv"
	"strings"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/env"
)

type Config struct {
	Enabled        bool
	ServiceName    string
	ServiceVersion string
	Environment    string

	Endpoint string
	Insecure bool

	TracesEnabled  bool
	MetricsEnabled bool

	TraceSampleRatio float64

	MetricExportInterval time.Duration
	ShutdownTimeout      time.Duration
}

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
