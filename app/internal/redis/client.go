// Package redis wraps go-redis with project-wide defaults: URL parsing,
// startup PING, sane timeouts, and a small surface that the rest of the
// codebase can depend on without pulling go-redis types around.
package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

// Options carries the small set of connection knobs we expose. Anything
// not listed is taken from the URL or go-redis defaults.
type Options struct {
	URL          string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
}

// New parses url, builds a *goredis.Client, and verifies connectivity with
// a PING using the supplied context. The caller owns the returned client
// and is responsible for calling Close on shutdown.
func New(ctx context.Context, opts Options) (*goredis.Client, error) {
	if strings.TrimSpace(opts.URL) == "" {
		return nil, errors.New("redis: URL is required")
	}
	o, err := goredis.ParseURL(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("redis: parse URL: %w", err)
	}
	if opts.DialTimeout > 0 {
		o.DialTimeout = opts.DialTimeout
	}
	if opts.ReadTimeout > 0 {
		o.ReadTimeout = opts.ReadTimeout
	}
	if opts.WriteTimeout > 0 {
		o.WriteTimeout = opts.WriteTimeout
	}
	if opts.PoolSize > 0 {
		o.PoolSize = opts.PoolSize
	}

	client := goredis.NewClient(o)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("redis: ping failed: %w", err)
	}
	return client, nil
}
