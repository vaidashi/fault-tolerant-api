package api 

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/vaidashi/fault-tolerant-api/internal/config"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
	"github.com/vaidashi/fault-tolerant-api/internal/database"
	"github.com/vaidashi/fault-tolerant-api/internal/repository"
	"github.com/vaidashi/fault-tolerant-api/internal/service"
	"github.com/vaidashi/fault-tolerant-api/internal/outbox"
	"github.com/vaidashi/fault-tolerant-api/internal/handlers"
	"github.com/vaidashi/fault-tolerant-api/pkg/kafka"
	"github.com/vaidashi/fault-tolerant-api/pkg/retry"
)

type Server struct {
	config *config.Config
	logger logger.Logger
	router *mux.Router
	httpServer *http.Server
	db     *database.Database
	orderRepo *repository.OrderRepository
	outboxRepo *repository.OutboxRepository
	outboxProcessor *outbox.Processor
	orderService *service.OrderService
	kafkaProducer *kafka.Producer
	kafkaConsumer *kafka.Consumer
	dlqRepo *repository.DeadLetterRepository
	deadLetterProcessor *outbox.DeadLetterProcessor
}

// NewServer creates a new API server with the given configuration and logger.
func NewServer(cfg *config.Config, logger logger.Logger) *Server {
	r := mux.NewRouter()
	db, err := database.New(cfg, logger)

	if err != nil {
		logger.Error("Failed to connect to database", "error", err)
		// In a production app, you would handle this more gracefully
		panic(err)
	}
	
	// Run migrations
	if err := db.RunMigrations(); err != nil {
		logger.Error("Failed to run database migrations", "error", err)
		panic(err)
	}
	
	// Initialize repositories
	orderRepo := repository.NewOrderRepository(db, logger)
	outboxRepo := repository.NewOutboxRepository(db, logger)
	dlqRepo := repository.NewDeadLetterRepository(db, logger)

	// Initialize Kafka producer
    kafkaProducer, err := kafka.NewProducer(cfg.Kafka.Brokers, logger)

    if err != nil {
        logger.Error("Failed to create Kafka producer", "error", err)
        panic(err)
    }

	// Initialize services
	orderService := service.NewOrderService(orderRepo, outboxRepo, logger)

	// Initialize outbox processor
	backoffStrategy := retry.NewDefaultExponentialBackoff()
	processorConfig := &outbox.ProcessorConfig{
		PollingInterval: 5 * time.Second,
		BatchSize:       10,
		MaxRetries:      3,
		BackoffStrategy: backoffStrategy,
		UseDLQ:          true, 
	}
	outboxProcessor := outbox.NewProcessor(outboxRepo, dlqRepo, logger, processorConfig)

	// Create the dead letter processor
	    // Initialize dead letter processor
    dlqProcessorConfig := &outbox.DeadLetterProcessorConfig{
        PollingInterval: 30 * time.Second, // Process less frequently than outbox
        BatchSize:       5,
        MaxRetries:      5,                // More retries for DLQ processing
        BackoffStrategy: &retry.ExponentialBackoff{
            InitialInterval: 1 * time.Second,
            MaxInterval:     2 * time.Minute,
            Multiplier:      2.0,
            JitterFactor:    0.1,
        },
    }

    deadLetterProcessor := outbox.NewDeadLetterProcessor(dlqRepo, outboxRepo, logger, dlqProcessorConfig)
    
	// Register message handlers
    kafkaHandler := outbox.NewKafkaHandler(kafkaProducer, cfg.Kafka.OrdersTopic, logger)
    
	// Register handlers for different event types for outbox processor
	outboxProcessor.RegisterHandler("order_created", kafkaHandler)
    outboxProcessor.RegisterHandler("order_updated", kafkaHandler)
    outboxProcessor.RegisterHandler("order_status_changed", kafkaHandler)

	// For dead letter queue (same handlers)
    deadLetterProcessor.RegisterHandler("order_created", kafkaHandler)
    deadLetterProcessor.RegisterHandler("order_updated", kafkaHandler)
    deadLetterProcessor.RegisterHandler("order_status_changed", kafkaHandler)

	// Initialize Kafka consumer
    consumerConfig := &kafka.ConsumerConfig{
        Brokers:       cfg.Kafka.Brokers,
        Topics:        []string{cfg.Kafka.OrdersTopic},
        ConsumerGroup: cfg.Kafka.ConsumerGroup,
    }

	kafkaConsumer, err := kafka.NewConsumer(consumerConfig, logger)

    if err != nil {
        logger.Error("Failed to create Kafka consumer", "error", err)
        panic(err)
    }

	// Register event handlers for Kafka consumer
    orderEventsHandler := handlers.NewOrderEventsHandler(logger)
    kafkaConsumer.RegisterHandler(cfg.Kafka.OrdersTopic, orderEventsHandler)
	
	server := &Server{
		router: r,
		httpServer: &http.Server{
			Addr:         fmt.Sprintf(":%d", cfg.Port),
			Handler:      r,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		logger:    logger,
		config:    cfg,
		db:        db,
		orderRepo: orderRepo,
		outboxRepo: outboxRepo,
		orderService: orderService,
		outboxProcessor: outboxProcessor,
		kafkaProducer: kafkaProducer,
		kafkaConsumer: kafkaConsumer,
		dlqRepo: dlqRepo,
		deadLetterProcessor: deadLetterProcessor,
	}
	
	server.setupRoutes()
	// Start the processors
	outboxProcessor.Start()
	deadLetterProcessor.Start()

	// Start the Kafka consumer
    if err := kafkaConsumer.Start(); err != nil {
        logger.Error("Failed to start Kafka consumer", "error", err)
        // Non-fatal error, continue without the consumer
    }

	return server
}

// Start starts the HTTP server 
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop the processors
    s.outboxProcessor.Stop()
	s.deadLetterProcessor.Stop()
    
    // Stop the Kafka consumer
    if s.kafkaConsumer != nil {
        if err := s.kafkaConsumer.Stop(); err != nil {
            s.logger.Error("Error stopping Kafka consumer", "error", err)
        }
    }
    
    // Close the Kafka producer
    if s.kafkaProducer != nil {
        if err := s.kafkaProducer.Close(); err != nil {
            s.logger.Error("Error closing Kafka producer", "error", err)
        }
    }
    
    // Close database connection
    if err := s.db.Close(); err != nil {
        s.logger.Error("Error closing database connection", "error", err)
    }
    
    return s.httpServer.Shutdown(ctx)
}

