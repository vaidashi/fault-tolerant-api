package api 

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/vaidashi/fault-tolerant-api/internal/config"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

type Server struct {
	config *config.Config
	logger logger.Logger
	router *mux.Router
	httpServer *http.Server
}

// NewServer creates a new API server with the given configuration and logger.
func NewServer(cfg *config.Config, l logger.Logger) *Server {
	r := mux.NewRouter()

	server := &Server{
		router: r,
		httpServer: &http.Server{
			Addr: fmt.Sprintf(":%d", cfg.Port),
			Handler: r,
			ReadTimeout: 15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout: 60 * time.Second,
		},
		logger: l,
		config: cfg,
	}

	server.setupRoutes()
	return server
}

// Start starts the HTTP server 
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
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