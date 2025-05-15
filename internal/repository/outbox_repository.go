package repository

import (
    "context"
    "database/sql"
    "errors"
    "fmt"
    "time"

    // "github.com/jmoiron/sqlx"
    "github.com/vaidashi/fault-tolerant-api/internal/database"
    "github.com/vaidashi/fault-tolerant-api/internal/models"
    "github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

// OutboxRepository handles database operations for outbox messages
type OutboxRepository struct {
	db *database.Database
	logger logger.Logger
}

// NewOutboxRepository creates a new OutboxRepository
func NewOutboxRepository(db *database.Database, logger logger.Logger) *OutboxRepository {
	return &OutboxRepository{
		db:     db,
		logger: logger,
	}
}

// Create inserts a new outbox message into the database
func (r *OutboxRepository) Create(ctx context.Context, message *models.OutboxMessage) error {
	query := `
        INSERT INTO outbox_messages (
            aggregate_type, aggregate_id, event_type, payload, 
            created_at, status
        ) VALUES (
            $1, $2, $3, $4, $5, $6
        ) RETURNING id
    `

    var id int64

    err := r.db.DB.QueryRowContext(
        ctx,
        query,
        message.AggregateType,
        message.AggregateID,
        message.EventType,
        message.Payload,
        message.CreatedAt,
        message.Status,
    ).Scan(&id)

    if err != nil {
        r.logger.Error("Failed to create outbox message", "error", err)
        return fmt.Errorf("%w: %v", ErrDatabase, err)
    }

    message.ID = id
    return nil
}

// GetPendingMessages retrieves pending outbox messages from the database
func (r *OutboxRepository) GetPendingMessages(ctx context.Context, limit int) ([]*models.OutboxMessage, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, 
			   created_at, processed_at, processing_attempts, last_error, status
		FROM outbox_messages
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2
	`

	var messages []*models.OutboxMessage

	err := r.db.DB.SelectContext(
		ctx,
		&messages,
		query,
		models.OutboxStatusPending,
		limit,
	)

	if err != nil {
		r.logger.Error("Failed to get pending outbox messages", "error", err)
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return messages, nil
}

// MarkAsProcessing updates the status of an outbox message to processing
func (r *OutboxRepository) MarkAsProcessing(ctx context.Context, id int64) error {
	query := `
		UPDATE outbox_messages
		SET status = $1, processing_attempts = processing_attempts + 1
		WHERE id = $2
	`

	_, err := r.db.DB.ExecContext(
		ctx,
		query,
		models.OutboxStatusProcessing,
		id,
	)

	if err != nil {
		r.logger.Error("Failed to mark outbox message as processing", "error", err, "message_id", id)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return nil
}

// MarkAsCompleted updates the status of an outbox message to completed
func (r *OutboxRepository) MarkAsCompleted(ctx context.Context, id int64) error {
	query := `
		UPDATE outbox_messages
		SET status = $1, processed_at = $2
		WHERE id = $3
	`

	_, err := r.db.DB.ExecContext(
		ctx,
		query,
		models.OutboxStatusCompleted,
		time.Now().UTC(),
		id,
	)

	if err != nil {
		r.logger.Error("Failed to mark outbox message as completed", "error", err, "message_id", id)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return nil
}

// MarkAsFailed updates the status of an outbox message to failed
func (r *OutboxRepository) MarkAsFailed(ctx context.Context, id int64, errorMessage string) error {
	query := `
		UPDATE outbox_messages
		SET status = $1, last_error = $2
		WHERE id = $3
	`

	_, err := r.db.DB.ExecContext(
		ctx,
		query,
		models.OutboxStatusFailed,
		errorMessage,
		id,
	)

	if err != nil {
		r.logger.Error("Failed to mark outbox message as failed", "error", err, "message_id", id)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return nil
}

// GetMessage retrieves an outbox message by ID
func (r *OutboxRepository) GetMessage(ctx context.Context, id int64) (*models.OutboxMessage, error) {
	query := `
		SELECT id, aggregate_type, aggregate_id, event_type, payload, 
			   created_at, processed_at, processing_attempts, last_error, status
		FROM outbox_messages
		WHERE id = $1
	`

	var message models.OutboxMessage

	err := r.db.DB.GetContext(
		ctx,
		&message,
		query,
		id,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		r.logger.Error("Failed to get outbox message", "error", err, "message_id", id)
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return &message, nil
}

// CreateInTx creates a new outbox message within a transaction
func (r *OutboxRepository) CreateInTx(tx *sql.Tx, message *models.OutboxMessage) error {
	query := `
		INSERT INTO outbox_messages (
			aggregate_type, aggregate_id, event_type, payload, 
			created_at, status
		) VALUES (
			$1, $2, $3, $4, $5, $6
		) RETURNING id
	`

	var id int64

	err := tx.QueryRow(
		query,
		message.AggregateType,
		message.AggregateID,
		message.EventType,
		message.Payload,
		message.CreatedAt,
		message.Status,
	).Scan(&id)

	if err != nil {
		return fmt.Errorf("failed to create outbox message in transaction: %w", err)
	}

	message.ID = id
	return nil
}