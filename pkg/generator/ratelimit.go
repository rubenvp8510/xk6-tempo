package generator

import (
	"context"
	"sync"

	"golang.org/x/time/rate"
)

const (
	bytesPerMB             = 1024 * 1024
	defaultBurstMultiplier = 1.5
	defaultTargetMBps      = 1.0
	minBurstSize           = 1
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
		burstMultiplier = defaultBurstMultiplier
	}
	if targetMBps <= 0 {
		targetMBps = defaultTargetMBps
	}

	targetBytesPerSec := targetMBps * bytesPerMB
	burstSize := calculateBurstSize(targetBytesPerSec, burstMultiplier)
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

	// Wait for the required number of bytes (1 token = 1 byte)
	// If bytes exceed burst size, we need to wait in chunks
	for bytes > 0 {
		r.mu.Lock()
		limiter := r.limiter
		burstSize := limiter.Burst()
		r.mu.Unlock()

		waitAmount := bytes
		if waitAmount > burstSize {
			waitAmount = burstSize
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

	targetBytesPerSec := targetMBps * bytesPerMB
	burstSize := calculateBurstSize(targetBytesPerSec, defaultBurstMultiplier)

	r.targetBytesPerSec = targetBytesPerSec
	r.limiter = rate.NewLimiter(rate.Limit(targetBytesPerSec), burstSize)
}

// calculateBurstSize calculates the burst size based on target rate and multiplier
func calculateBurstSize(targetBytesPerSec float64, burstMultiplier float64) int {
	burstSize := int(targetBytesPerSec * burstMultiplier)
	if burstSize < minBurstSize {
		burstSize = minBurstSize
	}
	return burstSize
}
