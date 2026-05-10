package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type FailMode int

const (
	FailOpen FailMode = iota
	FailClosed
)

func ParseFailMode(s string) FailMode {
	switch s {
	case "fail-closed", "closed", "deny":
		return FailClosed
	default:
		return FailOpen
	}
}

const luaTokenBucket = `
local key   = KEYS[1]
local burst = tonumber(ARGV[1])
local rate  = tonumber(ARGV[2])
local now   = tonumber(ARGV[3])
local ttl   = tonumber(ARGV[4])

local data = redis.call('HMGET', key, 'tokens', 'ts')
local tokens = tonumber(data[1])
local ts     = tonumber(data[2])
if tokens == nil then
  tokens = burst
  ts = now
end

local delta = math.max(0, now - ts)
tokens = math.min(burst, tokens + delta * rate)

local allowed = 0
local retry_ms = 0
if tokens >= 1 then
  tokens = tokens - 1
  allowed = 1
else
  if rate > 0 then
    retry_ms = math.ceil(((1 - tokens) / rate) * 1000)
  else
    retry_ms = 60000
  end
end

redis.call('HMSET', key, 'tokens', tokens, 'ts', now)
redis.call('EXPIRE', key, ttl)

return {allowed, retry_ms}
`

var luaScript = goredis.NewScript(luaTokenBucket)

type RedisBackend struct {
	client       goredis.Scripter
	keyPrefix    string
	burst        float64
	refillPerSec float64
	timeout      time.Duration
	failMode     FailMode
	logger       *slog.Logger
}

type RedisBackendOptions struct {
	Client       goredis.Scripter
	Name         string
	Burst        float64
	RefillPerSec float64
	Timeout      time.Duration
	FailMode     FailMode
	Logger       *slog.Logger
}

func NewRedisBackend(opts RedisBackendOptions) (*RedisBackend, error) {
	if opts.Client == nil {
		return nil, errors.New("ratelimit: redis client is required")
	}
	if opts.Burst <= 0 || opts.RefillPerSec < 0 {
		return nil, errors.New("ratelimit: burst must be > 0 and refill >= 0")
	}
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 50 * time.Millisecond
	}
	logger := opts.Logger
	if logger == nil {
		logger = slog.Default()
	}
	prefix := "rl:" + opts.Name + ":"
	return &RedisBackend{
		client:       opts.Client,
		keyPrefix:    prefix,
		burst:        opts.Burst,
		refillPerSec: opts.RefillPerSec,
		timeout:      timeout,
		failMode:     opts.FailMode,
		logger:       logger,
	}, nil
}

func (r *RedisBackend) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	ctx, cancel := context.WithTimeout(ctx, r.timeout)
	defer cancel()

	now := float64(time.Now().UnixMilli()) / 1000.0
	ttl := int64(r.burst/maxRate(r.refillPerSec)) + 60

	res, err := luaScript.Run(ctx, r.client,
		[]string{r.keyPrefix + key},
		strconv.FormatFloat(r.burst, 'f', -1, 64),
		strconv.FormatFloat(r.refillPerSec, 'f', -1, 64),
		strconv.FormatFloat(now, 'f', -1, 64),
		strconv.FormatInt(ttl, 10),
	).Result()
	if err != nil {
		r.logger.Warn("ratelimit redis backend error",
			"err", err, "fail_mode", r.failMode, "key", key)
		if r.failMode == FailClosed {
			return false, time.Second, err
		}
		return true, 0, err
	}

	arr, ok := res.([]any)
	if !ok || len(arr) != 2 {
		return true, 0, fmt.Errorf("ratelimit: unexpected redis reply %T", res)
	}
	allowed, _ := arr[0].(int64)
	retryMS, _ := arr[1].(int64)
	return allowed == 1, time.Duration(retryMS) * time.Millisecond, nil
}

func (r *RedisBackend) Close() error { return nil }

func maxRate(r float64) float64 {
	if r > 0 {
		return r
	}
	return 1
}

type RedisFactory struct {
	Client   *goredis.Client
	Timeout  time.Duration
	FailMode FailMode
	Logger   *slog.Logger
}

func (f RedisFactory) NewBackend(name string, burst, refillPerSec float64) Backend {
	b, err := NewRedisBackend(RedisBackendOptions{
		Client:       f.Client,
		Name:         name,
		Burst:        burst,
		RefillPerSec: refillPerSec,
		Timeout:      f.Timeout,
		FailMode:     f.FailMode,
		Logger:       f.Logger,
	})
	if err != nil {

		slog.Error("ratelimit: redis factory misconfigured; falling back to memory backend",
			"name", name, "err", err)
		return NewMemoryBackend(burst, refillPerSec)
	}
	return b
}
