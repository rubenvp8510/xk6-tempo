package generator

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

// ByteRateLimiter limits throughput based on bytes per second
type ByteRateLimiter struct {
	targetBytesPerSec float64
	limiter           *rate.Limiter
	mu                sync.Mutex
}

// NewByteRateLimiter creates a new byte-based rate limiter
func NewByteRateLimiter(targetMBps float64, burstMultiplier float64) *ByteRateLimiter {
	if burstMultiplier <= 0 {
		burstMultiplier = 1.5
	}
	if targetMBps <= 0 {
		targetMBps = 1.0
	}

	targetBytesPerSec := targetMBps * 1024 * 1024 // Convert MB/s to bytes/s
	burstSize := int(targetBytesPerSec * burstMultiplier)
	if burstSize < 1 {
		burstSize = 1
	}

	limiter := rate.NewLimiter(rate.Limit(targetBytesPerSec), burstSize)

	return &ByteRateLimiter{
		targetBytesPerSec: targetBytesPerSec,
		limiter:           limiter,
	}
}

// Wait waits for permission to send the specified number of bytes
func (r *ByteRateLimiter) Wait(ctx context.Context, bytes int) error {
	if bytes <= 0 {
		return nil
	}

	r.mu.Lock()
	limiter := r.limiter
	r.mu.Unlock()

	// Wait for the required number of bytes (1 token = 1 byte)
	// If bytes exceed burst size, we need to wait in chunks
	for bytes > 0 {
		waitAmount := bytes
		if waitAmount > limiter.Burst() {
			waitAmount = limiter.Burst()
		}

		if err := limiter.WaitN(ctx, waitAmount); err != nil {
			return err
		}

		bytes -= waitAmount
	}

	return nil
}

// SetRate updates the rate limiter's target rate
func (r *ByteRateLimiter) SetRate(targetMBps float64) {
	if targetMBps <= 0 {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	targetBytesPerSec := targetMBps * 1024 * 1024
	burstSize := int(targetBytesPerSec * 1.5) // Use default burst multiplier
	if burstSize < 1 {
		burstSize = 1
	}

	r.targetBytesPerSec = targetBytesPerSec
	r.limiter = rate.NewLimiter(rate.Limit(targetBytesPerSec), burstSize)
}

