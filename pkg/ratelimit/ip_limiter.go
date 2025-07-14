package ratelimit

import (
	"sync"
	"time"
)

// IPRateLimiter rate limits based on IP addresses
type IPRateLimiter struct {
	limiters map[string]*TokenBucket
	mu       sync.Mutex
	maxTokens float64
	refillRate float64
	cleanup *time.Ticker
	stopChan chan struct{}
}

// NewIPRateLimiter creates a new IPRateLimiter
func NewIPRateLimiter(maxTokens, refillRate float64) *IPRateLimiter {
	limiter := &IPRateLimiter{
		limiters:   make(map[string]*TokenBucket),
		maxTokens:  maxTokens,
		refillRate: refillRate,
		cleanup:    time.NewTicker(10 * time.Minute),
		stopChan:   make(chan struct{}),
	}
	
	// Start cleanup goroutine
	go limiter.cleanupLoop()
	
	return limiter
}

// Allow checks if a request from the given IP can proceed
func (ipl *IPRateLimiter) Allow(ip string) bool {
	limiter := ipl.getLimiter(ip)
	return limiter.Allow()
}

// getLimiter returns the token bucket for the given IP
func (ipl *IPRateLimiter) getLimiter(ip string) *TokenBucket {
	ipl.mu.Lock()
	defer ipl.mu.Unlock()

	limiter, exists := ipl.limiters[ip]

	if !exists {
		limiter = NewTokenBucket(ipl.maxTokens, ipl.refillRate)
		ipl.limiters[ip] = limiter
	}
	return limiter
}

// cleanupLoop periodically removes old limiters to prevent memory leaks
func (ipl *IPRateLimiter) cleanupLoop() {
	for {
		select {
		case <-ipl.cleanup.C:
			ipl.mu.Lock()
			// In a real implementation, you'd track last use time and remove old entries
			ipl.mu.Unlock()
		case <-ipl.stopChan:
			ipl.cleanup.Stop()
			return
		}
	}
}

// Stop stops the IP rate limiter
func (ipl *IPRateLimiter) Stop() {
	close(ipl.stopChan)
}
