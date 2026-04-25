package sync

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"time"
)

// RetryConfig configures retry behavior for failed operations.
type RetryConfig struct {
	// MaxRetries is the maximum number of retry attempts.
	// 0 means no retries (fail on first error).
	MaxRetries int

	// InitialDelay is the delay before the first retry.
	// Default is 1 second.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	// Default is 30 seconds.
	MaxDelay time.Duration

	// Multiplier is the factor by which delay increases after each retry.
	// Default is 2.0 (exponential backoff).
	Multiplier float64

	// Jitter adds randomness to delays to prevent thundering herd.
	// 0.1 means +/- 10% random variation. Default is 0.1.
	Jitter float64

	// RetryableErrors is a function that determines if an error should be retried.
	// If nil, all errors are retried.
	RetryableErrors func(error) bool
}

// DefaultRetryConfig returns retry config with sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:   3,
		InitialDelay: time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
	}
}

// retryOperation retries an operation with exponential backoff.
func retryOperation(ctx context.Context, config RetryConfig, op func() error) error {
	if config.MaxRetries <= 0 {
		return op()
	}

	// Apply defaults
	if config.InitialDelay <= 0 {
		config.InitialDelay = time.Second
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 30 * time.Second
	}
	if config.Multiplier <= 0 {
		config.Multiplier = 2.0
	}

	var lastErr error
	delay := config.InitialDelay

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Try the operation
		err := op()
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if config.RetryableErrors != nil && !config.RetryableErrors(err) {
			return err
		}

		// Check if context is cancelled
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Don't delay after the last attempt
		if attempt == config.MaxRetries {
			break
		}

		// Apply jitter to delay.
		// Using math/rand is intentional - crypto/rand is unnecessary for timing jitter.
		// Jitter is used only to spread out retry timing to avoid thundering herd,
		// not for any security purpose.
		actualDelay := delay
		if config.Jitter > 0 {
			jitter := float64(delay) * config.Jitter
			actualDelay = delay + time.Duration((rand.Float64()*2-1)*jitter) //nolint:gosec // G404: math/rand is appropriate for timing jitter
		}

		// Wait before retry
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(actualDelay):
		}

		// Increase delay for next iteration
		delay = time.Duration(float64(delay) * config.Multiplier)
		if delay > config.MaxDelay {
			delay = config.MaxDelay
		}
	}

	return &RetryError{
		Attempts: config.MaxRetries + 1,
		LastErr:  lastErr,
	}
}

// RetryError indicates an operation failed after all retry attempts.
type RetryError struct {
	Attempts int
	LastErr  error
}

func (e *RetryError) Error() string {
	return fmt.Sprintf("operation failed after %d attempts: %v", e.Attempts, e.LastErr)
}

func (e *RetryError) Unwrap() error {
	return e.LastErr
}

// IsRetryError returns true if err is a RetryError.
func IsRetryError(err error) bool {
	var re *RetryError
	return errors.As(err, &re)
}

// IsTemporaryError returns true if err is likely temporary and worth retrying.
// This is a reasonable default for RetryConfig.RetryableErrors.
func IsTemporaryError(err error) bool {
	if err == nil {
		return false
	}

	// Check for temporary interface (many network errors implement this)
	var temp interface{ Temporary() bool }
	if errors.As(err, &temp) {
		return temp.Temporary()
	}

	// Check for timeout interface
	var timeout interface{ Timeout() bool }
	if errors.As(err, &timeout) {
		return timeout.Timeout()
	}

	return false
}
