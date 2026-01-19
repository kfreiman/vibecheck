package ingest

import (
	"fmt"
	"net/http"
)

// StorageError represents a storage-related failure
type StorageError struct {
	Operation string
	Path      string
	Err       error
}

func (e *StorageError) Error() string {
	msg := fmt.Sprintf("storage error during %s", e.Operation)
	if e.Path != "" {
		msg += fmt.Sprintf(" (path: %s)", e.Path)
	}
	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}
	return msg
}

func (e *StorageError) Unwrap() error {
	return e.Err
}

// ConversionError represents a document conversion failure
type ConversionError struct {
	InputPath string
	Format    string
	Err       error
	Hint      string
}

func (e *ConversionError) Error() string {
	msg := fmt.Sprintf("conversion failed for %s", e.InputPath)
	if e.Format != "" {
		msg += fmt.Sprintf(" (format: %s)", e.Format)
	}
	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}
	if e.Hint != "" {
		msg += fmt.Sprintf("\nHint: %s", e.Hint)
	}
	return msg
}

func (e *ConversionError) Unwrap() error {
	return e.Err
}

// ValidationError represents input validation failure
type ValidationError struct {
	Field  string
	Value  string
	Reason string
}

func (e *ValidationError) Error() string {
	if e.Field != "" && e.Value != "" {
		return fmt.Sprintf("validation failed for %s '%s': %s", e.Field, e.Value, e.Reason)
	}
	if e.Field != "" {
		return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Reason)
	}
	return fmt.Sprintf("validation failed: %s", e.Reason)
}

// SecurityError represents a security violation
type SecurityError struct {
	Type    string // e.g., "path_traversal", "null_byte", "unauthorized"
	Details string
}

func (e *SecurityError) Error() string {
	return fmt.Sprintf("security violation (%s): %s", e.Type, e.Details)
}

// NetworkError represents a network-related failure
type NetworkError struct {
	URL    string
	Status int
	Err    error
}

func (e *NetworkError) Error() string {
	msg := "network error"
	if e.URL != "" {
		msg += fmt.Sprintf(" accessing %s", e.URL)
	}
	if e.Status != 0 {
		msg += fmt.Sprintf(" (status: %d %s)", e.Status, http.StatusText(e.Status))
	}
	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}
	return msg
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// RetryableError indicates an error that can be retried
type RetryableError struct {
	Err       error
	Attempts  int
	LastError error
}

func (e *RetryableError) Error() string {
	msg := fmt.Sprintf("retryable error after %d attempts", e.Attempts)
	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}
	if e.LastError != nil {
		msg += fmt.Sprintf(" (last error: %v)", e.LastError)
	}
	return msg
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// DegradedError represents a non-critical error that allows graceful degradation
type DegradedError struct {
	Component string
	Err       error
	Fallback  string
}

func (e *DegradedError) Error() string {
	msg := fmt.Sprintf("degraded operation in %s", e.Component)
	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}
	if e.Fallback != "" {
		msg += fmt.Sprintf(" (using fallback: %s)", e.Fallback)
	}
	return msg
}

func (e *DegradedError) Unwrap() error {
	return e.Err
}

// IsRetryable checks if an error is retryable
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}
	// Check for specific retryable error types
	switch err := err.(type) {
	case *NetworkError:
		ne := err
		// Retry on 5xx errors or timeout
		return ne.Status >= 500 || ne.Status == 0
	case *StorageError:
		// Retry on storage errors (could be temporary I/O issues)
		return true
	case *ConversionError:
		// Don't retry conversion errors (usually deterministic)
		return false
	case *DegradedError:
		// Degraded errors are not retryable (they already succeeded with fallback)
		return false
	default:
		// Check wrapped errors
		if err, ok := err.(interface{ Unwrap() error }); ok {
			return IsRetryable(err.Unwrap())
		}
		return false
	}
}

// IsDegraded checks if an error allows graceful degradation
func IsDegraded(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*DegradedError)
	return ok
}

// IsTransient checks if an error is likely transient
func IsTransient(err error) bool {
	if err == nil {
		return false
	}

	// Check for specific transient error types
	var netErr *NetworkError
	if as(err, &netErr) {
		// 5xx errors are typically transient
		return netErr.Status >= 500 || netErr.Status == 0
	}

	var storageErr *StorageError
	if as(err, &storageErr) {
		// Storage errors are often transient (I/O issues, locks)
		return true
	}

	// Check wrapped errors
	if err, ok := err.(interface{ Unwrap() error }); ok {
		return IsTransient(err.Unwrap())
	}

	return false
}

// as is a helper function to check if err can be type asserted to target
func as(err error, target interface{}) bool {
	if err == nil {
		return false
	}
	// Simple implementation - in production you'd use errors.As
	// but we're avoiding the standard library errors package for now
	switch e := err.(type) {
	case *NetworkError:
		if ptr, ok := target.(**NetworkError); ok {
			*ptr = e
			return true
		}
	case *StorageError:
		if ptr, ok := target.(**StorageError); ok {
			*ptr = e
			return true
		}
	}
	// Check wrapped errors
	if err, ok := err.(interface{ Unwrap() error }); ok {
		return as(err.Unwrap(), target)
	}
	return false
}
