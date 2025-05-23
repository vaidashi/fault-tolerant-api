package kafka

import (
	"context"
	"fmt"
	"time"
	"github.com/Shopify/sarama"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

// Producer is a wrapper around the Sarama producer
type Producer struct {
	producer sarama.SyncProducer
	logger   logger.Logger
}

// NewProducer creates a new Kafka producer
func NewProducer(brokers []string, logger logger.Logger) (*Producer, error) {
	config := sarama.NewConfig()
	config.Producer.RequiredAcks = sarama.WaitForAll // Wait for all replicas to acknowledge
	config.Producer.Retry.Max = 10
	config.Producer.Return.Successes = true // Return success message confirmations
	config.Producer.Retry.Backoff = 500 * time.Millisecond
	config.Producer.Timeout = 5 * time.Second

	producer, err := sarama.NewSyncProducer(brokers, config)

	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	return &Producer{
		producer: producer,
		logger:   logger,
	}, nil
}

// SendMessage sends a message to the specified topic
func (p *Producer) SendMessage(ctx context.Context, topic string, key string, value []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(value),
	}

	if key != "" {
		msg.Key = sarama.StringEncoder(key)
	}

	// Add a deadline to the context if given
	if deadline, ok := ctx.Deadline(); ok {
		msg.Metadata = deadline
	}

	partition, offset, err := p.producer.SendMessage(msg)

	if err != nil {
		p.logger.Error("Failed to send message to Kafka",
			"error", err,
			"topic", topic,
			"key", key)
		return fmt.Errorf("failed to send message to Kafka: %w", err)
	}

	p.logger.Debug("Message sent to Kafka",
		"topic", topic,
		"key", key,
		"partition", partition,
		"offset", offset)

	return nil
}

// Close closes the producer
func (p *Producer) Close() error {
	return p.producer.Close()
}