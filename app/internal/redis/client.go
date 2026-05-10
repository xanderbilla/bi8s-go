package redis

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Options struct {
	URL          string
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	PoolSize     int
}

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
