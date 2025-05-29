package retry

import (
	"time"
	"math/rand"
	"math"
)

// BackoffStrategy defines the interface for backoff strategies
type BackoffStrategy interface {
	// NextBackoff reutrns the next backoff duration based on the attempt number
	NextBackoff(attempt int) time.Duration
}

// ConstantBackoff implements a backoff strategy with a constant delay
type ConstantBackoff struct {
	Interval time.Duration
}

// NextBackoff returns the constant backoff interval
func (b *ConstantBackoff) NextBackoff(attempt int) time.Duration {
	return b.Interval
}

// ExponentialBackoff implements an exponential backoff strategy with jitter
type ExponentialBackoff struct {
	InitialInterval time.Duration // Initial backoff interval
	MaxInterval     time.Duration // Maximum backoff interval
	Multiplier     float64        // Multiplier for exponential growth
	JitterFactor  float64        // Jitter factor to add randomness
}

// NextBackoff calculates the next exponentially increasing backoff duration with jitter
func (b *ExponentialBackoff) NextBackoff(attempt int) time.Duration {
	// Calculate backoff without jitter
	backoff := float64(b.InitialInterval) * math.Pow(b.Multiplier, float64(attempt-1))

	// Add jitter
	if b.JitterFactor > 0 {
		jitter := rand.Float64() * b.JitterFactor * backoff
		backoff += jitter
	}

	// Ensure backoff does not exceed max interval
	if backoff > float64(b.MaxInterval) {
		backoff = float64(b.MaxInterval)
	}
	// Return the backoff duration as time.Duration
	return time.Duration(backoff)
}

// LinearBackoff implements a linear backoff strategy
type LinearBackoff struct {
	InitialInterval time.Duration // Initial backoff interval
	MaxInterval     time.Duration // Maximum backoff interval
	Stop time.Duration // Stop after this duration
}

// NextBackoff calculates the next linear backoff duration
func (b *LinearBackoff) NextBackoff(attempt int) time.Duration {
	backoff := b.InitialInterval + (b.Stop * time.Duration(attempt-1))

	if backoff > b.MaxInterval {
		return b.MaxInterval
	}

	return backoff
}

// NewDefaultExponentialBackoff creates a default exponential backoff strategy
func NewDefaultExponentialBackoff() *ExponentialBackoff {
	return &ExponentialBackoff{
		InitialInterval: 500 * time.Millisecond, 
		MaxInterval:     60 * time.Second,       
		Multiplier:      1.5,                    // 1.5x the interval each time
		JitterFactor:    0.2,                     // 20% jitter
	}
}