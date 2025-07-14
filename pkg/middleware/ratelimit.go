package middleware

import (
	"net/http"
	"strings"

	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
	"github.com/vaidashi/fault-tolerant-api/pkg/ratelimit"
)

// RateLimiterMiddleware is a middleware that applies rate limiting to incoming requests
type RateLimiterMiddleware struct {
	globalLimiter *ratelimit.AdaptiveRateLimiter
	ipLimiter      *ratelimit.IPRateLimiter
	logger         logger.Logger
	trustForwardedFor bool
}

// RateLimiterConfig configures the rate limiter middleware
type RateLimiterConfig struct {
	GlobalMaxTokens  float64
	GlobalMaxRate    float64
	GlobalMinRate    float64
	GlobalThreshold  float64
	IPMaxTokens      float64
	IPRefillRate     float64
	TrustForwardedFor bool
}

// NewRateLimiterMiddleware creates a new rate limiter middleware
func NewRateLimiterMiddleware(cfg *RateLimiterConfig, logger logger.Logger) *RateLimiterMiddleware {
	return &RateLimiterMiddleware{
		globalLimiter: ratelimit.NewAdaptiveRateLimiter(
			cfg.GlobalMaxTokens,
			cfg.GlobalMaxRate,
			cfg.GlobalMinRate,
			cfg.GlobalThreshold,
		),
		ipLimiter: ratelimit.NewIPRateLimiter(
			cfg.IPMaxTokens,
			cfg.IPRefillRate,
		),
		logger: logger,
		trustForwardedFor: cfg.TrustForwardedFor,
	}
}

// Middleware returns a middleware function 
func (m *RateLimiterMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check global rate limit
		if !m.globalLimiter.Allow() {
			m.logger.Warn("Global rate limit exceeded", "method", r.Method, "path", r.URL.Path)

			w.Header().Set("Retry-After", "10") // Retry after 10 seconds
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("Global rate limit exceeded. Please try again later."))
			return
		}

		// Get the client's IP address
		ip := m.getClientIP(r)

		// Check IP rate limit
		if !m.ipLimiter.Allow(ip) {
			m.logger.Warn("IP rate limit exceeded", "method", r.Method, "path", r.URL.Path, "ip", ip)

			w.Header().Set("Retry-After", "60") // Retry after 60 seconds
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte("IP rate limit exceeded. Please try again later."))
			return
		}
		// Continue to the next handler if both limits are not exceeded
		next.ServeHTTP(w, r)
	})
}

// getClientIP extracts the client IP from the request
func (m *RateLimiterMiddleware) getClientIP(r *http.Request) string {
	// If configured to trust X-Forwarded-For, use it
	if m.trustForwardedFor {
		forwardedFor := r.Header.Get("X-Forwarded-For")
		if forwardedFor != "" {
			// X-Forwarded-For can contain multiple IPs; use the first one
			ips := strings.Split(forwardedFor, ",")
			return strings.TrimSpace(ips[0])
		}
	}
	
	// Fall back to RemoteAddr
	ip := r.RemoteAddr
	// Strip port if present
	if i := strings.LastIndex(ip, ":"); i != -1 {
		ip = ip[:i]
	}
	return ip
}

// Stop stops the rate limiters
func (m *RateLimiterMiddleware) Stop() {
	m.globalLimiter.Stop()
	m.ipLimiter.Stop()
}

// GetMetrics returns metrics about rate limiting
func (m *RateLimiterMiddleware) GetMetrics() map[string]interface{} {
	return m.globalLimiter.GetMetrics()
}

