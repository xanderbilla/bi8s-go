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

// FailMode controls behaviour when the Redis backend cannot be reached.
//
//   - FailOpen   — allow the request (preserves availability under Redis
//     outage; the trade-off is that limits are not enforced during the
//     incident window).
//   - FailClosed — deny the request with retryAfter=1s (preserves rate-limit
//     guarantees at the cost of availability).
type FailMode int

const (
	FailOpen FailMode = iota
	FailClosed
)

// ParseFailMode is a small convenience for env-driven config.
func ParseFailMode(s string) FailMode {
	switch s {
	case "fail-closed", "closed", "deny":
		return FailClosed
	default:
		return FailOpen
	}
}

// luaTokenBucket is an atomic token-bucket implementation.
//
// KEYS[1] = bucket key
// ARGV[1] = burst (max tokens, float)
// ARGV[2] = refill rate (tokens per second, float)
// ARGV[3] = now (unix seconds, float)
// ARGV[4] = ttl seconds (int) — used to expire idle buckets.
//
// The bucket is stored as a Redis hash {tokens, ts}. On each call we
// refill based on elapsed time, debit one token if available, persist the
// new state with a TTL, and return [allowed, retryAfter_ms].
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

// RedisBackend implements Backend on top of an atomic Lua token-bucket
// stored in Redis. Safe for many replicas to share a single bucket per key.
type RedisBackend struct {
	client       goredis.Scripter
	keyPrefix    string
	burst        float64
	refillPerSec float64
	timeout      time.Duration
	failMode     FailMode
	logger       *slog.Logger
}

// RedisBackendOptions configures a RedisBackend.
type RedisBackendOptions struct {
	Client       goredis.Scripter // required; *goredis.Client satisfies this.
	Name         string           // bucket family name; included in keys.
	Burst        float64          // max tokens.
	RefillPerSec float64          // steady-state refill rate.
	Timeout      time.Duration    // per-call deadline; default 50ms.
	FailMode     FailMode         // behaviour on Redis errors.
	Logger       *slog.Logger     // optional; slog.Default() if nil.
}

// NewRedisBackend constructs a Backend keyed by Name. Each request's key is
// concatenated with KeyPrefix to namespace buckets across families.
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

// Allow runs the Lua token-bucket atomically. On Redis error the backend
// honours its configured FailMode.
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

// Close is a no-op; the underlying *redis.Client lifecycle is owned by the
// caller (so it can be shared across many backends).
func (r *RedisBackend) Close() error { return nil }

// maxRate avoids divide-by-zero when computing bucket TTL.
func maxRate(r float64) float64 {
	if r > 0 {
		return r
	}
	return 1
}

// RedisFactory builds RedisBackends sharing a single client connection.
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
		// Configuration error — fall back to memory so the server still
		// boots; loud log so the operator notices.
		slog.Error("ratelimit: redis factory misconfigured; falling back to memory backend",
			"name", name, "err", err)
		return NewMemoryBackend(burst, refillPerSec)
	}
	return b
}
