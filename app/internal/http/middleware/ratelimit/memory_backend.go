package ratelimit

import (
	"context"
	"time"
)

type MemoryBackend struct {
	rl *RateLimiter
}

func NewMemoryBackend(burst, refillPerSec float64) *MemoryBackend {
	return &MemoryBackend{rl: NewRateLimiter(burst, refillPerSec)}
}

func (m *MemoryBackend) Allow(_ context.Context, key string) (bool, time.Duration, error) {
	return m.rl.GetLimiter(key).Allow(), 0, nil
}

func (m *MemoryBackend) Close() error {
	m.rl.Close()
	return nil
}

type MemoryFactory struct{}

func (MemoryFactory) NewBackend(_ string, burst, refillPerSec float64) Backend {
	return NewMemoryBackend(burst, refillPerSec)
}
