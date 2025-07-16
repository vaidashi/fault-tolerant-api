package api

import (
	"net/http"
)

// getCircuitBreakerStatusHandler returns the current state of the circuit breaker
func (s *Server) getCircuitBreakerStatusHandler(w http.ResponseWriter, r *http.Request) {
	metrics := s.gracefulDegradation.GetMetrics()

	s.respondWithJSON(w, http.StatusOK, ApiResponse{Success: true, Data: metrics})
}

// resetCircuitBreakerHandler resets the circuit breaker to closed state
func (s *Server) resetCircuitBreakerHandler(w http.ResponseWriter, r *http.Request) {
	s.gracefulDegradation.Reset()

	s.respondWithJSON(w, http.StatusOK, ApiResponse{
		Success: true,
		Data: map[string]string{
			"message": "Circuit breaker reset successfully",
		},
	})
}