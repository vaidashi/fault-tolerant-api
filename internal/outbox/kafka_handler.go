package outbox

import (
	"context"
	"fmt"
    "math/rand"

	"github.com/vaidashi/fault-tolerant-api/internal/models"
	"github.com/vaidashi/fault-tolerant-api/pkg/kafka"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

// KafkaHandler publishes outbox messages to Kafka
type KafkaHandler struct {
	logger logger.Logger
	producer *kafka.Producer
	topic string
    failureRate float64 // Probability of failure when publishing messages
}

// NewKafkaHandler creates a new KafkaHandler
func NewKafkaHandler(producer *kafka.Producer, topic string, logger logger.Logger) *KafkaHandler {
    return &KafkaHandler{
        producer: producer,
        topic:    topic,
        logger:   logger,
        failureRate: 0.2, // 20% failure rate for demonstration
    }
}

// HandleMessage handles an outbox message by publishing it to Kafka
func (h *KafkaHandler) HandleMessage(ctx context.Context, message *models.OutboxMessage) error {
    // Simulate random failures for testing
	if rand.Float64() < h.failureRate {
		h.logger.Warn("Simulating random failure in Kafka publishing", 
			"messageID", message.ID,
			"aggregateID", message.AggregateID)
		return fmt.Errorf("simulated random failure in Kafka publishing")
	}
    // Use the aggregate ID (order ID) as the Kafka message key for partitioning
    key := message.AggregateID
    
    h.logger.Info("Publishing message to Kafka", 
        "topic", h.topic, 
        "messageID", message.ID, 
        "aggregateID", message.AggregateID, 
        "eventType", message.EventType)
    
    // Send the message to Kafka
    err := h.producer.SendMessage(ctx, h.topic, key, message.Payload)

    if err != nil {
        h.logger.Error("Failed to publish message to Kafka", 
            "error", err, 
            "messageID", message.ID, 
            "aggregateID", message.AggregateID)
        return fmt.Errorf("failed to publish message to Kafka: %w", err)
    }
    
    h.logger.Info("Successfully published message to Kafka", 
        "messageID", message.ID, 
        "aggregateID", message.AggregateID)
    
    return nil
}