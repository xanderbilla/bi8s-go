package app

import (
	"log/slog"
	"os"

	"github.com/xanderbilla/bi8s-go/internal/env"
	"github.com/xanderbilla/bi8s-go/internal/observability"
)

func SetupLogger() {
	base := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     env.ParseLogLevel(env.GetString("LOG_LEVEL", "info")),
		AddSource: env.GetBool("LOG_ADD_SOURCE", false),
	})
	slog.SetDefault(slog.New(observability.NewSlogHandler(base)))
}
