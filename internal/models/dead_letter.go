package models

import (
	"time"
)

// DeadLetterStatus represents the status of a dead letter message
type DeadLetterStatus string
const (
	DeadLetterStatusPending   DeadLetterStatus = "pending"
	DeadLetterStatusRetrying  DeadLetterStatus = "retrying"
	DeadLetterStatusResolved DeadLetterStatus = "resolved"
	DeadLetterStatusDiscarded DeadLetterStatus = "discarded"
)

// DeadLetterMessage represents a message in the dead-letter queue
type DeadLetterMessage struct {
	ID                 int64          `db:"id" json:"id"`
	OriginalMessageID  int64          `db:"original_message_id" json:"original_message_id"`
	AggregateType      string         `db:"aggregate_type" json:"aggregate_type"`
	AggregateID        string         `db:"aggregate_id" json:"aggregate_id"`
	EventType          string         `db:"event_type" json:"event_type"`
	Payload            []byte         `db:"payload" json:"payload"`
	ErrorMessage       string         `db:"error_message" json:"error_message"`
	FailureReason      string         `db:"failure_reason" json:"failure_reason"`
	RetryCount         int            `db:"retry_count" json:"retry_count"`
	LastRetryAt        *time.Time     `db:"last_retry_at" json:"last_retry_at,omitempty"`
	Status             string         `db:"status" json:"status"`
	CreatedAt          time.Time      `db:"created_at" json:"created_at"`
	ResolvedAt         *time.Time     `db:"resolved_at" json:"resolved_at,omitempty"`
}

// NewDeadLetterMessage creates a new dead letter message from an outbox message
func NewDeadLetterMessage(outboxMsg *OutboxMessage, errorMsg string, reason string) *DeadLetterMessage {
	return &DeadLetterMessage{
		OriginalMessageID: outboxMsg.ID,
		AggregateType:     outboxMsg.AggregateType,
		AggregateID:       outboxMsg.AggregateID,
		EventType:         outboxMsg.EventType,
		Payload:           outboxMsg.Payload,
		ErrorMessage:      errorMsg,
		FailureReason:     reason,
		RetryCount:        0,
		Status:            string(DeadLetterStatusPending),
		CreatedAt:         time.Now().UTC(),
	}
}