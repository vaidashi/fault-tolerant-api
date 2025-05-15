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

	// Initialize services
	orderService := service.NewOrderService(orderRepo, outboxRepo, logger)

	// Initialize outbox processor
	processorConfig := &outbox.ProcessorConfig{
		PollingInterval: 5 * time.Second,
		BatchSize:       10,
		MaxRetries:      3,
	}
    outboxProcessor := outbox.NewProcessor(outboxRepo, logger, processorConfig)

	// Register message handlers
    loggingHandler := outbox.NewLoggingHandler(logger)
    outboxProcessor.RegisterHandler("order_created", loggingHandler)
    outboxProcessor.RegisterHandler("order_updated", loggingHandler)
    outboxProcessor.RegisterHandler("order_status_changed", loggingHandler)
	
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
	}
	
	server.setupRoutes()
	// Start the outbox processor
	outboxProcessor.Start()

	return server
}

// Start starts the HTTP server 
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop the outbox processor
	s.outboxProcessor.Stop()

	if err := s.db.Close(); err != nil {
		s.logger.Error("Failed to close database connection", "error", err)
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
	
	// Example resource endpoints
	api.HandleFunc("/orders", s.getOrdersHandler).Methods(http.MethodGet)
	api.HandleFunc("/orders", s.createOrderHandler).Methods(http.MethodPost)
	api.HandleFunc("/orders/{id}", s.getOrderByIDHandler).Methods(http.MethodGet)
	api.HandleFunc("/orders/{id}", s.updateOrderHandler).Methods(http.MethodPut)
	api.HandleFunc("/orders/{id}", s.deleteOrderHandler).Methods(http.MethodDelete)
	 api.HandleFunc("/orders/{id}/status", s.updateOrderStatusHandler).Methods(http.MethodPatch)
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