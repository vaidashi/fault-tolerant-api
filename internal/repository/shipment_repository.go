package repository

import (
	"context"
	"fmt"
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
	"github.com/vaidashi/fault-tolerant-api/internal/models"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
	"github.com/vaidashi/fault-tolerant-api/internal/database"
)

// ShipmentRepository provides methods to interact with the shipment database
type ShipmentRepository struct {
	db *database.Database
	logger logger.Logger
}

// NewShipmentRepository creates a new ShipmentRepository instance
func NewShipmentRepository(db *database.Database, logger logger.Logger) *ShipmentRepository {
	return &ShipmentRepository{
		db:     db,
		logger: logger,
	}
}

// Create inserts a new shipment
func (r *ShipmentRepository) Create(ctx context.Context, shipment *models.Shipment) error {
	query := `
		INSERT INTO shipments (id, order_id, shipment_id, tracking_number, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := r.db.DB.ExecContext(
		ctx,
		query,
		shipment.ID,
		shipment.OrderID,
		shipment.ShipmentID,
		shipment.TrackingNumber,
		shipment.Status,
		shipment.CreatedAt,
		shipment.UpdatedAt,
	)

	if err != nil {
		r.logger.Error("Failed to create shipment", "error", err, "shipmentID", shipment.ID)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return nil
}

// GetByID retrieves a shipment by its ID
func (r *ShipmentRepository) GetByID(ctx context.Context, id string) (*models.Shipment, error) {
	query := `
		SELECT id, order_id, shipment_id, tracking_number, status, created_at, updated_at
		FROM shipments
		WHERE id = $1
	`

	var shipment models.Shipment
	err := r.db.DB.GetContext(ctx, &shipment, query, id)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		r.logger.Error("Failed to get shipment", "error", err, "shipmentID", id)
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return &shipment, nil
}

// GetByOrderID retrieves shipments for an order
func (r *ShipmentRepository) GetByOrderID(ctx context.Context, orderID string) ([]*models.Shipment, error) {
	query := `
		SELECT id, order_id, shipment_id, tracking_number, status, created_at, updated_at
		FROM shipments
		WHERE order_id = $1
		ORDER BY created_at DESC
	`

	var shipments []*models.Shipment
	err := r.db.DB.SelectContext(ctx, &shipments, query, orderID)

	if err != nil {
		r.logger.Error("Failed to get shipments by order ID", "error", err, "orderID", orderID)
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return shipments, nil
}

// UpdateStatus updates a shipment's status
func (r *ShipmentRepository) UpdateStatus(ctx context.Context, id, status string) error {
	query := `
		UPDATE shipments
		SET status = $1, updated_at = NOW()
		WHERE id = $2
	`

	result, err := r.db.DB.ExecContext(
		ctx,
		query,
		status,
		id,
	)

	if err != nil {
		r.logger.Error("Failed to update shipment status", "error", err, "shipmentID", id)
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

// CreateInTx creates a shipment within a transaction
func (r *ShipmentRepository) CreateInTx(tx *sqlx.Tx, shipment *models.Shipment) error {
	query := `
		INSERT INTO shipments (id, order_id, shipment_id, tracking_number, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`

	_, err := tx.Exec(
		query,
		shipment.ID,
		shipment.OrderID,
		shipment.ShipmentID,
		shipment.TrackingNumber,
		shipment.Status,
		shipment.CreatedAt,
		shipment.UpdatedAt,
	)

	if err != nil {
		return fmt.Errorf("failed to create shipment in transaction: %w", err)
	}

	return nil
}