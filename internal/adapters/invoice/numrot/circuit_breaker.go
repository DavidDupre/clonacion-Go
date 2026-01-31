package numrot

import (
	"context"
	"sync"
	"time"
)

// CircuitBreakerState represents the state of the circuit breaker
type CircuitBreakerState int

const (
	CircuitBreakerClosed   CircuitBreakerState = iota // Normal operation
	CircuitBreakerOpen                                // Circuit is open, requests fail fast
	CircuitBreakerHalfOpen                            // Testing if service recovered
)

// CircuitBreaker implements a simple circuit breaker pattern to prevent cascading failures
type CircuitBreaker struct {
	maxFailures      int           // Number of failures before opening circuit
	failureThreshold float64       // Failure rate threshold (0.0-1.0)
	cooldownPeriod   time.Duration // Time to wait before attempting half-open
	successThreshold int           // Number of successes needed to close from half-open

	mu              sync.RWMutex
	state           CircuitBreakerState
	failureCount    int
	successCount    int
	totalRequests   int
	lastFailureTime time.Time
	lastStateChange time.Time
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(maxFailures int, failureThreshold float64, cooldownPeriod time.Duration) *CircuitBreaker {
	if maxFailures <= 0 {
		maxFailures = 10 // Default to 10 failures
	}
	if failureThreshold <= 0 || failureThreshold > 1 {
		failureThreshold = 0.5 // Default to 50% failure rate
	}
	if cooldownPeriod <= 0 {
		cooldownPeriod = 30 * time.Second // Default to 30 seconds
	}

	return &CircuitBreaker{
		maxFailures:      maxFailures,
		failureThreshold: failureThreshold,
		cooldownPeriod:   cooldownPeriod,
		successThreshold: 3, // Need 3 successes to close from half-open
		state:            CircuitBreakerClosed,
	}
}

// Execute executes a function with circuit breaker protection
func (cb *CircuitBreaker) Execute(ctx context.Context, fn func() error) error {
	// Check if circuit is open
	cb.mu.RLock()
	state := cb.state
	lastStateChange := cb.lastStateChange
	cooldownPeriod := cb.cooldownPeriod
	cb.mu.RUnlock()

	if state == CircuitBreakerOpen {
		// Check if cooldown period has passed
		if time.Since(lastStateChange) < cooldownPeriod {
			return ErrCircuitBreakerOpen
		}
		// Transition to half-open
		cb.mu.Lock()
		if cb.state == CircuitBreakerOpen && time.Since(cb.lastStateChange) >= cooldownPeriod {
			cb.state = CircuitBreakerHalfOpen
			cb.successCount = 0
			cb.lastStateChange = time.Now()
		}
		cb.mu.Unlock()
	}

	// Execute the function
	err := fn()

	// Update circuit breaker state based on result
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.totalRequests++

	if err != nil {
		cb.failureCount++
		cb.lastFailureTime = time.Now()

		// Check if we should open the circuit
		failureRate := float64(cb.failureCount) / float64(cb.totalRequests)
		if cb.state == CircuitBreakerHalfOpen {
			// Any failure in half-open state opens the circuit
			cb.state = CircuitBreakerOpen
			cb.lastStateChange = time.Now()
		} else if cb.failureCount >= cb.maxFailures || failureRate >= cb.failureThreshold {
			// Open circuit if threshold exceeded
			if cb.state == CircuitBreakerClosed {
				cb.state = CircuitBreakerOpen
				cb.lastStateChange = time.Now()
			}
		}
	} else {
		// Success
		cb.successCount++

		if cb.state == CircuitBreakerHalfOpen {
			// If we have enough successes, close the circuit
			if cb.successCount >= cb.successThreshold {
				cb.state = CircuitBreakerClosed
				cb.failureCount = 0
				cb.successCount = 0
				cb.totalRequests = 0
				cb.lastStateChange = time.Now()
			}
		} else if cb.state == CircuitBreakerClosed {
			// Reset failure count on success (sliding window effect)
			// Only reset if we've had recent successes
			if cb.successCount > cb.failureCount {
				cb.failureCount = 0
			}
		}
	}

	return err
}

// State returns the current circuit breaker state
func (cb *CircuitBreaker) State() CircuitBreakerState {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Stats returns circuit breaker statistics
type CircuitBreakerStats struct {
	State         CircuitBreakerState
	FailureCount  int
	SuccessCount  int
	TotalRequests int
	FailureRate   float64
}

// Stats returns current circuit breaker statistics
func (cb *CircuitBreaker) Stats() CircuitBreakerStats {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	failureRate := 0.0
	if cb.totalRequests > 0 {
		failureRate = float64(cb.failureCount) / float64(cb.totalRequests)
	}

	return CircuitBreakerStats{
		State:         cb.state,
		FailureCount:  cb.failureCount,
		SuccessCount:  cb.successCount,
		TotalRequests: cb.totalRequests,
		FailureRate:   failureRate,
	}
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = CircuitBreakerClosed
	cb.failureCount = 0
	cb.successCount = 0
	cb.totalRequests = 0
	cb.lastStateChange = time.Now()
}

var (
	// ErrCircuitBreakerOpen is returned when circuit breaker is open
	ErrCircuitBreakerOpen = &CircuitBreakerError{Message: "circuit breaker is open"}
)

// CircuitBreakerError represents a circuit breaker error
type CircuitBreakerError struct {
	Message string
}

func (e *CircuitBreakerError) Error() string {
	return e.Message
}
