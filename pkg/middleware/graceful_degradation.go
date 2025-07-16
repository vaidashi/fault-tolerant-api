package middleware

import (
	"net/http"
	"time"
	"strings"

	"github.com/vaidashi/fault-tolerant-api/pkg/circuitbreaker"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

// GracefulDegradation handles graceful degradation of services under high load
type GracefulDegradation struct {
	breaker *circuitbreaker.CircuitBreaker
	logger logger.Logger
}

// NewGracefulDegradation creates a new graceful degradation middleware
func NewGracefulDegradation(logger logger.Logger) *GracefulDegradation {
	breaker := circuitbreaker.NewCircuitBreaker(circuitbreaker.CircuitBreakerConfig{
		FailureThreshold: 10,             // Open circuit after 10 failures
		ResetTimeout:     30 * time.Second, // Wait 30 seconds before trying again
		HalfOpenMaxCalls: 5,              // Allow 5 requests in half-open state
	})
	
	return &GracefulDegradation{
		breaker: breaker,
		logger:  logger,
	}
}

// Middleware returns a middleware function 
func (gd *GracefulDegradation) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc((func(w http.ResponseWriter, r *http.Request) {
		// Only apply circuit breaking to non-essential endpoints
		isEssential := isEssentialEndpoint(r.URL.Path)

		if !isEssential && !gd.breaker.Allow() {
			gd.logger.Warn("Circuit is open, request rejected",
				"path", r.URL.Path,
				"method", r.Method,
				"state", gd.breaker.GetState())
			
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("Service is temporarily unavailable. Please try again later."))
			return
		}

		// Use a custom response writer to track status codes
		wrappedWriter := newStatusCodeWriter(w)
		
		// Process the request
		next.ServeHTTP(wrappedWriter, r)
		
		// Update circuit breaker based on response
		if !isEssential {
			statusCode := wrappedWriter.statusCode
			if statusCode >= 500 {
				gd.breaker.Failure()
			} else if statusCode < 400 {
				gd.breaker.Success()
			}
		}
	}))
}

// isEssentialEndpoint determines if an endpoint is essential and shouldn't be circuit broken
func isEssentialEndpoint(path string) bool {
	// Health check and admin endpoints are essential
	return strings.HasPrefix(path, "/api/v1/health") ||
		strings.HasPrefix(path, "/api/v1/admin")
}

// statusCodeWriter is a wrapper around http.ResponseWriter that captures the status code
type statusCodeWriter struct {
	http.ResponseWriter
	statusCode int
}

// newStatusCodeWriter creates a new statusCodeWriter
func newStatusCodeWriter(w http.ResponseWriter) *statusCodeWriter {
	return &statusCodeWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // Default to 200 OK
	}
}

// WriteHeader captures the status code and passes it to the wrapped ResponseWriter
func (scw *statusCodeWriter) WriteHeader(code int) {
	scw.statusCode = code
	scw.ResponseWriter.WriteHeader(code)
}

// GetMetrics returns metrics about the circuit breaker
func (gd *GracefulDegradation) GetMetrics() map[string]interface{} {
	return gd.breaker.GetMetrics()
}

// Reset resets the circuit breaker
func (gd *GracefulDegradation) Reset() {
	gd.breaker.Reset()
}