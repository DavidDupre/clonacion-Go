package numrot

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ConcurrentRequestLimiter limits the number of concurrent requests to Numrot API
// Numrot has a limit of 1000 concurrent requests per token
type ConcurrentRequestLimiter struct {
	semaphore     chan struct{} // Buffered channel with capacity = maxConcurrent
	maxConcurrent int
	mu            sync.RWMutex
	activeCount   int
	waitCount     int64
	totalAcquired int64
}

// NewConcurrentRequestLimiter creates a new concurrency limiter
func NewConcurrentRequestLimiter(maxConcurrent int) *ConcurrentRequestLimiter {
	if maxConcurrent <= 0 {
		maxConcurrent = 1000 // Default to Numrot's limit
	}
	if maxConcurrent > 1000 {
		maxConcurrent = 1000 // Enforce Numrot's maximum limit
	}

	return &ConcurrentRequestLimiter{
		semaphore:     make(chan struct{}, maxConcurrent),
		maxConcurrent: maxConcurrent,
	}
}

// Acquire acquires a slot for a concurrent request
// Blocks until a slot is available or context is cancelled
func (l *ConcurrentRequestLimiter) Acquire(ctx context.Context) error {
	l.mu.Lock()
	l.waitCount++
	l.mu.Unlock()

	select {
	case l.semaphore <- struct{}{}:
		l.mu.Lock()
		l.activeCount++
		l.totalAcquired++
		l.waitCount--
		active := l.activeCount
		l.mu.Unlock()

		// Log warning if approaching limit
		if active > 900 {
			// Note: We don't have access to logger here, but this could be added
		}
		return nil
	case <-ctx.Done():
		l.mu.Lock()
		l.waitCount--
		l.mu.Unlock()
		return ctx.Err()
	}
}

// Release releases a slot after a request completes
func (l *ConcurrentRequestLimiter) Release() {
	<-l.semaphore
	l.mu.Lock()
	l.activeCount--
	l.mu.Unlock()
}

// ActiveCount returns the current number of active requests
func (l *ConcurrentRequestLimiter) ActiveCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.activeCount
}

// MaxConcurrent returns the maximum concurrent requests allowed
func (l *ConcurrentRequestLimiter) MaxConcurrent() int {
	return l.maxConcurrent
}

// Stats returns statistics about the limiter
type LimiterStats struct {
	MaxConcurrent int
	ActiveCount   int
	WaitCount     int64
	TotalAcquired int64
	Available     int
}

// Stats returns current statistics
func (l *ConcurrentRequestLimiter) Stats() LimiterStats {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return LimiterStats{
		MaxConcurrent: l.maxConcurrent,
		ActiveCount:   l.activeCount,
		WaitCount:     l.waitCount,
		TotalAcquired: l.totalAcquired,
		Available:     l.maxConcurrent - l.activeCount,
	}
}

// RateLimiter implements a token bucket rate limiter
type RateLimiter struct {
	tokens       chan struct{}
	refillTicker *time.Ticker
	rate         int // requests per second
	mu           sync.RWMutex
	closed       bool
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rate int) *RateLimiter {
	if rate <= 0 {
		rate = 100 // Default rate
	}

	rl := &RateLimiter{
		tokens: make(chan struct{}, rate),
		rate:   rate,
	}

	// Fill the bucket initially
	for i := 0; i < rate; i++ {
		rl.tokens <- struct{}{}
	}

	// Start refill ticker (refill at the specified rate)
	rl.refillTicker = time.NewTicker(time.Second / time.Duration(rate))
	go rl.refill()

	return rl
}

// refill continuously refills the token bucket
func (rl *RateLimiter) refill() {
	for range rl.refillTicker.C {
		rl.mu.RLock()
		closed := rl.closed
		rl.mu.RUnlock()

		if closed {
			return
		}

		// Try to add a token (non-blocking)
		select {
		case rl.tokens <- struct{}{}:
		default:
			// Bucket is full, skip
		}
	}
}

// Acquire acquires a token from the rate limiter
func (rl *RateLimiter) Acquire(ctx context.Context) error {
	select {
	case <-rl.tokens:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Rate returns the configured rate limit (requests per second)
func (rl *RateLimiter) Rate() int {
	rl.mu.RLock()
	defer rl.mu.RUnlock()
	return rl.rate
}

// Close stops the rate limiter
func (rl *RateLimiter) Close() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	if !rl.closed {
		rl.closed = true
		rl.refillTicker.Stop()
		close(rl.tokens)
	}
}

// BatchSplitter splits documents into batches of specified size
func BatchSplitter(documents []interface{}, batchSize int) [][]interface{} {
	if batchSize <= 0 {
		batchSize = 50 // Default batch size
	}

	var batches [][]interface{}
	for i := 0; i < len(documents); i += batchSize {
		end := i + batchSize
		if end > len(documents) {
			end = len(documents)
		}
		batches = append(batches, documents[i:end])
	}

	return batches
}

// ValidateConcurrencyConfig validates concurrency configuration
func ValidateConcurrencyConfig(maxConcurrent, batchSize int) error {
	if maxConcurrent <= 0 {
		return fmt.Errorf("maxConcurrent must be greater than 0")
	}
	if maxConcurrent > 1000 {
		return fmt.Errorf("maxConcurrent cannot exceed 1000 (Numrot limit per token)")
	}
	if batchSize <= 0 {
		return fmt.Errorf("batchSize must be greater than 0")
	}
	return nil
}
