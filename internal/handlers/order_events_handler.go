package handlers

import (
	"context"
	"fmt"
	"encoding/json"

	"github.com/Shopify/sarama"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
	"github.com/vaidashi/fault-tolerant-api/internal/models"
)

// OrderEventsHandler handles order events from Kafka
type OrderEventsHandler struct {
	logger logger.Logger
}

// NewOrderEventsHandler creates a new OrderEventsHandler
func NewOrderEventsHandler(logger logger.Logger) *OrderEventsHandler {
	return &OrderEventsHandler{
		logger: logger,
	}
}

// HandleMessage handles incoming order events from Kafka messages
func (h *OrderEventsHandler) HandleMessage(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var event models.OutboxMessageEvent
	
	if err := json.Unmarshal(msg.Value, &event); err != nil {
		h.logger.Error("failed to unmarshal message", "error", err)
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	h.logger.Info("Handling order event",
		"eventType", event.EventType,
		"eventId", event.EventID,
		"aggregateId", event.AggregateID,
		"occurredAt", event.OccurredAt,
	)

	// Handle different event types
	switch event.EventType {
	case "order_created":
		return h.handleOrderCreated(event)
	case "order_updated":
		return h.handleOrderUpdated(event)
	case "order_status_changed":
		return h.handleOrderStatusChanged(event)
	default:
		h.logger.Warn("unknown event type", "eventType", event.EventType)
		return nil
	}
}

// handleOrderCreated handles the order_created event
func (h *OrderEventsHandler) handleOrderCreated(event models.OutboxMessageEvent) error {
    h.logger.Info("Processing order created event", 
        "orderID", event.AggregateID, 
        "eventID", event.EventID,
	)
    
    // In a real application, you would:
    // 1. Extract the order data from event.Data
    // 2. Process the new order (e.g., send confirmation email, notify warehouse, etc.)
    // 3. Update any relevant systems
    
    return nil
}

// handleOrderUpdated handles the order_updated event
func (h *OrderEventsHandler) handleOrderUpdated(event models.OutboxMessageEvent) error {
    h.logger.Info("Processing order updated event", 
        "orderID", event.AggregateID, 
        "eventID", event.EventID)
    
    // In a real application, you would:
    // 1. Extract the updated order data from event.Data
    // 2. Update related systems or perform business logic
    // 3. Track order history
    
    return nil
}

// handleOrderStatusChanged handles the order_status_changed event
func (h *OrderEventsHandler) handleOrderStatusChanged(event models.OutboxMessageEvent) error {
    h.logger.Info("Processing order status changed event", 
        "orderID", event.AggregateID, 
        "eventID", event.EventID)
    
    // Extract status data
    data, ok := event.Data.(map[string]interface{})
	
    if !ok {
        h.logger.Error("Invalid event data format", "eventID", event.EventID)
        return fmt.Errorf("invalid event data format")
    }
    
    oldStatus, _ := data["old_status"].(string)
    newStatus, _ := data["new_status"].(string)
    
    h.logger.Info("Order status changed", 
        "orderID", event.AggregateID, 
        "oldStatus", oldStatus, 
        "newStatus", newStatus)
    
    // In a real application, you would:
    // 1. Perform different actions based on the new status
    // 2. For example, if status changed to "shipped", notify the customer
    // 3. If status changed to "delivered", update inventory, etc.
    
    return nil
}