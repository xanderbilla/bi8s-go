package ratelimit

import (
	"context"
	"time"
)

// MemoryBackend wraps the per-instance *RateLimiter so it satisfies the
// Backend interface. State lives in-process only; multi-replica deployments
// MUST use the Redis backend instead.
type MemoryBackend struct {
	rl *RateLimiter
}

// NewMemoryBackend creates a Backend backed by an in-process token-bucket
// map keyed by client IP. burst is the bucket size, refillPerSec the
// steady-state refill rate.
func NewMemoryBackend(burst, refillPerSec float64) *MemoryBackend {
	return &MemoryBackend{rl: NewRateLimiter(burst, refillPerSec)}
}

// Allow consults the per-key bucket. err is always nil for the in-process
// backend.
func (m *MemoryBackend) Allow(_ context.Context, key string) (bool, time.Duration, error) {
	return m.rl.GetLimiter(key).Allow(), 0, nil
}

// Close stops the background eviction loop.
func (m *MemoryBackend) Close() error {
	m.rl.Close()
	return nil
}

// MemoryFactory builds MemoryBackends. It owns no shared state; each call
// to NewBackend returns a fresh, isolated bucket map.
type MemoryFactory struct{}

func (MemoryFactory) NewBackend(_ string, burst, refillPerSec float64) Backend {
	return NewMemoryBackend(burst, refillPerSec)
}
