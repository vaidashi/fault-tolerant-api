package repository

import (
	"context"
	"fmt"
	"errors"
	"database/sql"

	"github.com/vaidashi/fault-tolerant-api/internal/database"
	"github.com/vaidashi/fault-tolerant-api/internal/models"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

var (
	ErrNotFound = errors.New("record not found")
	ErrDatabase = errors.New("database error")
)

// OrderRepository handles database operations for orders
type OrderRepository struct {
	db     *database.Database
	logger logger.Logger
}

// NewOrderRepository creates a new OrderRepository
func NewOrderRepository(db *database.Database, logger logger.Logger) *OrderRepository {
	return &OrderRepository{
		db:     db,
		logger: logger,
	}
}

// Create inserts a new order into the database
func (r *OrderRepository) Create(ctx context.Context, order *models.Order) error {
	query := `
		INSERT INTO orders (id, customer_id, amount, status, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.DB.ExecContext(
		ctx,
		query,
		order.ID,
		order.CustomerID,
		order.Amount,
		order.Status,
		order.Description,
		order.CreatedAt,
		order.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create order", "error", err, "orderID", order.ID)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return nil
}

// GetByID retrieves an order by its ID
func (r *OrderRepository) GetByID(ctx context.Context, id string) (*models.Order, error) {
	query := `
		SELECT id, customer_id, amount, status, description, created_at, updated_at
		FROM orders
		WHERE id = $1
	`

	var order models.Order
	err := r.db.DB.GetContext(ctx, &order, query, id)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		r.logger.Error("Failed to get order by ID", "error", err, "orderID", id)
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return &order, nil
}

// GetAll retrieves all orders with optional limit and offset
func (r *OrderRepository) GetAll(ctx context.Context, limit, offset int) ([]*models.Order, error) {
	query := `
		SELECT id, customer_id, amount, status, description, created_at, updated_at
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	var orders []*models.Order
	err := r.db.DB.SelectContext(ctx, &orders, query, limit, offset)

	if err != nil {
		r.logger.Error("Failed to get all orders", "error", err, "limit", limit, "offset", offset)
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return orders, nil
}

// Update updates an existing order
func (r *OrderRepository) Update(ctx context.Context, order *models.Order) error {
	query := `
		UPDATE orders
		SET customer_id = $1, amount = $2, status = $3, description = $4, updated_at = $5
		WHERE id = $6
	`

	result, err := r.db.DB.ExecContext(
		ctx,
		query,
		order.CustomerID,
		order.Amount,
		order.Status,
		order.Description,
		models.GetCurrentTime(), // Update the updated_at time
		order.ID,
	)

	if err != nil {
		r.logger.Error("Failed to update order", "error", err, "orderID", order.ID)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// Delete deletes an order by its ID
func (r *OrderRepository) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM orders WHERE id = $1`

	result, err := r.db.DB.ExecContext(ctx, query, id)

	if err != nil {
		r.logger.Error("Failed to delete order", "error", err, "orderID", id)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	rowsAffected, err := result.RowsAffected()

	if err != nil {
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

// Count counts the total number of orders
func (r *OrderRepository) Count(ctx context.Context) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM orders`
	
	err := r.db.DB.GetContext(ctx, &count, query)

	if err != nil {
		r.logger.Error("Failed to count orders", "error", err)
		return 0, fmt.Errorf("%w: %v", ErrDatabase, err)
	}
	
	return count, nil
}

// GetByCustomerID retrieves all orders for a specific customer
func (r *OrderRepository) GetByCustomerID(ctx context.Context, customerID string, limit, offset int) ([]*models.Order, error) {
	query := `
		SELECT id, customer_id, amount, status, description, created_at, updated_at
		FROM orders
		WHERE customer_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	var orders []*models.Order
	err := r.db.DB.SelectContext(ctx, &orders, query, customerID, limit, offset)
	
	if err != nil {
		r.logger.Error("Failed to get orders by customer ID", "error", err, "customerID", customerID)
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return orders, nil
}

