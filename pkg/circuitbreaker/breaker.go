package circuitbreaker

import (
	"sync"
	"time"
	"sync/atomic"
)

// State represents the state of the circuit breaker
type State int

const (
	StateClosed State = iota    // Normal operation, requests allowed
	StateHalfOpen               // Testing if service is healthy again
	StateOpen                   // Circuit is open, requests are not allowed
)

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	state           int32 // Using atomic operations
	failureThreshold int64
	resetTimeout    time.Duration
	halfOpenMaxCalls int64
	failureCount    int64
	halfOpenCalls   int64
	lastStateChange time.Time
	mutex           sync.RWMutex
}

// CircuitBreakerConfig configures a CircuitBreaker
type CircuitBreakerConfig struct {
	FailureThreshold int64
	ResetTimeout     time.Duration
	HalfOpenMaxCalls int64
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker(config CircuitBreakerConfig) *CircuitBreaker {
	return &CircuitBreaker{
		state:            int32(StateClosed),
		failureThreshold: config.FailureThreshold,
		resetTimeout:     config.ResetTimeout,
		halfOpenMaxCalls: config.HalfOpenMaxCalls,
		lastStateChange:  time.Now(),
	}
}

// Allow checks if a request is allowed based on the circuit breaker state
func (cb *CircuitBreaker) Allow() bool {
	state := State(atomic.LoadInt32(&cb.state))

	switch state {
	case StateClosed:
		return true
	case StateOpen:
		// Check if it's time to try half-open state
		cb.mutex.RLock()
		elaspsed := time.Since(cb.lastStateChange)
		cb.mutex.RUnlock()

		if elaspsed >= cb.resetTimeout {
			// Try to transition to half-open state
			if atomic.CompareAndSwapInt32(&cb.state, int32(StateOpen), int32(StateHalfOpen)) {
				cb.mutex.Lock()
				cb.lastStateChange = time.Now()
				atomic.StoreInt64(&cb.halfOpenCalls, 0)
				cb.mutex.Unlock()
			}
			return cb.Allow() // Retry with new state
		}
		return false 
	case StateHalfOpen:
		// Allow limited calls in half-open state
		calls := atomic.AddInt64(&cb.halfOpenCalls, 1)
		return calls <= cb.halfOpenMaxCalls
	default:
		return false // Unknown state, deny request
	}
}

// Success reports a successful operation
func (cb *CircuitBreaker) Success() {
	state := State(atomic.LoadInt32(&cb.state))

	if state == StateHalfOpen {
		// If in half-open state and successful, transition to closed state
		if atomic.CompareAndSwapInt32(&cb.state, int32(StateHalfOpen), int32(StateClosed)) {
			cb.mutex.Lock()
			cb.lastStateChange = time.Now()
			atomic.StoreInt64(&cb.failureCount, 0)
			cb.mutex.Unlock()
		} else if state == StateClosed {
			// Reset failure count if in closed state
			atomic.StoreInt64(&cb.failureCount, 0)
		}
	}
}

// Failure reports a failed operation
func (cb *CircuitBreaker) Failure() {
	state := State(atomic.LoadInt32(&cb.state))

	if state == StateClosed {
		// Increment failure count
		failureCount := atomic.AddInt64(&cb.failureCount, 1)

		if failureCount >= cb.failureThreshold {
			// Transition to open state if threshold is reached
			if atomic.CompareAndSwapInt32(&cb.state, int32(StateClosed), int32(StateOpen)) {
				cb.mutex.Lock()
				cb.lastStateChange = time.Now()
				cb.mutex.Unlock()
			}
		}
	} else if state == StateHalfOpen {
		// In half-open state, treat as a failure and transition to open state
		if atomic.CompareAndSwapInt32(&cb.state, int32(StateHalfOpen), int32(StateOpen)) {
			cb.mutex.Lock()
			cb.lastStateChange = time.Now()
			cb.mutex.Unlock()
		}
	}
}

// GetState returns the current state of the circuit breaker
func (cb *CircuitBreaker) GetState() State {
	return State(atomic.LoadInt32(&cb.state))
}

// GetMetrics returns metrics about the circuit breaker
func (cb *CircuitBreaker) GetMetrics() map[string]interface{} {
	state := State(atomic.LoadInt32(&cb.state))
	
	cb.mutex.RLock()
	lastChange := cb.lastStateChange
	cb.mutex.RUnlock()
	
	var stateStr string
	
	switch state {
	case StateClosed:
		stateStr = "closed"
	case StateHalfOpen:
		stateStr = "half-open"
	case StateOpen:
		stateStr = "open"
	}
	
	return map[string]interface{}{
		"state":             stateStr,
		"failure_count":     atomic.LoadInt64(&cb.failureCount),
		"failure_threshold": cb.failureThreshold,
		"half_open_calls":   atomic.LoadInt64(&cb.halfOpenCalls),
		"reset_timeout":     cb.resetTimeout.String(),
		"last_state_change": lastChange,
		"time_in_state":     time.Since(lastChange).String(),
	}
}