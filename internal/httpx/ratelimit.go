package httpx

import (
	"context"
	"sync"
	"time"
)

// HostLimiter enforces per-host rate limits using a simple token bucket.
type HostLimiter struct {
	mu         sync.Mutex
	buckets    map[string]*bucket
	defaultRPS float64
}

type bucket struct {
	tokens    float64
	maxTokens float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewHostLimiter creates a rate limiter with the given default requests-per-second.
func NewHostLimiter(defaultRPS float64) *HostLimiter {
	return &HostLimiter{
		buckets:    make(map[string]*bucket),
		defaultRPS: defaultRPS,
	}
}

// Wait blocks until the rate limit for the given host allows a request.
// Respects context cancellation.
func (h *HostLimiter) Wait(ctx context.Context, host string) error {
	h.mu.Lock()
	b, ok := h.buckets[host]
	if !ok {
		b = &bucket{
			tokens:     h.defaultRPS,
			maxTokens:  h.defaultRPS,
			refillRate: h.defaultRPS,
			lastRefill: time.Now(),
		}
		h.buckets[host] = b
	}
	h.mu.Unlock()

	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		h.mu.Lock()
		b.refill()
		if b.tokens >= 1 {
			b.tokens--
			h.mu.Unlock()
			return nil
		}
		// Calculate wait time for next token
		waitDur := time.Duration(float64(time.Second) / b.refillRate)
		h.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDur):
		}
	}
}

func (b *bucket) refill() {
	now := time.Now()
	elapsed := now.Sub(b.lastRefill).Seconds()
	b.tokens += elapsed * b.refillRate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastRefill = now
}

// SetHostRPS overrides the rate limit for a specific host.
func (h *HostLimiter) SetHostRPS(host string, rps float64) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.buckets[host] = &bucket{
		tokens:     rps,
		maxTokens:  rps,
		refillRate: rps,
		lastRefill: time.Now(),
	}
}
