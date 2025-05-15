package service

import (
	"context"
	"fmt"

	"github.com/vaidashi/fault-tolerant-api/internal/models"
	"github.com/vaidashi/fault-tolerant-api/internal/repository"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

// OrderService handles order-related operations
type OrderService struct {
	orderRepo  *repository.OrderRepository
	outboxRepo *repository.OutboxRepository
	logger     logger.Logger
}

// NewOrderService creates a new OrderService
func NewOrderService(
	orderRepo *repository.OrderRepository, 
	outboxRepo *repository.OutboxRepository, 
	logger logger.Logger,
) *OrderService {
	return &OrderService{
		orderRepo:  orderRepo,
		outboxRepo: outboxRepo,
		logger:     logger,
	}
}

// CreateOrder creates a new order and publishes an outbox message
func (s *OrderService) CreateOrder(
	ctx context.Context,
	customerID string,
	amount float64,
	description string,
 ) (*models.Order, error) {
	order := models.NewOrder(customerID, amount, description)

	outboxMsg, err := models.NewOrderCreatedEvent(order)

	if err != nil {
		s.logger.Error("Failed to create outbox message", "error", err)
		return nil, fmt.Errorf("failed to create outbox message: %w", err)
	}

	// Begin transaction
	tx, err := s.orderRepo.BeginTx(ctx)

	if err != nil {
		return nil, err
	}

	// Rollback transaction if any error occurs
	defer func() {
		if err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				s.logger.Error("Failed to rollback transaction", "error", rollbackErr)
			}
		}
	}()

	// Create order in transaction
	if err = s.orderRepo.CreateInTx(tx, order); err != nil {
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

	s.logger.Info("Order created with outbox message", "order_id", order.ID, "outbox_id", outboxMsg.ID)
	return order, nil
 }

 // UpdateOrderStatus updates an order's status and adds an outbox message in a transaction
func (s *OrderService) UpdateOrderStatus(ctx context.Context, orderID, newStatus string) (*models.Order, error) {
    order, err := s.orderRepo.GetByID(ctx, orderID)

    if err != nil {
        return nil, err
    }

    if order.Status == newStatus {
        // No change needed
        return order, nil
    }

    oldStatus := order.Status
    order.Status = newStatus

    // Create outbox message
    outboxMsg, err := models.NewOrderStatusChangedEvent(order, oldStatus)

    if err != nil {
        s.logger.Error("Failed to create outbox message", "error", err)
        return nil, fmt.Errorf("failed to create outbox message: %w", err)
    }

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

    // Create outbox message in transaction
    if err = s.outboxRepo.CreateInTx(tx, outboxMsg); err != nil {
        return nil, err
    }

    // Commit transaction
    if err = tx.Commit(); err != nil {
        s.logger.Error("Failed to commit transaction", "error", err)
        return nil, fmt.Errorf("failed to commit transaction: %w", err)
    }

    s.logger.Info("Order status updated with outbox message", 
        "orderID", order.ID, 
        "oldStatus", oldStatus, 
        "newStatus", newStatus,
        "messageID", outboxMsg.ID)
    
    return order, nil
}

// GetOrder retrieves an order by ID
func (s *OrderService) GetOrder(ctx context.Context, id string) (*models.Order, error) {
    return s.orderRepo.GetByID(ctx, id)
}

// GetAllOrders retrieves all orders with pagination
func (s *OrderService) GetAllOrders(ctx context.Context, limit, offset int) ([]*models.Order, error) {
    return s.orderRepo.GetAll(ctx, limit, offset)
}

// CountOrders counts the total number of orders
func (s *OrderService) CountOrders(ctx context.Context) (int, error) {
    return s.orderRepo.Count(ctx)
}

// UpdateOrder updates an order's details and adds an outbox message in a transaction
func (s *OrderService) UpdateOrder(ctx context.Context, orderID string, customerID string, amount float64, description string) (*models.Order, error) {
    order, err := s.orderRepo.GetByID(ctx, orderID)

    if err != nil {
        return nil, err
    }

    // Update fields if provided
    if customerID != "" {
        order.CustomerID = customerID
    }
    if amount > 0 {
        order.Amount = amount
    }
    if description != "" {
        order.Description = description
    }

    // Create outbox message
    outboxMsg, err := models.NewOrderUpdatedEvent(order)

    if err != nil {
        s.logger.Error("Failed to create outbox message", "error", err)
        return nil, fmt.Errorf("failed to create outbox message: %w", err)
    }

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

    // Create outbox message in transaction
    if err = s.outboxRepo.CreateInTx(tx, outboxMsg); err != nil {
        return nil, err
    }

    // Commit transaction
    if err = tx.Commit(); err != nil {
        s.logger.Error("Failed to commit transaction", "error", err)
        return nil, fmt.Errorf("failed to commit transaction: %w", err)
    }

    s.logger.Info("Order updated with outbox message", "orderID", order.ID, "messageID", outboxMsg.ID)
    return order, nil
}

// DeleteOrder deletes an order
func (s *OrderService) DeleteOrder(ctx context.Context, id string) error {
    return s.orderRepo.Delete(ctx, id)
}