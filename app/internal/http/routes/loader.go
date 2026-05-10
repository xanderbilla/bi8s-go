package routes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
)

func LoadFile(path string) (Config, error) {
	if path == "" {
		return Config{}, errors.New("routes: empty config path")
	}
	f, err := os.Open(path) //nolint:gosec // path comes from trusted env config
	if err != nil {
		return Config{}, fmt.Errorf("routes: open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()
	return Load(f)
}

func Load(r io.Reader) (Config, error) {
	var cfg Config
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		return Config{}, fmt.Errorf("routes: decode: %w", err)
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	return cfg, nil
}
