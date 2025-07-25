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
	"github.com/vaidashi/fault-tolerant-api/internal/clients"
	"github.com/vaidashi/fault-tolerant-api/pkg/middleware"
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
	warehouseClient *clients.WarehouseClient
	shipmentRepo *repository.ShipmentRepository
	shipmentService *service.ShipmentService
	rateLimiter *middleware.RateLimiterMiddleware
	endpointRateLimiter *middleware.EndpointRateLimiterMiddleware
	gracefulDegradation *middleware.GracefulDegradation
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

	// Initialize warehouse client
	warehouseClient := clients.NewWarehouseClient(cfg.WarehouseURL, logger)
	
	// Initialize repositories
	orderRepo := repository.NewOrderRepository(db, logger)
	outboxRepo := repository.NewOutboxRepository(db, logger)
	dlqRepo := repository.NewDeadLetterRepository(db, logger)
	shipmentRepo := repository.NewShipmentRepository(db, logger)

	// Initialize Kafka producer
    kafkaProducer, err := kafka.NewProducer(cfg.Kafka.Brokers, logger)

    if err != nil {
        logger.Error("Failed to create Kafka producer", "error", err)
        panic(err)
    }

	// Initialize services
	orderService := service.NewOrderService(orderRepo, outboxRepo, logger)
	shipmentService := service.NewShipmentService(shipmentRepo, orderRepo, outboxRepo, warehouseClient, logger)

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

	// Initialize dead letter processor
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

	// Initialize rate limiters
	rateLimiterConfig := &middleware.RateLimiterConfig{
		GlobalMaxTokens:   100,
		GlobalMaxRate:     20,  // 20 tokens per second
		GlobalMinRate:     5,   // 5 tokens per second when under high load
		GlobalThreshold:   0.7, // Start adapting at 70% load
		IPMaxTokens:       20,
		IPRefillRate:      1,   // 1 token per second per IP
		TrustForwardedFor: true,
	}

	rateLimiter := middleware.NewRateLimiterMiddleware(rateLimiterConfig, logger)
	gracefulDegradation := middleware.NewGracefulDegradation(logger)
	
	// Initialize endpoint rate limiter
	endpointRateLimiter := middleware.NewEndpointRateLimiterMiddleware(50, 10, logger)
	
	// Configure specific endpoint limits
	endpointRateLimiter.SetLimit("POST:/api/v1/orders", 10, 2)       // 2 orders/second
	endpointRateLimiter.SetLimit("POST:/api/v1/orders/*/shipments", 5, 1) // 1 shipment/second
	
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
		warehouseClient: warehouseClient,
		shipmentRepo: shipmentRepo,
		shipmentService: shipmentService,
		rateLimiter: rateLimiter,
		endpointRateLimiter: endpointRateLimiter,
		gracefulDegradation: gracefulDegradation,
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

	// Stop rate limiters
	s.rateLimiter.Stop()
    
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
	// Add graceful degradation middleware
	s.router.Use(s.gracefulDegradation.Middleware)
	// Add the global rate limiter middleware
	s.router.Use(s.rateLimiter.Middleware)
	// Add the endpoint rate limiter middleware
	s.router.Use(s.endpointRateLimiter.Middleware)
	
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
	admin.HandleFunc("/rate-limits", s.getRateLimitsHandler).Methods(http.MethodGet)
	admin.HandleFunc("/rate-limits/endpoint", s.setEndpointRateLimitHandler).Methods(http.MethodPost)
	admin.HandleFunc("/circuit-breaker", s.getCircuitBreakerStatusHandler).Methods(http.MethodGet)
	admin.HandleFunc("/circuit-breaker/reset", s.resetCircuitBreakerHandler).Methods(http.MethodPost)

	// Shipment endpoints
	api.HandleFunc("/orders/{id}/shipments", s.createShipmentHandler).Methods(http.MethodPost)
	api.HandleFunc("/orders/{id}/shipments", s.getShipmentsForOrderHandler).Methods(http.MethodGet)
	api.HandleFunc("/shipments/{id}", s.getShipmentHandler).Methods(http.MethodGet)
	api.HandleFunc("/shipments/{id}/sync", s.syncShipmentHandler).Methods(http.MethodPost)
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