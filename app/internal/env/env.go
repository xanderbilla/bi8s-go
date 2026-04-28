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

// GetSecret returns the value of key without logging on miss. Use this for
// credentials and other sensitive values.
func GetSecret(key string) string {
	return os.Getenv(key)
}

// GetBool returns the parsed boolean value of key, or fallback if unset or
// unparseable. Accepts any value parseable by strconv.ParseBool
// (1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False).
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

// MustString returns the value of key or an error if unset/empty.
func MustString(key string) (string, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return "", fmt.Errorf("required env var %q is not set", key)
	}
	return v, nil
}

// IntInRange returns GetInt(key, fallback) clamped to [min,max] and warns when out of range.
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

// ParseLogLevel maps a case-insensitive log level string to slog.Level.
// Unknown or empty values default to slog.LevelInfo.
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

// ParseCommaSeparated splits a CSV string and returns trimmed, non-empty parts.
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
