package middleware

import (
	"net/http"
	"sync"

	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
	"github.com/vaidashi/fault-tolerant-api/pkg/ratelimit"
)

// EndpointRateLimiterMiddleware provies per-endpoint rate limiting
type EndpointRateLimiterMiddleware struct {
	limiters map[string]*ratelimit.TokenBucket
	mu 	 sync.RWMutex
	defaultTokens float64
	defaultRate float64
	logger logger.Logger
}

// NewEndpointRateLimiterMiddleware creates a new EndpointRateLimiterMiddleware
func NewEndpointRateLimiterMiddleware(defaultTokens, defaultRate float64, logger logger.Logger) *EndpointRateLimiterMiddleware {
	return &EndpointRateLimiterMiddleware{
		limiters: make(map[string]*ratelimit.TokenBucket),
		defaultTokens: defaultTokens,
		defaultRate: defaultRate,
		logger: logger,
	}
}

// SetLimit sets the rate limit for a specific endpoint
func (m *EndpointRateLimiterMiddleware) SetLimit(endpoint string, maxTokens, refillRate float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.limiters[endpoint] = ratelimit.NewTokenBucket(maxTokens, refillRate)
}

// getLimiter gets or creates a rate limiter for the specified endpoint
func (m *EndpointRateLimiterMiddleware) getLimiter(endpoint string) *ratelimit.TokenBucket {
	m.mu.RLock()
	limiter, exists := m.limiters[endpoint]
	m.mu.RUnlock()

	if exists {
		return limiter
	}

	// Create a new limiter with default values if it doesn't exist
	m.mu.Lock()
	defer m.mu.Unlock()

	limiter = ratelimit.NewTokenBucket(m.defaultTokens, m.defaultRate)
	m.limiters[endpoint] = limiter
	return limiter
}

// Middleware returns a middleware function for per-endpoint rate limiting
func (m *EndpointRateLimiterMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Use the request path as the endpoint key
		endpoint := r.Method + ":" + r.URL.Path
		
		// Get the limiter for this endpoint
		limiter := m.getLimiter(endpoint)
		
		// Check if request is allowed
		if !limiter.Allow() {
			m.logger.Warn("Endpoint rate limit exceeded",
				"endpoint", endpoint,
				"method", r.Method,
				"path", r.URL.Path)
			
			w.Header().Set("Retry-After", "5") // Suggest retry after 5 seconds
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("Endpoint rate limit exceeded. Please try again later."))
			return
		}
		
		// Continue to the next handler
		next.ServeHTTP(w, r)
	})
}

// GetAllLimits returns all configured endpoint limits
func (m *EndpointRateLimiterMiddleware) GetAllLimits() map[string]map[string]float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	result := make(map[string]map[string]float64)
	
	for endpoint, limiter := range m.limiters {
		result[endpoint] = map[string]float64{
			"max_tokens": limiter.MaxTokens(),
			"refill_rate": limiter.RefillRate(),
			"available": limiter.Available(),
		}
	}
	
	return result
}