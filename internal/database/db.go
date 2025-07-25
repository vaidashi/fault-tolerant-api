package database

import (
	"context"
	"fmt"
	"time"


	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq" // PostgreSQL driver
	"github.com/vaidashi/fault-tolerant-api/internal/config"
	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

// Database represents a database connection
type Database struct {
	DB    *sqlx.DB
	logger logger.Logger
}

// New creates a new database connection
func New(cfg *config.Config, logger logger.Logger) (*Database, error) {
	db, err := sqlx.Connect("postgres", cfg.GetDBConnString())

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	logger.Info("Connected to database", "host", cfg.DB.Host, "database", cfg.DB.Name)

	return &Database{
		DB:     db,
		logger: logger,
	}, nil
}

// Ping checks the database connection
func (d *Database) Ping(ctx context.Context) error {
	return d.DB.PingContext(ctx)
}

// Close closes the database connection
func (d *Database) Close() error {
	return d.DB.Close()
}

// RunMigrations runs database migrations
func (d *Database) RunMigrations() error {
	// For initial setup, just create tables directly
	// In a real project, you'd want to use a migration tool
	schema := `
	CREATE TABLE IF NOT EXISTS orders (
		id VARCHAR(50) PRIMARY KEY,
		customer_id VARCHAR(50) NOT NULL,
		amount DECIMAL(10, 2) NOT NULL,
		status VARCHAR(20) NOT NULL,
		description TEXT,
		created_at TIMESTAMP NOT NULL DEFAULT NOW(),
		updated_at TIMESTAMP NOT NULL DEFAULT NOW()
	);

	CREATE INDEX IF NOT EXISTS idx_orders_customer_id ON orders(customer_id);
	CREATE INDEX IF NOT EXISTS idx_orders_status ON orders(status);

	-- Outbox table for message publishing
    CREATE TABLE IF NOT EXISTS outbox_messages (
        id SERIAL PRIMARY KEY,
        aggregate_type VARCHAR(50) NOT NULL,
        aggregate_id VARCHAR(50) NOT NULL,
        event_type VARCHAR(50) NOT NULL, 
        payload JSONB NOT NULL,
        created_at TIMESTAMP NOT NULL DEFAULT NOW(),
        processed_at TIMESTAMP,
        processing_attempts INT NOT NULL DEFAULT 0,
        last_error TEXT,
        status VARCHAR(20) NOT NULL DEFAULT 'pending'
    );

    CREATE INDEX IF NOT EXISTS idx_outbox_status ON outbox_messages(status);
    CREATE INDEX IF NOT EXISTS idx_outbox_aggregate ON outbox_messages(aggregate_type, aggregate_id);

	-- Dead letter queue for failed messages
    CREATE TABLE IF NOT EXISTS dead_letter_messages (
        id SERIAL PRIMARY KEY,
        original_message_id BIGINT NOT NULL,
        aggregate_type VARCHAR(50) NOT NULL,
        aggregate_id VARCHAR(50) NOT NULL,
        event_type VARCHAR(50) NOT NULL,
        payload JSONB NOT NULL,
        error_message TEXT NOT NULL,
        failure_reason VARCHAR(100) NOT NULL,
        retry_count INT NOT NULL DEFAULT 0,
        last_retry_at TIMESTAMP,
        status VARCHAR(20) NOT NULL DEFAULT 'pending',
        created_at TIMESTAMP NOT NULL DEFAULT NOW(),
        resolved_at TIMESTAMP
    );

    CREATE INDEX IF NOT EXISTS idx_dlq_status ON dead_letter_messages(status);
    CREATE INDEX IF NOT EXISTS idx_dlq_aggregate ON dead_letter_messages(aggregate_type, aggregate_id);

	-- Shipments table for tracking order shipments
    CREATE TABLE IF NOT EXISTS shipments (
        id VARCHAR(50) PRIMARY KEY,
        order_id VARCHAR(50) NOT NULL,
        shipment_id VARCHAR(50) NOT NULL,
        tracking_number VARCHAR(50),
        status VARCHAR(20) NOT NULL,
        created_at TIMESTAMP NOT NULL DEFAULT NOW(),
        updated_at TIMESTAMP NOT NULL DEFAULT NOW()
    );

    CREATE INDEX IF NOT EXISTS idx_shipments_order_id ON shipments(order_id);
    CREATE INDEX IF NOT EXISTS idx_shipments_status ON shipments(status);
	`

	_, err := d.DB.Exec(schema)
	
	if err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	d.logger.Info("Database migrations completed successfully")
	return nil
}