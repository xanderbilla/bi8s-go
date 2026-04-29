package logger

import (
	"context"
	"log/slog"

	"github.com/xanderbilla/bi8s-go/internal/ctxutil"
)

func InfoContext(ctx context.Context, msg string, args ...any) {
	args = appendRequestID(ctx, args)
	slog.InfoContext(ctx, msg, args...)
}

func WarnContext(ctx context.Context, msg string, args ...any) {
	args = appendRequestID(ctx, args)
	slog.WarnContext(ctx, msg, args...)
}

func ErrorContext(ctx context.Context, msg string, args ...any) {
	args = appendRequestID(ctx, args)
	slog.ErrorContext(ctx, msg, args...)
}

func DebugContext(ctx context.Context, msg string, args ...any) {
	args = appendRequestID(ctx, args)
	slog.DebugContext(ctx, msg, args...)
}

func appendRequestID(ctx context.Context, args []any) []any {
	if reqID := ctxutil.GetRequestID(ctx); reqID != "" {
		return append([]any{"request_id", reqID}, args...)
	}
	return args
}
