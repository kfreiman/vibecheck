package mcp

import (
	"context"
	"time"

	"github.com/kfreiman/vibecheck/internal/ingest"
)

// RetryConfig is re-exported from ingest package for backward compatibility
type RetryConfig = ingest.RetryConfig

// BackoffStrategy is re-exported from ingest package for backward compatibility
type BackoffStrategy = ingest.BackoffStrategy

// DefaultRetryConfig is re-exported from ingest package for backward compatibility
var DefaultRetryConfig = ingest.DefaultRetryConfig

// RetryableFunc is re-exported from ingest package for backward compatibility
type RetryableFunc = ingest.RetryableFunc

// Retry is re-exported from ingest package for backward compatibility
func Retry(ctx context.Context, config RetryConfig, fn RetryableFunc) error {
	return ingest.Retry(ctx, config, fn)
}

// RetryWithExponentialBackoff is re-exported from ingest package for backward compatibility
func RetryWithExponentialBackoff(ctx context.Context, maxAttempts int, baseDelay time.Duration, fn RetryableFunc) error {
	return ingest.RetryWithExponentialBackoff(ctx, maxAttempts, baseDelay, fn)
}

// RetryStorageOperation is re-exported from ingest package for backward compatibility
func RetryStorageOperation(ctx context.Context, operation string, fn func() error) error {
	return ingest.RetryStorageOperation(ctx, operation, fn)
}

// RetryConversionOperation is re-exported from ingest package for backward compatibility
func RetryConversionOperation(ctx context.Context, inputPath string, fn func() error) error {
	return ingest.RetryConversionOperation(ctx, inputPath, fn)
}

// IsTransient is re-exported from ingest package for backward compatibility
func IsTransient(err error) bool {
	return ingest.IsTransient(err)
}
