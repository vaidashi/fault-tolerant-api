package outbox

import (
	"context"
	"fmt"
	"encoding/json"

	"github.com/vaidashi/fault-tolerant-api/internal/models"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

// LoggingHandler is a message handler that logs the outbox message
type LoggingHandler struct {
	logger logger.Logger
}

// NewLoggingHandler creates a new LoggingHandler
func NewLoggingHandler(logger logger.Logger) *LoggingHandler {
	return &LoggingHandler{
		logger: logger,
	}
}

// HandleMessage handles the outbox message by logging it
func (h *LoggingHandler) HandleMessage(ctx context.Context, message *models.OutboxMessage) error {
	var event models.OutboxMessageEvent

	if err := json.Unmarshal(message.Payload, &event); err != nil {
		return fmt.Errorf("failed to unmarshal outbox message: %w", err)
	}

	// Simulate message processing
	h.logger.Info("Handling outbox message",
	"messageID", message.ID,
	"eventType", message.EventType,
	"aggregateID", message.AggregateID,
	"eventID", event.EventID,
	"occurredAt", event.OccurredAt)
    
    
    return nil
}