package control

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

// RateLimiter enforces a maximum number of operations per second.
// A zero-value RateLimiter is NOT usable — always construct via NewRateLimiter.
//
// Ported from control/rate_limiter.py. The Python original holds the lock for
// the entire acquire window (sleep + jitter) so that the next caller waits
// min_interval measured from the actual fire time of the prior caller, not from
// when the prior caller first grabbed the lock. This Go port preserves that
// invariant by keeping mutex held across the entire Acquire body.
type RateLimiter struct {
	maxPerSecond  float64
	minInterval   time.Duration
	lastRequest   time.Time
	mu            sync.Mutex
}

// NewRateLimiter creates a RateLimiter that permits at most maxPerSecond
// operations per second. Non-positive values default to 2/s (matching the
// Python default).
func NewRateLimiter(maxPerSecond float64) *RateLimiter {
	if maxPerSecond <= 0 {
		maxPerSecond = 2
	}
	return &RateLimiter{
		maxPerSecond: maxPerSecond,
		minInterval:  time.Duration(float64(time.Second) / maxPerSecond),
	}
}

// Acquire blocks the calling goroutine until at least minInterval has elapsed
// since the previous acquire, then applies a random jitter of 0–500ms (inside
// the lock) before returning. Returns ctx.Err() if the context is cancelled
// while waiting.
func (r *RateLimiter) Acquire(ctx context.Context) error {
	// Hold the lock across the full acquire body so that concurrent callers
	// serialize: each waits minInterval relative to the previous *actual* fire.
	r.mu.Lock()
	defer r.mu.Unlock()

	// Compute the raw wait needed to honor minInterval.
	if !r.lastRequest.IsZero() {
		elapsed := time.Since(r.lastRequest)
		if elapsed < r.minInterval {
			wait := r.minInterval - elapsed
			if err := sleepCtx(ctx, wait); err != nil {
				return err
			}
		}
	}

	// Jitter must run inside the lock so the next caller waits min_interval
	// since the actual fire time, not since the prior caller acquired the lock.
	jitter := time.Duration(rand.Float64()*0.5*float64(time.Second))
	if err := sleepCtx(ctx, jitter); err != nil {
		return err
	}

	r.lastRequest = time.Now()
	return nil
}

// sleepCtx sleeps for d, returning ctx.Err() if the context is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
