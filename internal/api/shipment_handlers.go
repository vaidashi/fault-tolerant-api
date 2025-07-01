package api

import (
	"errors"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/vaidashi/fault-tolerant-api/internal/repository"
)

// createShipmentHandler handles the creation of a shipment for an order
func (s *Server) createShipmentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	orderID := vars["id"]

	shipment, err := s.shipmentService.CreateShipmentForOrder(ctx, orderID)

	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.respondWithError(w, http.StatusNotFound, "Order not found")
			return
		}
		s.logger.Error("Failed to create shipment", "error", err, "orderID", orderID)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to create shipment")
		return
	}

	s.respondWithJSON(w, http.StatusCreated, ApiResponse{Success: true, Data: shipment})
}

// getShipmentsForOrderHandler retrieves all shipments for a given order
func (s *Server) getShipmentsForOrderHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	orderID := vars["id"]

	shipments, err := s.shipmentService.GetShipmentsByOrderID(ctx, orderID)

	if err != nil {
		s.logger.Error("Failed to retrieve shipments", "error", err, "orderID", orderID)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to retrieve shipments")
		return
	}

	s.respondWithJSON(w, http.StatusOK, ApiResponse{Success: true, Data: shipments})
}

// getShipmentHandler returns a single shipment
func (s *Server) getShipmentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	shipment, err := s.shipmentService.GetShipmentByID(ctx, id)

	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.respondWithError(w, http.StatusNotFound, "Shipment not found")
			return
		}
		s.logger.Error("Failed to get shipment", "error", err, "shipmentID", id)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to get shipment")
		return
	}
	
	s.respondWithJSON(w, http.StatusOK, ApiResponse{Success: true, Data: shipment})
}

// syncShipmentHandler syncs a shipment's status with the warehouse
func (s *Server) syncShipmentHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]
	
	shipment, err := s.shipmentService.UpdateShipmentStatus(ctx, id)

	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			s.respondWithError(w, http.StatusNotFound, "Shipment not found")
			return
		}
		s.logger.Error("Failed to sync shipment", "error", err, "shipmentID", id)
		s.respondWithError(w, http.StatusInternalServerError, "Failed to sync shipment with warehouse")
		return
	}
	
	s.respondWithJSON(w, http.StatusOK, ApiResponse{Success: true, Data: shipment})
}