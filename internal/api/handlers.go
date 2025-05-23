package api

import (
	"encoding/json"
	"net/http"
	"errors"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/vaidashi/fault-tolerant-api/internal/models"
	"github.com/vaidashi/fault-tolerant-api/internal/repository"
)

type ApiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string     `json:"error,omitempty"`
}

// OrderRequest represents the request body for creating/updating an order
type OrderRequest struct {
	CustomerID string    `json:"customer_id"`
	Amount  float64   `json:"amount"`
	Status   string    `json:"status, omitempty"`
	Description string `json:"description, omitempty"`
}

// PaginationResponse is a wrapper for paginated results
type PaginationResponse struct {
	Items      interface{} `json:"items"`
	TotalCount int         `json:"total_count"`
	Page       int         `json:"page"`
	PageSize   int         `json:"page_size"`
}

// Health represents the health check response
type Health struct {
	Status string `json:"status"`
	Version string `json:"version"`
	Timestamp string `json:"timestamp"`
}

// healthCheckHandler handles the health check endpoint
func (s *Server) healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
    
    health := map[string]string{
        "status":    "ok",
        "version":   "0.1.0",
        "timestamp": time.Now().Format(time.RFC3339),
    }
    
    // Check database connectivity
    dbStatus := "connected"

    if err := s.db.Ping(ctx); err != nil {
        dbStatus = "disconnected"
        health["status"] = "degraded"
        s.logger.Error("Health check: database ping error", "error", err)
    }
    health["database"] = dbStatus
    
    // For Kafka, we can only report if it's configured
    // Real Kafka health checking would require more complex logic
    kafkaStatus := "configured"
	
    if s.kafkaProducer == nil {
        kafkaStatus = "not_configured"
    }
    health["kafka"] = kafkaStatus
    
    s.respondWithJSON(w, http.StatusOK, ApiResponse{
        Success: true,
        Data:    health,
    })
}

// getOrdersHandler returns a list of orders
func (s *Server) getOrdersHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get pagination parameters
	page, err := strconv.Atoi(r.URL.Query().Get("page"))

	if err != nil || page < 1 {
		page = 1
	}

	pageSize, err := strconv.Atoi(r.URL.Query().Get("pageSize"))

	if err != nil || pageSize > 100 {
		pageSize = 10
	}

	offset := (page - 1) * pageSize
	orders, err := s.orderService.GetAllOrders(ctx, pageSize, offset)

	if err != nil {
		s.logger.Error("Failed to get orders", "error", err)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to retrieve orders")
		return
	}

	totalCount, err := s.orderService.CountOrders(ctx)

	if err != nil {
		s.logger.Error("Failed to count orders", "error", err)
	}

	response := PaginationResponse{
		Items:      orders,
		TotalCount: totalCount,
		Page:       page,
		PageSize:   pageSize,
	}

	s.respondWithJSON(w, http.StatusOK, ApiResponse{
		Success: true,
		Data:    response,
	})
}

// createOrderHandler creates a new order
func (s *Server) createOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	
	var req OrderRequest
	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&req); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()
	
	// Validate request
	if req.CustomerID == "" {
		s.respondWithError(w, http.StatusBadRequest, "Customer ID is required")
		return
	}
	
	if req.Amount <= 0 {
		s.respondWithError(w, http.StatusBadRequest, "Amount must be greater than zero")
		return
	}
	
	order, err := s.orderService.CreateOrder(ctx, req.CustomerID, req.Amount, req.Description)

	if err != nil {
		s.logger.Error("Failed to create order", "error", err)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to create order")
		return
	}
	
	s.respondWithJSON(w, http.StatusCreated, ApiResponse{Success: true, Data: order})
}

// getOrderByIDHandler returns an order by ID
func (s *Server) getOrderByIDHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]
	
	order, err := s.orderService.GetOrder(ctx, id)

	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.respondWithError(w, http.StatusNotFound, "Order not found")
			return
		}
		s.logger.Error("Failed to get order", "error", err, "orderID", id)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to retrieve order")
		return
	}
	
	s.respondWithJSON(w, http.StatusOK, ApiResponse{Success: true, Data: order})
}

// updateOrderHandler updates an existing order
func (s *Server) updateOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]
	
	// Parse and validate the request
	var req OrderRequest
	decoder := json.NewDecoder(r.Body)

	if err := decoder.Decode(&req); err != nil {
		s.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
		return
	}
	defer r.Body.Close()

	order, err := s.orderService.UpdateOrder(ctx, id, req.CustomerID, req.Amount, req.Description)

	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.respondWithError(w, http.StatusNotFound, "Order not found")
			return
		}
		s.logger.Error("Failed to update order", "error", err, "orderID", id)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to update order")
		return
	}
	
	s.respondWithJSON(w, http.StatusOK, ApiResponse{Success: true, Data: order})
}

// updateOrderStatusHandler updates an order's status
func (s *Server) updateOrderStatusHandler(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()
    vars := mux.Vars(r)
    id := vars["id"]
    
    // Parse the status from the request
    var statusRequest struct {
        Status string `json:"status"`
    }
    
    decoder := json.NewDecoder(r.Body)

    if err := decoder.Decode(&statusRequest); err != nil {
        s.respondWithError(w, http.StatusBadRequest, "Invalid request payload")
        return
    }
    defer r.Body.Close()
    
    if statusRequest.Status == "" {
        s.respondWithError(w, http.StatusBadRequest, "Status is required")
        return
    }
    
    // Validate the status (in a real app, you'd have more rigorous validation)
    validStatuses := map[string]bool{
        string(models.OrderStatusPending):   true,
        string(models.OrderStatusApproved):  true,
        string(models.OrderStatusRejected):  true,
        string(models.OrderStatusShipped):   true,
        string(models.OrderStatusDelivered): true,
        string(models.OrderStatusCancelled): true,
    }
    
    if !validStatuses[statusRequest.Status] {
        s.respondWithError(w, http.StatusBadRequest, "Invalid status value")
        return
    }
    
    
    order, err := s.orderService.UpdateOrderStatus(ctx, id, statusRequest.Status)
    
	if err != nil {
        if errors.Is(err, repository.ErrNotFound) {
            s.respondWithError(w, http.StatusNotFound, "Order not found")
            return
        }
        s.logger.Error("Failed to update order status", "error", err, "orderID", id)
        s.respondWithError(w, http.StatusInternalServerError, "Failed to update order status")
        return
    }
    
    s.respondWithJSON(w, http.StatusOK, ApiResponse{Success: true, Data: order})
}

// deleteOrderHandler deletes an order
func (s *Server) deleteOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]
	
	err := s.orderService.DeleteOrder(ctx, id)

	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.respondWithError(w, http.StatusNotFound, "Order not found")
			return
		}
		s.logger.Error("Failed to delete order", "error", err, "orderID", id)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to delete order")
		return
	}
	
	s.respondWithJSON(w, http.StatusOK, ApiResponse{Success: true, Data: map[string]string{"message": "Order deleted successfully"}})
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