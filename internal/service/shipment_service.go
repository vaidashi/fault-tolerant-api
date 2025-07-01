package service

import (
	"context"
	"fmt"

	"github.com/vaidashi/fault-tolerant-api/internal/models"
	"github.com/vaidashi/fault-tolerant-api/internal/repository"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
	"github.com/vaidashi/fault-tolerant-api/internal/clients"
)

// ShipmentService provides methods to manage shipments
type ShipmentService struct {
	shipmentRepo *repository.ShipmentRepository
	orderRepo  *repository.OrderRepository
	outboxRepo *repository.OutboxRepository
	warehouseClient *clients.WarehouseClient
	logger logger.Logger
}

// NewShipmentService creates a new ShipmentService instance
func NewShipmentService(
	shipmentRepo *repository.ShipmentRepository,
	orderRepo *repository.OrderRepository,
	outboxRepo *repository.OutboxRepository,
	warehouseClient *clients.WarehouseClient,
	logger logger.Logger,
) *ShipmentService {
	return &ShipmentService{
		shipmentRepo: shipmentRepo,
		orderRepo: orderRepo,
		outboxRepo: outboxRepo,
		warehouseClient: warehouseClient,
		logger: logger,
	}
}

// CreateShipmentForOrder creates a shipment for a given order
func (s *ShipmentService) CreateShipmentForOrder(ctx context.Context, orderID string) (*models.Shipment, error) {
	order, err := s.orderRepo.GetByID(ctx, orderID)

	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Simplified shipment creation logic
	shipmentReq := &clients.ShipmentRequest{
		OrderID: order.ID,
		CustomerID: order.CustomerID,
		Products: []struct {
			ProductID string `json:"product_id"`
			Quantity  int    `json:"quantity"`
		}{
			{
				ProductID: "prod-sample",
				Quantity:  1,
			},
		},
		ShippingAddress: "123 Main St, Anytown, USA",
	}

	shipmentResp, err := s.warehouseClient.CreateShipment(ctx, shipmentReq)

	if err != nil {
		s.logger.Error("Failed to create shipment in warehouse", "error", err, "orderID", order.ID)
		return nil, fmt.Errorf("failed to create shipment: %w", err)
	}

		// Create a shipment record in our database
	shipment := models.NewShipment(
		order.ID,
		shipmentResp.ShipmentID,
		shipmentResp.TrackingNumber,
		string(models.ShipmentStatusPending),
	)

	// Save the shipment
	if err := s.shipmentRepo.Create(ctx, shipment); err != nil {
		s.logger.Error("Failed to save shipment", "error", err, "shipmentID", shipment.ID)
		return nil, fmt.Errorf("failed to save shipment: %w", err)
	}

	// Update the order status if needed 
	if order.Status == string(models.OrderStatusApproved) {
		oldStatus := order.Status
		order.Status = string(models.OrderStatusShipped)

		// Begin transaction
		tx, err := s.orderRepo.BeginTx(ctx)

		if err != nil {
			return nil, err
		}

		// Rollback transaction if any error occurs
		defer func() {
			if err != nil {
				if rbErr := tx.Rollback(); rbErr != nil {
					s.logger.Error("Failed to rollback transaction", "error", rbErr, "orderID", order.ID)
				}
			}
		}()

		// Update order status in transaction
		if err = s.orderRepo.UpdateInTx(tx, order); err != nil {
			return nil, err
		}

		// Create outbox message for status change
		outboxMsg, err := models.NewOrderStatusChangedEvent(order, oldStatus)
		if err != nil {
			return nil, err
		}
		
		// Create outbox message in transaction
		if err = s.outboxRepo.CreateInTx(tx, outboxMsg); err != nil {
			return nil, err
		}
		
		// Commit transaction
		if err = tx.Commit(); err != nil {
			s.logger.Error("Failed to commit transaction", "error", err)
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
	}

	return shipment, nil
}

// GetShipmentByID retrieves a shipment by ID
func (s *ShipmentService) GetShipmentByID(ctx context.Context, id string) (*models.Shipment, error) {
	return s.shipmentRepo.GetByID(ctx, id)
}

// GetShipmentsByOrderID retrieves shipments for an order
func (s *ShipmentService) GetShipmentsByOrderID(ctx context.Context, orderID string) ([]*models.Shipment, error) {
	return s.shipmentRepo.GetByOrderID(ctx, orderID)
}

// UpdateShipmentStatus updates a shipment's status and syncs with the warehouse
func (s *ShipmentService) UpdateShipmentStatus(ctx context.Context, id string) (*models.Shipment, error) {
	shipment, err := s.shipmentRepo.GetByID(ctx, id)

	if err != nil {
		return nil, fmt.Errorf("failed to get shipment: %w", err)
	}
	
	// Get status from warehouse
	warehouseResp, err := s.warehouseClient.GetShipmentStatus(ctx, shipment.ShipmentID)
	
	if err != nil {
		s.logger.Error("Failed to get shipment status from warehouse", "error", err, "shipmentID", shipment.ShipmentID)
		return nil, fmt.Errorf("failed to get shipment status from warehouse: %w", err)
	}
	
	// Map warehouse status to our status
	var newStatus string
	switch warehouseResp.Status {
	case "PENDING":
		newStatus = string(models.ShipmentStatusPending)
	case "SHIPPED":
		newStatus = string(models.ShipmentStatusShipped)
	case "DELIVERED":
		newStatus = string(models.ShipmentStatusDelivered)
	default:
		newStatus = string(models.ShipmentStatusPending)
	}
	
	// Only update if status has changed
	if newStatus != shipment.Status {
		if err := s.shipmentRepo.UpdateStatus(ctx, shipment.ID, newStatus); err != nil {
			s.logger.Error("Failed to update shipment status", "error", err, "shipmentID", shipment.ID)
			return nil, fmt.Errorf("failed to update shipment status: %w", err)
		}
		
		shipment.Status = newStatus
		
		// If shipment is delivered, update order status
		if newStatus == string(models.ShipmentStatusDelivered) {
			// Get the order
			order, err := s.orderRepo.GetByID(ctx, shipment.OrderID)
			
			if err != nil {
				s.logger.Error("Failed to get order for delivered shipment", "error", err, "orderID", shipment.OrderID)
				// Continue anyway, don't fail the whole operation
			} else if order.Status != string(models.OrderStatusDelivered) {
				oldStatus := order.Status
				order.Status = string(models.OrderStatusDelivered)
				
				// Begin transaction
				tx, err := s.orderRepo.BeginTx(ctx)
				
				if err != nil {
					return nil, err
				}
				
				// Rollback transaction in case of error
				defer func() {
					if err != nil {
						if rbErr := tx.Rollback(); rbErr != nil {
							s.logger.Error("Failed to rollback transaction", "error", rbErr)
						}
					}
				}()
				
				// Update order in transaction
				if err = s.orderRepo.UpdateInTx(tx, order); err != nil {
					return nil, err
				}
				
				// Create outbox message for status change
				outboxMsg, err := models.NewOrderStatusChangedEvent(order, oldStatus)
				if err != nil {
					return nil, err
				}
				
				// Create outbox message in transaction
				if err = s.outboxRepo.CreateInTx(tx, outboxMsg); err != nil {
					return nil, err
				}
				
				// Commit transaction
				if err = tx.Commit(); err != nil {
					s.logger.Error("Failed to commit transaction", "error", err)
					return nil, fmt.Errorf("failed to commit transaction: %w", err)
				}
			}
		}
	}
	
	return shipment, nil
}