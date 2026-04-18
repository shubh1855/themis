package httpx

import (
	"context"
	"math"
	"math/rand"
	"time"
)

type Retrier struct {
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

func NewRetrier(maxRetries int, baseDelay, maxDelay time.Duration) *Retrier {
	return &Retrier{
		MaxRetries: maxRetries,
		BaseDelay:  baseDelay,
		MaxDelay:   maxDelay,
	}
}

func (r *Retrier) Do(ctx context.Context, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= r.MaxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		if _, ok := lastErr.(*RetryableError); !ok {
			return lastErr
		}

		if attempt < r.MaxRetries {
			delay := r.backoffDelay(attempt)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
		}
	}

	return lastErr
}

func (r *Retrier) backoffDelay(attempt int) time.Duration {
	delay := float64(r.BaseDelay) * math.Pow(2, float64(attempt))
	if delay > float64(r.MaxDelay) {
		delay = float64(r.MaxDelay)
	}
	jitter := delay * 0.25 * (rand.Float64()*2 - 1)
	return time.Duration(delay + jitter)
}
