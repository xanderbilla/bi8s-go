package ratelimit

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xanderbilla/bi8s-go/internal/response"
)

var trustedProxies atomic.Pointer[[]*net.IPNet]

func SetTrustedProxies(cidrs []string) error {
	parsed := make([]*net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}

		if !strings.Contains(c, "/") {
			ip := net.ParseIP(c)
			if ip == nil {
				return &net.ParseError{Type: "trusted proxy", Text: c}
			}
			if ip.To4() != nil {
				c += "/32"
			} else {
				c += "/128"
			}
		}
		_, network, err := net.ParseCIDR(c)
		if err != nil {
			return err
		}
		parsed = append(parsed, network)
	}
	trustedProxies.Store(&parsed)
	return nil
}

func isTrustedProxy(ip net.IP) bool {
	list := trustedProxies.Load()
	if list == nil || len(*list) == 0 {
		return false
	}
	for _, n := range *list {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

type Limiter struct {
	tokens         float64
	maxTokens      float64
	refillRate     float64
	lastRefillTime time.Time
	mu             sync.Mutex
}

func NewLimiter(maxTokens, refillRate float64) *Limiter {
	return &Limiter{
		tokens:         maxTokens,
		maxTokens:      maxTokens,
		refillRate:     refillRate,
		lastRefillTime: time.Now(),
	}
}

func (l *Limiter) Allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastRefillTime).Seconds()

	l.tokens += elapsed * l.refillRate
	if l.tokens > l.maxTokens {
		l.tokens = l.maxTokens
	}
	l.lastRefillTime = now

	if l.tokens >= 1.0 {
		l.tokens -= 1.0
		return true
	}

	return false
}

type RateLimiter struct {
	limiters map[string]*Limiter
	mu       sync.RWMutex

	maxTokens  float64
	refillRate float64

	cleanupInterval time.Duration

	stop chan struct{}
}

func NewRateLimiter(maxTokens, refillRate float64) *RateLimiter {
	rl := &RateLimiter{
		limiters:        make(map[string]*Limiter),
		maxTokens:       maxTokens,
		refillRate:      refillRate,
		cleanupInterval: 10 * time.Minute,
		stop:            make(chan struct{}),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *RateLimiter) Close() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	select {
	case <-rl.stop:

	default:
		close(rl.stop)
	}
}

func (rl *RateLimiter) cleanupLoop() {
	t := time.NewTicker(rl.cleanupInterval)
	defer t.Stop()
	for {
		select {
		case <-rl.stop:
			return
		case <-t.C:
			rl.mu.Lock()
			rl.cleanup()
			rl.mu.Unlock()
		}
	}
}

func (rl *RateLimiter) GetLimiter(key string) *Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	limiter, exists = rl.limiters[key]
	if exists {
		return limiter
	}

	limiter = NewLimiter(rl.maxTokens, rl.refillRate)
	rl.limiters[key] = limiter

	return limiter
}

func (rl *RateLimiter) cleanup() {
	now := time.Now()
	for key, limiter := range rl.limiters {
		limiter.mu.Lock()
		inactive := now.Sub(limiter.lastRefillTime) > 30*time.Minute
		limiter.mu.Unlock()

		if inactive {
			delete(rl.limiters, key)
		}
	}
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	limitHeader := strconv.FormatFloat(rl.maxTokens, 'f', -1, 64)
	retryAfter := "60"
	if rl.refillRate > 0 {
		retryAfter = strconv.Itoa(int(1.0/rl.refillRate) + 1)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		key := GetClientIP(r)

		limiter := rl.GetLimiter(key)
		if !limiter.Allow() {
			w.Header().Set("X-RateLimit-Limit", limitHeader)
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("Retry-After", retryAfter)
			_ = response.Error(w, r, http.StatusTooManyRequests, "RATE_LIMITED", "Rate limit exceeded. Please try again later.", nil)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func GetClientIP(r *http.Request) string {
	peerHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		peerHost = r.RemoteAddr
	}
	peerIP := net.ParseIP(peerHost)

	if peerIP != nil && isTrustedProxy(peerIP) {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {

			parts := strings.Split(xff, ",")
			for i := len(parts) - 1; i >= 0; i-- {
				candidate := strings.TrimSpace(parts[i])
				ip := net.ParseIP(candidate)
				if ip == nil {
					continue
				}
				if !isTrustedProxy(ip) {
					return candidate
				}
			}
		}
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			return xri
		}
	}

	return peerHost
}
