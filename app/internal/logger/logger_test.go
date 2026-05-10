package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/xanderbilla/bi8s-go/internal/ctxutil"
)

func captureJSON(t *testing.T, level slog.Level, fn func()) map[string]any {
	t.Helper()
	prev := slog.Default()
	defer slog.SetDefault(prev)

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: level})))
	fn()

	out := strings.TrimSpace(buf.String())
	if out == "" {
		return nil
	}
	if i := strings.LastIndex(out, "\n"); i >= 0 {
		out = out[i+1:]
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("invalid json: %v: %s", err, out)
	}
	return m
}

func TestLoggerInjectsRequestID(t *testing.T) {
	ctx := ctxutil.WithRequestID(context.Background(), "req-xyz")
	m := captureJSON(t, slog.LevelInfo, func() {
		InfoContext(ctx, "hello", "k", "v")
	})
	if got, want := m["request_id"], "req-xyz"; got != want {
		t.Fatalf("request_id = %v, want %v", got, want)
	}
	if got, want := m["k"], "v"; got != want {
		t.Fatalf("custom field k = %v, want %v", got, want)
	}
	if got, want := m["msg"], "hello"; got != want {
		t.Fatalf("msg = %v, want %v", got, want)
	}
}

func TestLoggerOmitsRequestIDWhenAbsent(t *testing.T) {
	m := captureJSON(t, slog.LevelInfo, func() {
		InfoContext(context.Background(), "no-req")
	})
	if _, ok := m["request_id"]; ok {
		t.Fatal("request_id should be omitted when not in context")
	}
}

func TestLoggerLevels(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		name  string
		fn    func()
		level string
	}{
		{"debug", func() { DebugContext(ctx, "d") }, "DEBUG"},
		{"info", func() { InfoContext(ctx, "i") }, "INFO"},
		{"warn", func() { WarnContext(ctx, "w") }, "WARN"},
		{"error", func() { ErrorContext(ctx, "e") }, "ERROR"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := captureJSON(t, slog.LevelDebug, tc.fn)
			if got := m["level"]; got != tc.level {
				t.Fatalf("level = %v, want %v", got, tc.level)
			}
		})
	}
}
