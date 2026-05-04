package env

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

func GetString(key, fallback string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		slog.Debug("env var not set, using default", "key", key)
		return fallback
	}
	return val
}

func GetInt(key string, fallback int) int {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	v, err := strconv.Atoi(val)
	if err != nil {
		slog.Warn("env var is not a valid int, using default", "key", key, "value", val, "default", fallback)
		return fallback
	}
	return v
}

func GetSecret(key string) string {
	return os.Getenv(key)
}

func GetBool(key string, fallback bool) bool {
	val, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	v, err := strconv.ParseBool(val)
	if err != nil {
		slog.Warn("env var is not a valid bool, using default", "key", key, "value", val, "default", fallback)
		return fallback
	}
	return v
}

func MustString(key string) (string, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return "", fmt.Errorf("required env var %q is not set", key)
	}
	return v, nil
}

func IntInRange(key string, fallback, min, max int) int {
	v := GetInt(key, fallback)
	if v < min {
		slog.Warn("env var below allowed range, clamping", "key", key, "value", v, "min", min)
		return min
	}
	if v > max {
		slog.Warn("env var above allowed range, clamping", "key", key, "value", v, "max", max)
		return max
	}
	return v
}

func ParseLogLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func ParseCommaSeparated(val string) []string {
	parts := strings.Split(val, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
