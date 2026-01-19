package ingest

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig holds configuration for retry behavior
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      bool
	Backoff     BackoffStrategy
}

// BackoffStrategy defines the backoff algorithm
type BackoffStrategy int

const (
	BackoffExponential BackoffStrategy = iota
	BackoffLinear
	BackoffFixed
)

// DefaultRetryConfig provides sensible defaults for retry behavior
var DefaultRetryConfig = RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   1 * time.Second,
	MaxDelay:    30 * time.Second,
	Jitter:      true,
	Backoff:     BackoffExponential,
}

// RetryableFunc is a function that can be retried
type RetryableFunc func(attempt int) error

// Retry executes a function with retry logic
func Retry(ctx context.Context, config RetryConfig, fn RetryableFunc) error {
	var lastErr error

	for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
		// Execute the function
		err := fn(attempt)
		if err == nil {
			return nil
		}

		lastErr = err

		// Check if error is retryable
		if !IsRetryable(err) {
			return err
		}

		// Don't wait after the last attempt
		if attempt >= config.MaxAttempts {
			break
		}

		// Calculate delay
		delay := calculateDelay(config, attempt)

		// Apply jitter if enabled
		if config.Jitter {
			delay = applyJitter(delay)
		}

		// Wait with context cancellation support
		select {
		case <-time.After(delay):
			// Continue to next attempt
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		}
	}

	return &RetryableError{
		Err:      lastErr,
		Attempts: config.MaxAttempts,
	}
}

// calculateDelay computes the delay between retry attempts
func calculateDelay(config RetryConfig, attempt int) time.Duration {
	var delay time.Duration

	switch config.Backoff {
	case BackoffExponential:
		// Exponential backoff: baseDelay * 2^(attempt-1)
		delay = config.BaseDelay * time.Duration(math.Pow(2, float64(attempt-1)))
	case BackoffLinear:
		// Linear backoff: baseDelay * attempt
		delay = config.BaseDelay * time.Duration(attempt)
	case BackoffFixed:
		// Fixed delay
		delay = config.BaseDelay
	default:
		delay = config.BaseDelay
	}

	// Cap at max delay
	if delay > config.MaxDelay {
		delay = config.MaxDelay
	}

	return delay
}

// applyJitter adds random jitter to the delay to prevent thundering herd
func applyJitter(delay time.Duration) time.Duration {
	// Add ±25% jitter
	jitter := (rand.Float64() - 0.5) * 0.5 // [-0.5, 0.5)
	return time.Duration(float64(delay) * (1 + jitter))
}

// RetryWithExponentialBackoff is a convenience function for exponential backoff
func RetryWithExponentialBackoff(ctx context.Context, maxAttempts int, baseDelay time.Duration, fn RetryableFunc) error {
	config := RetryConfig{
		MaxAttempts: maxAttempts,
		BaseDelay:   baseDelay,
		MaxDelay:    30 * time.Second,
		Jitter:      true,
		Backoff:     BackoffExponential,
	}
	return Retry(ctx, config, fn)
}

// RetryStorageOperation retries a storage operation with exponential backoff
func RetryStorageOperation(ctx context.Context, operation string, fn func() error) error {
	return RetryWithExponentialBackoff(ctx, 3, 1*time.Second, func(attempt int) error {
		err := fn()
		if err != nil {
			return &StorageError{
				Operation: fmt.Sprintf("%s (attempt %d)", operation, attempt),
				Err:       err,
			}
		}
		return nil
	})
}

// RetryConversionOperation retries a conversion operation with exponential backoff
func RetryConversionOperation(ctx context.Context, inputPath string, fn func() error) error {
	return RetryWithExponentialBackoff(ctx, 2, 500*time.Millisecond, func(attempt int) error {
		err := fn()
		if err != nil {
			// Wrap the error with context
			if convErr, ok := err.(*ConversionError); ok {
				convErr.Hint = fmt.Sprintf("%s (attempt %d)", convErr.Hint, attempt)
				return convErr
			}
			return &ConversionError{
				InputPath: inputPath,
				Err:       err,
				Hint:      fmt.Sprintf("conversion failed on attempt %d", attempt),
			}
		}
		return nil
	})
}