// setupRoutes configures all the routes for our API
func (s *Server) setupRoutes() {
	// Add middleware for all routes
	s.router.Use(s.loggingMiddleware)
	
	// API v1 routes
	api := s.router.PathPrefix("/api/v1").Subrouter()
	
	// Health check endpoint
	api.HandleFunc("/health", s.healthCheckHandler).Methods(http.MethodGet)
	
	// Resource endpoints
	api.HandleFunc("/orders", s.getOrdersHandler).Methods(http.MethodGet)
	api.HandleFunc("/orders", s.createOrderHandler).Methods(http.MethodPost)
	api.HandleFunc("/orders/{id}", s.getOrderByIDHandler).Methods(http.MethodGet)
	api.HandleFunc("/orders/{id}", s.updateOrderHandler).Methods(http.MethodPut)
	api.HandleFunc("/orders/{id}", s.deleteOrderHandler).Methods(http.MethodDelete)
	api.HandleFunc("/orders/{id}/status", s.updateOrderStatusHandler).Methods(http.MethodPatch)

	 // Admin API for monitoring and management
    admin := s.router.PathPrefix("/api/v1/admin").Subrouter()
    admin.HandleFunc("/dead-letters", s.getDeadLettersHandler).Methods(http.MethodGet)
    admin.HandleFunc("/dead-letters/{id}/retry", s.retryDeadLetterHandler).Methods(http.MethodPost)
    admin.HandleFunc("/dead-letters/{id}/discard", s.discardDeadLetterHandler).Methods(http.MethodPost)
}

// Middleware for logging requests
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Call the next handler
		next.ServeHTTP(w, r)
		
		// Log after request is processed
		s.logger.Info("Request processed",
			"method", r.Method,
			"path", r.URL.Path,
			"duration", time.Since(start),
			"remoteAddr", r.RemoteAddr,
		)
	})
}