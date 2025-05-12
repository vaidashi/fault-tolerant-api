package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type ApiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string     `json:"error,omitempty"`
}

// Order represents an order in the system
type Order struct {
	ID        string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Amount  float64   `json:"amount"`
	Status   string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	Description string `json:"description, omitempty"`
}

// Health represents the health check response
type Health struct {
	Status string `json:"status"`
	Version string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// healthCheckHandler handles the health check endpoint
func (s *Server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	health := Health{
		Status:    "ok",
		Version:   "0.1.0",
		Timestamp: time.Now().Format(time.RFC3339),
	}

	s.respondWithJSON(w, http.StatusOK, ApiResponse{
		Success: true,
		Data:    health,
	})
}

// getOrdersHandler returns a list of orders
func (s *Server) getOrdersHandler(w http.ResponseWriter, r *http.Request) {
	// mock for now
	orders := []Order{
		{ID: "1", CustomerID: "123", Amount: 100.50, Status: "completed", CreatedAt: time.Now()},
		{ID: "2", CustomerID: "456", Amount: 200.75, Status: "pending", CreatedAt: time.Now()},
	}

	s.respondWithJSON(w, http.StatusOK, ApiResponse{
		Success: true,
		Data:    orders,
	})
}

// createOrderHandler creates a new order
func (s *Server) createOrderHandler(w http.ResponseWriter, r *http.Request) {
	var order Order
	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&order); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}

	defer r.Body.Close()
	order.ID = "3" // mock ID
	order.CreatedAt = time.Now()
	order.Status = "pending"

	s.respondWithJSON(w, http.StatusCreated, ApiResponse{
		Success: true,
		Data:    order,
	})
}

// getOrderByIDHandler returns an order by ID
func (s *Server) getOrderByIDHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]
	// mock for now
	order := Order{
		ID:        id,
		CustomerID: "123",
		Amount: 100.50,
		Status:   "completed",
		CreatedAt: time.Now().Add(-24 * time.Hour),
	}

	s.respondWithJSON(w, http.StatusOK, ApiResponse{
		Success: true,
		Data:    order,
	})
}

// respondWithError sends a JSON response with an error message
func (s *Server) respondWithError(w http.ResponseWriter, code int, message string) {
	s.respondWithJSON(w, code, ApiResponse{
		Success: false,
		Error:   message,
	})
}

// respondWithJSON sends a JSON response
func (s *Server) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	
	if err != nil {
		s.logger.Error("Failed to marshal response", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(response)
}