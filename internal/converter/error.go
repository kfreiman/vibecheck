package converter

import (
	"fmt"
	"net/http"
)

// ConversionError represents a conversion failure with detailed error info
type ConversionError struct {
	OriginalError error
	Stderr        string
	Path          string
	Hint          string
}

func (e *ConversionError) Error() string {
	msg := fmt.Sprintf("markitdown conversion failed: %v", e.OriginalError)
	if e.Path != "" {
		msg += fmt.Sprintf(" (file: %s)", e.Path)
	}
	if e.Stderr != "" {
		// Truncate stderr if too long
		stderr := e.Stderr
		if len(stderr) > 500 {
			stderr = stderr[:500] + "..."
		}
		msg += fmt.Sprintf("\nstderr: %s", stderr)
	}
	if e.Hint != "" {
		msg += fmt.Sprintf("\nHint: %s", e.Hint)
	}
	return msg
}

func (e *ConversionError) Unwrap() error {
	return e.OriginalError
}

// FileNotFoundError represents a file not found error
type FileNotFoundError struct {
	Path string
}

func (e *FileNotFoundError) Error() string {
	return fmt.Sprintf("file not found: %s", e.Path)
}

// PathValidationError represents a path validation error
type PathValidationError struct {
	Path   string
	Reason string
}

func (e *PathValidationError) Error() string {
	return fmt.Sprintf("path validation failed for %s: %s", e.Path, e.Reason)
}

// BinaryNotFoundError represents a missing binary error
type BinaryNotFoundError struct{}

// HTTPError represents an HTTP error
type HTTPError struct {
	StatusCode int
	URL        string
}

func (e *HTTPError) Error() string {
	return http.StatusText(e.StatusCode)
}
