package api

import (
	"encoding/json"
	"net/http"
)

// getRateLimitsHandler returns the current rate limit settings and metrics
func (s *Server) getRateLimitsHandler(w http.ResponseWriter, r *http.Request) {
	metrics := s.rateLimiter.GetMetrics()
	endpointLimits := s.endpointRateLimiter.GetAllLimits()

	response := map[string]interface{}{
		"global_metrics": metrics,
		"endpoint_limits": endpointLimits,
	}

	s.respondWithJSON(w, http.StatusOK, ApiResponse{Success: true, Data: response})
}

// setEndpointRateLimitHandler allows setting rate limits for specific endpoints
func (s *Server) setEndpointRateLimitHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Endpoint     string  `json:"endpoint"`
		MaxTokens    float64 `json:"max_tokens"`
		RefillRate   float64 `json:"refill_rate"`
	}

	decoder := json.NewDecoder(r.Body)
	
	if err := decoder.Decode(&req); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()
	
	// Validate input
	if req.Endpoint == "" {
		s.respondWithError(w, http.StatusBadRequest, "Endpoint is required")
		return
	}
	
	if req.MaxTokens <= 0 || req.RefillRate <= 0 {
		s.respondWithError(w, http.StatusBadRequest, "MaxTokens and RefillRate must be greater than zero")
		return
	}
	
	// Update the rate limit
	s.endpointRateLimiter.SetLimit(req.Endpoint, req.MaxTokens, req.RefillRate)
	
	s.respondWithJSON(w, http.StatusOK, ApiResponse{
		Success: true,
		Data: map[string]interface{}{
			"message": "Rate limit updated successfully",
			"endpoint": req.Endpoint,
			"max_tokens": req.MaxTokens,
			"refill_rate": req.RefillRate,
		},
	})
}