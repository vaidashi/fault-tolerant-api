package ratelimit

import (
	"sync"
	"time"
)

// TokenBucket implements a token bucket rate limiting algorithm
type TokenBucket struct {
	tokens float64
	maxTokens float64
	refillRate float64
	lastRefillTime time.Time
	mutex sync.Mutex
}

// NewTokenBucket creates a new token bucket rate limiter
func NewTokenBucket(maxTokens float64, refillRate float64) *TokenBucket {
	return &TokenBucket{
		tokens: maxTokens,
		maxTokens: maxTokens,
		refillRate: refillRate,
		lastRefillTime: time.Now(),
	}
}

// Allow checks if a request can proceed based on the token bucket algorithm
func (tb *TokenBucket) Allow() bool {
	return tb.AllowN(1)
}

// AllowN checks if n requests should be allowed based on available tokens
func (tb *TokenBucket) AllowN(n float64) bool {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	// Refill tokens based on the time elapsed since the last refill
	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime).Seconds()
	tb.lastRefillTime = now

	// Calculate new tokens to add
	newTokens := elapsed * tb.refillRate
	tb.tokens = min(tb.maxTokens, tb.tokens+newTokens)
	
	// Check if enough tokens are available
	if tb.tokens >= n {
		tb.tokens -= n
		return true
	}
	return false
}

// min returns the smaller of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// Reset resets the token bucket to its initial state
func (tb *TokenBucket) Reset() {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	tb.tokens = tb.maxTokens
	tb.lastRefillTime = time.Now()
}

// Available returns the number of available tokens in the bucket
func (tb *TokenBucket) Available() float64 {
	tb.mutex.Lock()
	defer tb.mutex.Unlock()

	// Refill tokens based on time elapsed since the last refill
	now := time.Now()
	elapsed := now.Sub(tb.lastRefillTime).Seconds()

	// Calculate new tokens to add but don't update the actual state
	newTokens := elapsed * tb.refillRate
	available := min(tb.maxTokens, tb.tokens+newTokens)
	
	return available
}