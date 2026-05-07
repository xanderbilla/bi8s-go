package env

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

// LoadDotEnv loads key/value pairs from a .env-style file.
// Existing process environment variables are never overwritten.
func LoadDotEnv(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer func() { _ = f.Close() }()

	s := bufio.NewScanner(f)
	lineNo := 0
	for s.Scan() {
		lineNo++
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, raw, ok := strings.Cut(line, "=")
		if !ok {
			return true, fmt.Errorf("%s:%d invalid dotenv line: missing '='", path, lineNo)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return true, fmt.Errorf("%s:%d invalid dotenv line: empty key", path, lineNo)
		}
		val, err := parseDotEnvValue(raw)
		if err != nil {
			return true, fmt.Errorf("%s:%d %w", path, lineNo, err)
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, val); err != nil {
			return true, err
		}
	}
	if err := s.Err(); err != nil {
		return true, err
	}
	return true, nil
}

func parseDotEnvValue(raw string) (string, error) {
	val := strings.TrimSpace(raw)
	if val == "" {
		return "", nil
	}

	if strings.HasPrefix(val, "\"") {
		if len(val) < 2 || !strings.HasSuffix(val, "\"") {
			return "", errors.New("invalid quoted value")
		}
		content := strings.TrimSuffix(strings.TrimPrefix(val, "\""), "\"")
		repl := strings.NewReplacer(`\\n`, "\n", `\\r`, "\r", `\\t`, "\t", `\\\"`, `\"`, `\\\\`, `\\`)
		return repl.Replace(content), nil
	}
	if strings.HasPrefix(val, "'") {
		if len(val) < 2 || !strings.HasSuffix(val, "'") {
			return "", errors.New("invalid quoted value")
		}
		return strings.TrimSuffix(strings.TrimPrefix(val, "'"), "'"), nil
	}

	if i := strings.Index(val, " #"); i >= 0 {
		val = val[:i]
	}
	return strings.TrimSpace(val), nil
}
