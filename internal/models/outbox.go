package models 

import (
	"encoding/json"
	"time"
)

// OutboxStatus represents the status of an outbox message
type OutboxStatus string
const (
	OutboxStatusPending   OutboxStatus = "pending"
	OutboxStatusProcessing OutboxStatus = "processing"
	OutboxStatusCompleted OutboxStatus = "completed"
	OutboxStatusFailed    OutboxStatus = "failed"
)

// OutboxMessage represents a message to be published from the outbox table
type OutboxMessage struct {
	ID              int64       `db:"id" json:"id"`
	AggregateType     string      `db:"aggregate_type" json:"aggregate_type"`
	AggregateID       string      `db:"aggregate_id" json:"aggregate_id"`
	EventType         string      `db:"event_type" json:"event_type"`
	Payload           []byte      `db:"payload" json:"payload"`
	CreatedAt         time.Time   `db:"created_at" json:"created_at"`
	ProcessedAt       *time.Time  `db:"processed_at" json:"processed_at,omitempty"`
	ProcessingAttempts int        `db:"processing_attempts" json:"processing_attempts"`
	LastError         *string     `db:"last_error" json:"last_error,omitempty"`
	Status            OutboxStatus `db:"status" json:"status"`
}

// OutboxMessageEvent represents the event data in the outbox message
type OutboxMessageEvent struct {
	EventType string          `json:"event_type"`
	EventID   string          `json:"event_id"`
	AggregateId string          `json:"aggregate_id"`
	OccurredAt time.Time     `json:"occurred_at"`
	Data interface{} `json:"data"`
}

// NewOrderCreatedEvent creates a new order created event
func NewOrderCreatedEvent(order *Order) (*OutboxMessage, error) {
	event := OutboxMessageEvent{
		EventType: "order_created",
		EventID: GenerateID("evt"),
		AggregateId: order.ID,
		OccurredAt: time.Now().UTC(),
		Data: order,
	}

	payload, err := json.Marshal(event)

	if err != nil {
		return nil, err
	}

	return &OutboxMessage{
		EventType: event.EventType,
		Payload: payload,
		AggregateType: "order",
		AggregateID: order.ID,
		CreatedAt: time.Now().UTC(),
		ProcessingAttempts: 0,
		Status: OutboxStatusPending,
	}, nil
}

// NewOrderUpdatedEvent creates a new order updated event
func NewOrderUpdatedEvent(order *Order) (*OutboxMessage, error) {
	event := OutboxMessageEvent{
		EventType: "order_updated",
		EventID: GenerateID("evt"),
		AggregateId: order.ID,
		OccurredAt: time.Now().UTC(),
		Data: order,
	}

	payload, err := json.Marshal(event)

	if err != nil {
		return nil, err
	}

	return &OutboxMessage{
		EventType: event.EventType,
		Payload: payload,
		AggregateType: "order",
		AggregateID: order.ID,
		CreatedAt: time.Now().UTC(),
		ProcessingAttempts: 0,
		Status: OutboxStatusPending,
	}, nil
}

// NewOrderStatusChangedEvent creates a new event for order status change
func NewOrderStatusChangedEvent(order *Order, oldStatus string) (*OutboxMessage, error) {
	event := OutboxMessageEvent{
		EventType: "order_status_changed",
		EventID: GenerateID("evt"),
		AggregateId: order.ID,
		OccurredAt: time.Now().UTC(),
		Data: map[string]interface{}{
			"old_status": oldStatus,
			"new_status": order.Status,
			"order_id": order.ID,
			"customer_id": order.CustomerID,
		},
	}

	payload, err := json.Marshal(event)

	if err != nil {
		return nil, err
	}

	return &OutboxMessage{
		EventType: event.EventType,
		Payload: payload,
		AggregateType: "order",
		AggregateID: order.ID,
		CreatedAt: time.Now().UTC(),
		ProcessingAttempts: 0,
		Status: OutboxStatusPending,
	}, nil
}