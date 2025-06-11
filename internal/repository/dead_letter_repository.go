package repository

import (
	"context"
	"fmt"
	"time"
	"database/sql"
	"errors"

	"github.com/vaidashi/fault-tolerant-api/internal/database"
	"github.com/vaidashi/fault-tolerant-api/internal/models"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

// DeadLetterRepository handles database operations related to dead letter messages
type DeadLetterRepository struct {
	db     *database.Database
	logger logger.Logger
}
// NewDeadLetterRepository creates a new DeadLetterRepository
func NewDeadLetterRepository(db *database.Database, logger logger.Logger) *DeadLetterRepository {
	return &DeadLetterRepository{
		db:     db,
		logger: logger,
	}
}

// Create inserts a new dead letter message
func (r *DeadLetterRepository) Create(ctx context.Context, message *models.DeadLetterMessage) error {
	query := `
		INSERT INTO dead_letter_messages (
			original_message_id, aggregate_type, aggregate_id, event_type, payload,
			error_message, failure_reason, retry_count, status, created_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10
		) RETURNING id
	`

	var id int64

	err := r.db.DB.QueryRowContext(
		ctx,
		query,
		message.OriginalMessageID,
		message.AggregateType,
		message.AggregateID,
		message.EventType,
		message.Payload,
		message.ErrorMessage,
		message.FailureReason,
		message.RetryCount,
		message.Status,
		message.CreatedAt,
	).Scan(&id)

	if err != nil {
		r.logger.Error("Failed to create dead letter message", "error", err)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	message.ID = id
	return nil
}

// GetPendingMessages retrieves pending dead letter messages
func (r *DeadLetterRepository) GetPendingMessages(ctx context.Context, limit int) ([]*models.DeadLetterMessage, error) {
	query := `
		SELECT 
			id, original_message_id, aggregate_type, aggregate_id, event_type, payload,
			error_message, failure_reason, retry_count, last_retry_at, status, created_at, resolved_at
		FROM 
			dead_letter_messages
		WHERE 
			status = $1
		ORDER BY 
			created_at ASC
		LIMIT $2
	`

	var messages []*models.DeadLetterMessage

	err := r.db.DB.SelectContext(
		ctx,
		&messages,
		query,
		string(models.DeadLetterStatusPending),
		limit,
	)

	if err != nil {
		r.logger.Error("Failed to get pending dead letter messages", "error", err)
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return messages, nil
}

// MarkAsRetrying marks a message as being retried
func (r *DeadLetterRepository) MarkAsRetrying(ctx context.Context, id int64) error {
	query := `
		UPDATE dead_letter_messages
		SET 
			status = $1,
			retry_count = retry_count + 1,
			last_retry_at = $2
		WHERE 
			id = $3
	`

	now := time.Now().UTC()

	_, err := r.db.DB.ExecContext(
		ctx,
		query,
		string(models.DeadLetterStatusRetrying),
		now,
		id,
	)

	if err != nil {
		r.logger.Error("Failed to mark dead letter message as retrying", "error", err, "messageID", id)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return nil
}

// MarkAsResolved marks a message as resolved
func (r *DeadLetterRepository) MarkAsResolved(ctx context.Context, id int64) error {
	query := `
		UPDATE dead_letter_messages
		SET 
			status = $1,
			resolved_at = $2
		WHERE 
			id = $3
	`

	now := time.Now().UTC()

	_, err := r.db.DB.ExecContext(
		ctx,
		query,
		string(models.DeadLetterStatusResolved),
		now,
		id,
	)

	if err != nil {
		r.logger.Error("Failed to mark dead letter message as resolved", "error", err, "messageID", id)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return nil
}

// MarkAsDiscarded marks a message as permanently discarded
func (r *DeadLetterRepository) MarkAsDiscarded(ctx context.Context, id int64, reason string) error {
	query := `
		UPDATE dead_letter_messages
		SET 
			status = $1,
			failure_reason = CONCAT(failure_reason, ' | Discarded: ', $2),
			resolved_at = $3
		WHERE 
			id = $4
	`

	now := time.Now().UTC()

	_, err := r.db.DB.ExecContext(
		ctx,
		query,
		string(models.DeadLetterStatusDiscarded),
		reason,
		now,
		id,
	)

	if err != nil {
		r.logger.Error("Failed to mark dead letter message as discarded", "error", err, "messageID", id)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return nil
}

// ResetToRetry resets a retrying message back to pending state
func (r *DeadLetterRepository) ResetToRetry(ctx context.Context, id int64) error {
	query := `
		UPDATE dead_letter_messages
		SET 
			status = $1
		WHERE 
			id = $2 AND status = $3
	`

	_, err := r.db.DB.ExecContext(
		ctx,
		query,
		string(models.DeadLetterStatusPending),
		id,
		string(models.DeadLetterStatusRetrying),
	)

	if err != nil {
		r.logger.Error("Failed to reset dead letter message to pending", "error", err, "messageID", id)
		return fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return nil
}

// GetMessage retrieves a message by ID
func (r *DeadLetterRepository) GetMessage(ctx context.Context, id int64) (*models.DeadLetterMessage, error) {
	query := `
		SELECT 
			id, original_message_id, aggregate_type, aggregate_id, event_type, payload,
			error_message, failure_reason, retry_count, last_retry_at, status, created_at, resolved_at
		FROM 
			dead_letter_messages
		WHERE 
			id = $1
	`

	var message models.DeadLetterMessage
	err := r.db.DB.GetContext(ctx, &message, query, id)
	
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		r.logger.Error("Failed to get dead letter message", "error", err, "messageID", id)
		return nil, fmt.Errorf("%w: %v", ErrDatabase, err)
	}

	return &message, nil
}
