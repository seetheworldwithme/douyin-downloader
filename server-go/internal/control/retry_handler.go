package control

import (
	"context"
	"fmt"
	"time"
)

// RetryHandler wraps an operation with bounded retries and backoff.
// Ported from control/retry_handler.py.
//
// maxRetries is the number of retries AFTER the initial attempt; total
// attempts = maxRetries + 1.
type RetryHandler struct {
	maxRetries  int
	retryDelays []int // seconds, indexed by attempt number (0-based retry)
}

// NewRetryHandler constructs a handler. maxRetries < 0 is clamped to 0.
// retryDelays are seconds; when nil the default [1,2,5] is used. If the slice
// is shorter than maxRetries, the last element repeats.
func NewRetryHandler(maxRetries int, retryDelays []int) *RetryHandler {
	if maxRetries < 0 {
		maxRetries = 0
	}
	if retryDelays == nil {
		retryDelays = []int{1, 2, 5}
	}
	return &RetryHandler{
		maxRetries:  maxRetries,
		retryDelays: retryDelays,
	}
}

// delayFor returns the backoff for the given 0-based retry attempt.
func (h *RetryHandler) delayFor(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	idx := attempt
	if idx >= len(h.retryDelays) {
		idx = len(h.retryDelays) - 1
	}
	return time.Duration(h.retryDelays[idx]) * time.Second
}

// ExecuteWithRetry runs fn up to maxRetries+1 times. On failure it sleeps for
// retryDelays[min(attempt, len-1)] seconds before the next attempt. The last
// error is returned (or ctx.Err() if the context was cancelled while backing
// off). Errors are logged via fmt rather than the stdlib log to keep this
// package dependency-free.
func (h *RetryHandler) ExecuteWithRetry(ctx context.Context, fn func(context.Context) error) error {
	var lastErr error
	totalAttempts := h.maxRetries + 1

	for attempt := 0; attempt < totalAttempts; attempt++ {
		// Bail out early if the context is already done.
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := fn(ctx); err != nil {
			lastErr = err
			if attempt < h.maxRetries {
				delay := h.delayFor(attempt)
				fmt.Printf("[RetryHandler] attempt %d failed: %v, retrying in %s...\n", attempt+1, err, delay)
				if sleepErr := sleepCtx(ctx, delay); sleepErr != nil {
					return sleepErr
				}
				continue
			}
		} else {
			return nil
		}
	}

	fmt.Printf("[RetryHandler] all %d attempts failed: %v\n", totalAttempts, lastErr)
	return lastErr
}
