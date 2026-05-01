package app

import (
	"log/slog"
	"os"

	"github.com/xanderbilla/bi8s-go/internal/env"
)

func SetupLogger() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     env.ParseLogLevel(env.GetString("LOG_LEVEL", "info")),
		AddSource: env.GetBool("LOG_ADD_SOURCE", false),
	})))
}
