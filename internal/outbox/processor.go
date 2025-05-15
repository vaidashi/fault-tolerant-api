package outbox

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/vaidashi/fault-tolerant-api/internal/models"
	"github.com/vaidashi/fault-tolerant-api/internal/repository"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

// MessageHandler defines the interface for handling outbox messages
type MessageHandler interface {
	HandleMessage(ctx context.Context, message *models.OutboxMessage) error
}

// Processor is responsible for processing outbox messages
type Processor struct {
	outboxRepo   *repository.OutboxRepository
	handlers map[string]MessageHandler
	pollingInterval time.Duration
	batchSize      int
	maxRetries      int
	logger         logger.Logger
	ctx 		 context.Context
	cancel context.CancelFunc
	wg            sync.WaitGroup
	running 	 bool
	mu sync.Mutex
}

// ProcessorConfig holds the configuration for the Processor
type ProcessorConfig struct {
	PollingInterval time.Duration
	BatchSize      int
	MaxRetries     int
}

// NewProcessor creates a new Processor
func NewProcessor(
	outboxRepo *repository.OutboxRepository,
	config ProcessorConfig,
	logger logger.Logger,
) *Processor {
	ctx, cancel := context.WithCancel(context.Background())

	return &Processor{
		outboxRepo:     outboxRepo,
		handlers:       make(map[string]MessageHandler),
		pollingInterval: config.PollingInterval,
		batchSize:      config.BatchSize,
		maxRetries:     config.MaxRetries,
		logger:         logger,
		ctx:           ctx,
		cancel:        cancel,
		running: 	false,
	}
}

// RegisterHandler registers a message handler for a specific event type
func (p *Processor) RegisterHandler(eventType string, handler MessageHandler) {
	p.handlers[eventType] = handler
}

// Start starts the outbox processor
func (p *Processor) Start() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return
	}

	p.running = true
	p.wg.Add(1)

	go func() {
		defer p.wg.Done()
		p.processOutbox()
	}()

	p.logger.Info("Outbox processor started",
		"pollingInterval", p.pollingInterval,
		"batchSize", p.batchSize)
}

// Stop stops the outbox processor
func (p *Processor) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return
	}

	p.cancel()
	p.wg.Wait()
	p.running = false

	p.logger.Info("Outbox processor stopped")
}

// processOutbox processes outbox messages in a loop
func (p *Processor) processOutbox() {
	ticker := time.NewTicker(p.pollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			if err := p.processBatch(); err != nil {
				p.logger.Error("Failed to process outbox batch", "error", err)
			}
		}
	}
}

// processBatch processes a batch of outbox messages
func (p *Processor) processBatch() error {
	ctx, cancel := context.WithTimeout(p.ctx, p.pollingInterval)
	defer cancel()

	messages, err := p.outboxRepo.GetPendingMessages(ctx, p.batchSize)

	if err != nil {
		return fmt.Errorf("failed to get pending messages: %w", err)
	}

	if len(messages) == 0 {
		p.logger.Info("No pending messages to process")
		return nil
	}

	p.logger.Info("Processing batch of outbox messages", "count", len(messages))

	for _, msg := range messages {
		if err := p.processMessage(ctx, msg); err != nil {
			 p.logger.Error("Failed to process message", 
                "error", err, 
                "messageID", msg.ID,
                "aggregateID", msg.AggregateID,
                "eventType", msg.EventType)
            
            // Continue processing other messages
            continue
		}
	}

	return nil
}

// processMessage processes a single outbox message
func (p *Processor) processMessage(ctx context.Context, msg *models.OutboxMessage) error {
    // Mark as processing
    if err := p.outboxRepo.MarkAsProcessing(ctx, msg.ID); err != nil {
        return fmt.Errorf("failed to mark message as processing: %w", err)
    }
    
    // Find appropriate handler
    handler, exists := p.handlers[msg.EventType]

    if !exists {
        errorMsg := fmt.Sprintf("no handler registered for event type: %s", msg.EventType)
        p.logger.Error(errorMsg, "messageID", msg.ID)
        
        // Mark as failed
        if err := p.outboxRepo.MarkAsFailed(ctx, msg.ID, errorMsg); err != nil {
            p.logger.Error("Failed to mark message as failed", "error", err, "messageID", msg.ID)
        }
        
        return fmt.Errorf(errorMsg)
    }
    
    // Handle the message
    err := handler.HandleMessage(ctx, msg)

    if err != nil {
        // Check if we've reached max retries
        if msg.ProcessingAttempts >= p.maxRetries {
            errorMsg := fmt.Sprintf("max retries reached: %s", err.Error())
            p.logger.Error(errorMsg, 
                "messageID", msg.ID, 
                "attempts", msg.ProcessingAttempts)
            
            // Mark as failed
            if markErr := p.outboxRepo.MarkAsFailed(ctx, msg.ID, errorMsg); markErr != nil {
                p.logger.Error("Failed to mark message as failed", "error", markErr, "messageID", msg.ID)
            }
            
            return fmt.Errorf("message failed after %d attempts: %w", msg.ProcessingAttempts, err)
        }
        
        // Return without updating status so it will be retried
        p.logger.Warn("Message processing failed, will retry", 
            "error", err, 
            "messageID", msg.ID, 
            "attempt", msg.ProcessingAttempts)
        return err
    }
    
    // Mark as completed
    if err := p.outboxRepo.MarkAsCompleted(ctx, msg.ID); err != nil {
        p.logger.Error("Failed to mark message as completed", "error", err, "messageID", msg.ID)
        return fmt.Errorf("failed to mark message as completed: %w", err)
    }
    
    p.logger.Info("Successfully processed message", 
        "messageID", msg.ID, 
        "aggregateID", msg.AggregateID, 
        "eventType", msg.EventType)
    
    return nil
}