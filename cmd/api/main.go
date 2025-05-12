package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/vaidashi/fault-tolerant-api/internal/api"
	"github.com/vaidashi/fault-tolerant-api/internal/config"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

func main() {
	cfg, err := config.Load()

	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	l := logger.NewLogger(cfg.LogLevel)
	l.Info("Starting API server...")

	server := api.NewServer(cfg, l)

	// Start the server in a goroutine
	go func() {
		l.Info(fmt.Sprintf("Server is starting on port %d", cfg.Port))

		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			l.Error("Failed to start server", "error", err)
			os.Exit(1)
		}
	}()

	// Graceful shutdown via interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	l.Info("Shutting down server...")

	// Create a context with a timeout for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		l.Error("Server forced to shutdown:", "error", err)
	} else {
		l.Info("Server exiting")
	}
}