package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/app"
	"github.com/xanderbilla/bi8s-go/internal/env"
	httpkg "github.com/xanderbilla/bi8s-go/internal/http"
	"github.com/xanderbilla/bi8s-go/internal/observability"
)

func main() {
	app.SetupLogger()

	if err := run(); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	cfg := app.LoadConfigFromEnv()
	if err := cfg.Validate(); err != nil {
		return err
	}

	if err := app.ConfigureTrustedProxies(cfg); err != nil {
		return err
	}

	app.ConfigureRuntime()
	httpkg.ConfigureLimits(httpkg.Limits{
		MultipartBodyBytes: int64(env.GetInt("HTTP_MAX_MULTIPART_BODY_BYTES", 12_582_912)),
		MultipartFileBytes: int64(env.GetInt("HTTP_MAX_MULTIPART_FILE_BYTES", 10_485_760)),
		VideoBodyBytes:     int64(env.GetInt("HTTP_MAX_VIDEO_BODY_BYTES", 10_737_418_240)),
		VideoFileBytes:     int64(env.GetInt("HTTP_MAX_VIDEO_FILE_BYTES", 10_737_418_240)),
	})

	slog.Info("starting server",
		"addr", cfg.Addr,
		"env", cfg.Env,
		"movie_table", cfg.TableName,
		"person_table", cfg.PersonTableName,
		"attribute_table", cfg.AttributeTableName,
		"encoder_table", cfg.EncoderTableName,
		"cors_origins", cfg.CORSAllowedOrigins,
	)

	initCtx, cancel := context.WithTimeout(
		context.Background(),
		time.Duration(env.GetInt("INIT_TIMEOUT_SECONDS", 30))*time.Second,
	)
	defer cancel()

	// Initialize OpenTelemetry BEFORE building AWS clients so otelaws
	// middleware has live providers to record startup health-check traces.
	obsCfg := observability.LoadConfig(
		env.GetString("OTEL_SERVICE_NAME", "bi8s-api"),
		env.GetString("BUILD_VERSION", "dev"),
		cfg.Env,
	)
	obsProvider, err := observability.Init(initCtx, obsCfg)
	if err != nil {
		slog.Warn("observability init failed; continuing without telemetry", "error", err)
	}

	application, err := app.Build(initCtx, cfg)
	if err != nil {
		return errors.Join(err, shutdownObs(obsProvider, obsCfg.ShutdownTimeout))
	}

	httpMetrics, err := observability.NewHTTPMetrics()
	if err != nil {
		return errors.Join(err, shutdownObs(obsProvider, obsCfg.ShutdownTimeout))
	}
	application.HTTPMetrics = httpMetrics

	probeCtx, probeCancel := context.WithTimeout(
		context.Background(),
		time.Duration(env.GetInt("STARTUP_HEALTHCHECK_TIMEOUT_SECONDS", 10))*time.Second,
	)
	defer probeCancel()
	if err := app.RunStartupHealthChecks(probeCtx, application); err != nil {
		return errors.Join(err, shutdownObs(obsProvider, obsCfg.ShutdownTimeout))
	}

	serveErr := serve(application)
	if shutErr := shutdownObs(obsProvider, obsCfg.ShutdownTimeout); shutErr != nil {
		serveErr = errors.Join(serveErr, shutErr)
	}
	return serveErr
}

func shutdownObs(p *observability.Provider, timeout time.Duration) error {
	if p == nil {
		return nil
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return p.Shutdown(ctx)
}
