package outbox

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vaidashi/fault-tolerant-api/internal/models"
	"github.com/vaidashi/fault-tolerant-api/internal/repository"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
	"github.com/vaidashi/fault-tolerant-api/pkg/retry"
)

// DeadLetterProcessor processes dead letter messages
type DeadLetterProcessor struct {
	dlqRepo         *repository.DeadLetterRepository
	outboxRepo      *repository.OutboxRepository
	handlers        map[string]MessageHandler
	pollingInterval time.Duration
	batchSize       int
	maxRetries      int
	backoffStrategy retry.BackoffStrategy
	logger          logger.Logger
	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	running         bool
	mu              sync.Mutex
}

// DeadLetterProcessorConfig holds the configuration for the DeadLetterProcessor
type DeadLetterProcessorConfig struct {
	PollingInterval time.Duration
	BatchSize       int
	MaxRetries      int
	BackoffStrategy retry.BackoffStrategy
}

// NewDeadLetterProcessor creates a new dead letter processor
func NewDeadLetterProcessor(
	dlqRepo *repository.DeadLetterRepository,
	outboxRepo *repository.OutboxRepository,
	logger logger.Logger,
	config *DeadLetterProcessorConfig,
) *DeadLetterProcessor {
	ctx, cancel := context.WithCancel(context.Background())
	
	// Use default backoff if none is provided
	backoffStrategy := config.BackoffStrategy

	if backoffStrategy == nil {
		backoffStrategy = retry.NewDefaultExponentialBackoff()
	}
	
	return &DeadLetterProcessor{
		dlqRepo:         dlqRepo,
		outboxRepo:      outboxRepo,
		handlers:        make(map[string]MessageHandler),
		pollingInterval: config.PollingInterval,
		batchSize:       config.BatchSize,
		maxRetries:      config.MaxRetries,
		backoffStrategy: backoffStrategy,
		logger:          logger,
		ctx:             ctx,
		cancel:          cancel,
		running:         false,
	}
}

// RegisterHandler registers a message handler for a specific event type
func (p *DeadLetterProcessor) RegisterHandler(eventType string, handler MessageHandler) {
	p.handlers[eventType] = handler
}

// Start starts the dead letter processor
func (p *DeadLetterProcessor) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return
	}

	p.running = true
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()
		p.processDLQ()
	}()

	p.logger.Info("Dead letter processor started",
		"pollingInterval", p.pollingInterval,
		"batchSize", p.batchSize,
		"maxRetries", p.maxRetries)
}

// Stop stops the dead letter processor
func (p *DeadLetterProcessor) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	
	if !p.running {
		return
	}
	
	p.cancel()
	p.wg.Wait()
	p.running = false
	
	p.logger.Info("Dead letter processor stopped")
}

// processDLQ processes messages from the dead letter queue
func (p *DeadLetterProcessor) processDLQ() {
	ticker := time.NewTicker(p.pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			if err := p.processBatch(); err != nil {
				p.logger.Error("Failed to process dead letter batch", "error", err)
			}
		}
	}
}

// processBatch processes a batch of messages from the dead letter queue
func (p *DeadLetterProcessor) processBatch() error {
	ctx, cancel := context.WithTimeout(p.ctx, p.pollingInterval)
	defer cancel()

	messages, err := p.dlqRepo.GetPendingMessages(ctx, p.batchSize)

	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	if len(messages) == 0 {
		p.logger.Info("No pending messages in dead letter queue")
		return nil
	}

	p.logger.Info("Processing batch of dead letter messages", "count", len(messages))

	for _, msg := range messages {
		if err := p.processMessage(ctx, msg); err != nil {
			p.logger.Error("Failed to process dead letter message", 
				"error", err,
				"messageID", msg.ID, 
				"aggregateID", msg.AggregateID,
				"eventType", msg.EventType,
				"retryCount", msg.RetryCount)

			continue
		}
	}

	return nil
}

// processMessage processes a single dead letter message
func (p *DeadLetterProcessor) processMessage(ctx context.Context, msg *models.DeadLetterMessage) error {
	if err := p.dlqRepo.MarkAsRetrying(ctx, msg.ID); err != nil {
		return fmt.Errorf("failed to mark message as retrying: %w", err)
	}

	handler, exists := p.handlers[msg.EventType]

	if !exists {
		errorMsg := fmt.Sprintf("no handler registered for event type %s", msg.EventType)
		p.logger.Error(errorMsg, "messageID", msg.ID)

		if err := p.dlqRepo.MarkAsDiscarded(ctx, msg.ID, "No handler available"); err != nil {
			p.logger.Error("Failed to mark message as discarded",
				"error", err,
				"messageID", msg.ID,)
		}

		return fmt.Errorf(errorMsg)
	}

	// Create an outbox message from the dead letter message
	outboxMsg := &models.OutboxMessage{
		ID:                0, // Will be assigned when created
		AggregateType:     msg.AggregateType,
		AggregateID:       msg.AggregateID,
		EventType:         msg.EventType,
		Payload:           msg.Payload,
		CreatedAt:         time.Now().UTC(),
		ProcessingAttempts: 0,
		Status:            models.OutboxStatusPending,
	}

	// Configure retry
	retryConfig := &retry.RetryConfig{
		MaxAttempts: p.maxRetries,
		BackoffStrategy: p.backoffStrategy,
		Logger: p.logger,
	}

	// Define the retryable function
	retryFunc := func() error {
		return handler.HandleMessage(ctx, outboxMsg)
	}

	// Define what to if all retries fail
	discardFunc := func(err error) error {
		reason := fmt.Sprintf("Failed to process message after %d attempts: %v", p.maxRetries, err)

		if markErr := p.dlqRepo.MarkAsDiscarded(ctx, msg.ID, reason); markErr != nil {
			p.logger.Error("Failed to mark message as discarded",
				"error", markErr,
				"messageID", msg.ID,)
		}

		return fmt.Errorf("message discard after %d retries: %w", p.maxRetries, err)
	}

	// Execute with retry and discard logic
	err := retry.RetryWithDiscard(ctx, retryFunc, retryConfig, discardFunc)

	if err != nil {
		p.logger.Error("Failed to process dead letter message with retries",
			"error", err,
			"messageID", msg.ID,
			"retryCount", msg.RetryCount)

		return err
	}

		// Mark as resolved
	if err := p.dlqRepo.MarkAsResolved(ctx, msg.ID); err != nil {
		p.logger.Error("Failed to mark dead letter message as resolved", "error", err, "messageID", msg.ID)
		return fmt.Errorf("failed to mark message as resolved: %w", err)
	}
	
	p.logger.Info("Successfully processed dead letter message", 
		"messageID", msg.ID, 
		"aggregateID", msg.AggregateID, 
		"eventType", msg.EventType)
	
	return nil
}