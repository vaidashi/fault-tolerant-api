package retry

import (
	"time"
	"context"
	"errors"
	"fmt"

	"github.com/vaidashi/fault-tolerant-api/pkg/logger"
)

// RetryableFunc defines a function that can be retried
type RetryableFunc func() error

// RetryConfig holds the configuration for retrying operations
type RetryConfig struct {
	MaxAttempts int           
	BackoffStrategy BackoffStrategy
	Logger logger.Logger 
	RetryableErrors []error // List of errors to retry on
}

// Retry retries the given function according to the provided configuration
func Retry(ctx context.Context, fn RetryableFunc, cfg *RetryConfig) error {
	var lastErr error

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled by context: %w", ctx.Err())
		default:
			// Continue with retry
		}

		// Execute the function
		err := fn()

		if err == nil {
			// Success
			return nil
		}

		lastErr = err

		// Check if this is the last attempt
		if attempt == cfg.MaxAttempts {
			break
		}

		// Check if error is retryable
		if !isRetryable(err, cfg.RetryableErrors) {
			cfg.Logger.Warn("Non-retryable error encountered, giving up",
				"error", err,
				"attempt", attempt)
			return err
		}

		// Calculate backoff duration
		backoff := cfg.BackoffStrategy.NextBackoff(attempt)

		cfg.Logger.Info("Retrying after error",
			"error", err,
			"attempt", attempt,
			"maxAttempts", cfg.MaxAttempts,
			"backoff", backoff)

		// Wait for backoff period or context cancellation
		select {
		case <-time.After(backoff):
		// Continue with next attempt
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled by context during backoff: %w", ctx.Err())
		}
	}

	return fmt.Errorf("all %d retry attempts failed, last error: %w", cfg.MaxAttempts, lastErr)
}

// isRetryable checks if an error is retryable
func isRetryable(err error, retryableErrors []error) bool {
	// If no specific errors are defined, assume all errors are retryable
	if len(retryableErrors) == 0 {
		return true
	}

	// Check if the error is in the list of retryable errors
	for _, retryableErr := range retryableErrors {
		if errors.Is(err, retryableErr) {
			return true
		}
	}

	return false
}

// RetryWithDiscard retries a function and applies the discard policy if all retries fail
func RetryWithDiscard(ctx context.Context, fn RetryableFunc, cfg *RetryConfig, discardFn func(error) error) error {
	err := Retry(ctx, fn, cfg)

	if err != nil {
		cfg.Logger.Error("All retries failed, applying discard policy",
			"error", err,
			"maxAttempts", cfg.MaxAttempts)
		return discardFn(err)
	}
	return nil
}