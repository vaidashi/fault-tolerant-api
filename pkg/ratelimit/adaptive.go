package ratelimit

import (
	"sync"
	"time"
	"sync/atomic"
	"runtime"
)

// AdaptiveRateLimiter adjusts rate limits based on system load
type AdaptiveRateLimiter struct {
	baseLimiter     *TokenBucket
	maxRate         float64
	minRate         float64
	currentRate     float64
	loadThreshold   float64 // 0.0-1.0 where 1.0 is 100% CPU utilization
	currentLoad     float64
	requestCount    int64
	successCount    int64
	rejectionCount  int64
	mutex           sync.Mutex
	stopChan        chan struct{}
	adaptationInterval time.Duration
}

// NewAdaptiveRateLimiter creates a new adaptive rate limiter
func NewAdaptiveRateLimiter(maxTokens, maxRate, minRate float64, loadThreshold float64) *AdaptiveRateLimiter {
	arl := &AdaptiveRateLimiter{
		baseLimiter:     NewTokenBucket(maxTokens, maxRate),
		maxRate:         maxRate,
		minRate:         minRate,
		currentRate:     maxRate,
		loadThreshold:   loadThreshold,
		adaptationInterval: 5 * time.Second,
		stopChan:        make(chan struct{}),
	}
	
	go arl.adaptationLoop()
	
	return arl
}

// Allow checks if a request can proceed based on the adaptive rate limit
func (arl *AdaptiveRateLimiter) Allow() bool {
	// Increment request count
	atomic.AddInt64(&arl.requestCount, 1)
	allowed := arl.baseLimiter.Allow()

	if allowed {
		// Increment success count
		atomic.AddInt64(&arl.successCount, 1)
	} else {
		// Increment rejection count
		atomic.AddInt64(&arl.rejectionCount, 1)
	}
	
	return allowed
}

// adaptationLoop adjusts the rate limit based on system load
func (arl *AdaptiveRateLimiter) adaptationLoop() {
	ticker := time.NewTicker(arl.adaptationInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			arl.adapt()
		case <-arl.stopChan:
			return
		}
	}
}

// adapt adjusts the current rate based on system load
func (arl *AdaptiveRateLimiter) adapt() {
	arl.mutex.Lock()
	defer arl.mutex.Unlock()

	// Get CPU utilization
	arl.updateCPULoad()

	// Adjust rate based on CPU load
	var newRate float64

	if arl.currentLoad > arl.loadThreshold {
		// Reduce rate if load is high, higher the load, the closer to minRate
		loadFactor := (arl.currentLoad - arl.loadThreshold) / (1.0 - arl.loadThreshold)

		if loadFactor > 1.0 {
			loadFactor = 1.0
		}
		newRate = arl.maxRate - (arl.maxRate - arl.minRate) * loadFactor
	} else {
		// Gradually increase rate if load is low, closer to maxRate
		loadFactor := arl.currentLoad / arl.loadThreshold
		newRate = arl.minRate + (arl.maxRate - arl.minRate) * (1.0 - loadFactor) 
	}

	// Apply the new rate
	arl.currentRate = newRate
	arl.baseLimiter.refillRate = newRate
}

// updateCPULoad calculates the current CPU load
func (arl *AdaptiveRateLimiter) updateCPULoad() {
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)

	// Use a number of goroutines as a proxy for load
	numGoroutines := runtime.NumGoroutine()
	maxGoroutines := 10000 // arbitrary limit for scaling

	// Linear scale from 0.0 to 1.0 based on number of goroutines
	arl.currentLoad = float64(numGoroutines) / float64(maxGoroutines)
	if arl.currentLoad > 1.0 {
		arl.currentLoad = 1.0
	}
}

// Stop stops the adaptive rate limiter
func (arl *AdaptiveRateLimiter) Stop() {
	close(arl.stopChan)
}

// GetMetrics returns metrics about the rate limiter
func (arl *AdaptiveRateLimiter) GetMetrics() map[string]interface{} {
	return map[string]interface{}{
		"current_rate":      arl.currentRate,
		"max_rate":          arl.maxRate,
		"min_rate":          arl.minRate,
		"current_load":      arl.currentLoad,
		"load_threshold":    arl.loadThreshold,
		"request_count":     atomic.LoadInt64(&arl.requestCount),
		"success_count":     atomic.LoadInt64(&arl.successCount),
		"rejection_count":   atomic.LoadInt64(&arl.rejectionCount),
		"available_tokens":  arl.baseLimiter.Available(),
	}
}

// Reset resets the rate limiter to its initial state
func (arl *AdaptiveRateLimiter) Reset() {
	arl.mutex.Lock()
	defer arl.mutex.Unlock()
	
	arl.baseLimiter.Reset()
	arl.currentRate = arl.maxRate
	arl.baseLimiter.refillRate = arl.maxRate
	
	atomic.StoreInt64(&arl.requestCount, 0)
	atomic.StoreInt64(&arl.successCount, 0)
	atomic.StoreInt64(&arl.rejectionCount, 0)
}
