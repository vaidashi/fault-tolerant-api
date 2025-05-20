package kafka

import (
	"context"
	"fmt"
	"sync"

	"github.com/Shopify/sarama"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

// MessageHandler is the interface for handling messages from Kafka
type MessageHandler interface {
	HandleMessage(ctx context.Context, msg *sarama.ConsumerMessage) error
}

// Consumer is a wrapper around sarama.ConsumerGroup
type Consumer struct {
	consumerGroup sarama.ConsumerGroup
	topics        []string
	handlers map[string]MessageHandler
	logger        logger.Logger
	wg            sync.WaitGroup
	ctx           context.Context
	cancel        context.CancelFunc
}

// ConsumerConfig is the configuration for the Kafka consumer
type ConsumerConfig struct {
	Brokers []string
	Topics []string
	ConsumerGroup string
}

// NewConsumer creates a new Kafka consumer
func NewConsumer(cfg *ConsumerConfig, logger logger.Logger) (*Consumer, error) {
	saramaCfg := sarama.NewConfig()
	saramaCfg.Consumer.Return.Errors = true
	saramaCfg.Consumer.Offsets.Initial = sarama.OffsetOldest

	consumerGroup, err := sarama.NewConsumerGroup(cfg.Brokers, cfg.ConsumerGroup, saramaCfg)

	if err != nil {
		return nil, fmt.Errorf("failed to create consumer group: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Consumer{
		consumerGroup: consumerGroup,
		topics: cfg.Topics,
		handlers: make(map[string]MessageHandler),
		logger: logger,
		ctx: ctx,
		cancel: cancel,
	}, nil
}

// RegisterHandler registers a message handler for a specific topic
func (c *Consumer) RegisterHandler(topic string, handler MessageHandler) {
	c.handlers[topic] = handler
}

// Start starts the Kafka consumer
func (c *Consumer) Start() error {
	if len(c.topics) == 0 {
		return fmt.Errorf("no topics to consume")
	}

	c.wg.Add(1)

	go func() {
		defer c.wg.Done()

		// Keep trying to join the consumer group until successful
		for {
			if err := c.consumerGroup.Consume(c.ctx, c.topics, c); err != nil {
				c.logger.Error("Kafka consumer error", "error", err)

				// Check if the context is done, indicating shutdown
				if c.ctx.Err() != nil {
					return
				}

				// Otherwise continue to retry
				c.logger.Info("Retrying to join consumer group")
				continue
			}

			if c.ctx.Err() != nil {
				return
			}
		}
	}()

	c.logger.Info("Kafka consumer started", "topics", c.topics)
	return nil
}

// Stop stops the Kafka consumer
func (c *Consumer) Stop() error {
	c.cancel()
	c.wg.Wait()
	return c.consumerGroup.Close()
}

// Setup is run when the consumer group is first created, (required by ConsumerGroupHandler interface)
func (c *Consumer) Setup(sarama.ConsumerGroupSession) error {
	// Nothing to do here
	return nil
}

// Cleanup is run when the consumer group is closed, (required by ConsumerGroupHandler interface)
func (c *Consumer) Cleanup(sarama.ConsumerGroupSession) error {
	// Nothing to do here
	return nil
}

// ConsumeClaim is run when a new claim is received, (required by ConsumerGroupHandler interface)
func (c *Consumer) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
			case msg, ok := <-claim.Messages():
				if !ok {
					return nil // Channel closed
				}

				c.logger.Debug("Received message from Kafka", 
					"topic", msg.Topic, 
					"partition", msg.Partition, 
					"offset", msg.Offset,
					"key", string(msg.Key))

				
				// Find the handler for the topic
				handler, exists := c.handlers[msg.Topic]

				if !exists {
					c.logger.Warn("No handler registered for topic", "topic", msg.Topic)
					session.MarkMessage(msg, "") // Mark the message as processed
					continue
				}

				// Handle the message
				if err := handler.HandleMessage(session.Context(), msg); err != nil {
					c.logger.Error("Error handling message",
						"error", err, 
						"topic", msg.Topic,
						"partition", msg.Partition,
						"offset", msg.Offset)
					
					// Don't mark the message, so it will be redelivered
					// In a production system, you'd want more sophisticated error handling
					continue
				}

				// Mark the message as processed
				session.MarkMessage(msg, "")
			
			case <-session.Context().Done():
				c.logger.Info("Consumer session context canceled, stopping consumption")
				return nil
			case <-c.ctx.Done():
				return nil
		}
	}
}