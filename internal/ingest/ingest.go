package ingest

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/kfreiman/vibecheck/internal/converter"
	"github.com/kfreiman/vibecheck/internal/storage"
)

// Ingestor defines the interface for document ingestion
type Ingestor interface {
	// Ingest ingests a document from the given path and returns the URI
	// Returns: URI (e.g., "cv://<uuid>" or "jd://<uuid>")
	Ingest(ctx context.Context, path string, docType string) (string, error)
}

// DocumentIngestor implements the Ingestor interface
type DocumentIngestor struct {
	storageManager    *storage.StorageManager
	documentConverter converter.DocumentConverter
	logger            *slog.Logger
}

// NewIngestor creates a new document ingestor
func NewIngestor(storageManager *storage.StorageManager, documentConverter converter.DocumentConverter) *DocumentIngestor {
	return &DocumentIngestor{
		storageManager:    storageManager,
		documentConverter: documentConverter,
		logger:            slog.Default(),
	}
}

// WithLogger sets a custom logger for the ingestor
func (i *DocumentIngestor) WithLogger(logger *slog.Logger) *DocumentIngestor {
	i.logger = logger
	return i
}

// Ingest implements the Ingestor interface
func (i *DocumentIngestor) Ingest(ctx context.Context, path string, docType string) (string, error) {
	// Validate type
	if docType != "cv" && docType != "jd" {
		return "", &ValidationError{
			Field:  "type",
			Value:  docType,
			Reason: "must be 'cv' or 'jd'",
		}
	}

	// Validate path to prevent security issues
	if err := validatePath(path); err != nil {
		return "", err
	}

	// Determine storage type
	var storageType storage.DocumentType
	if docType == "cv" {
		storageType = storage.DocumentTypeCV
	} else {
		storageType = storage.DocumentTypeJD
	}

	// Extract filename for original name
	originalFilename := extractFilename(path)
	if originalFilename == "" {
		originalFilename = "document.md"
	}

	// Get markdown content with retry
	markdownContent, err := i.getMarkdownContentWithRetry(ctx, path)
	if err != nil {
		return "", err
	}

	// Save to storage with retry
	uri, err := i.saveDocumentWithRetry(storageType, []byte(markdownContent), originalFilename)
	if err != nil {
		return "", err
	}

	return uri, nil
}

// getMarkdownContentWithRetry retrieves and converts document content with retry logic
func (i *DocumentIngestor) getMarkdownContentWithRetry(ctx context.Context, path string) (string, error) {
	// Parse input to determine type
	inputInfo := converter.ParseInput(path)

	var markdownContent string
	var err error

	switch inputInfo.Type {
	case converter.InputTypeURL, converter.InputTypeFile:
		// Try conversion first with retry
		if i.documentConverter != nil && i.documentConverter.Supports(path) {
			err = RetryConversionOperation(ctx, path, func() error {
				markdownContent, err = i.documentConverter.Convert(ctx, path)
				return err
			})

			if err != nil {
				// Conversion failed - try fallback to reading as markdown
				markdownContent, err = i.readLocalFileWithFallback(path)
				if err != nil {
					return "", &DegradedError{
						Component: "converter",
						Err:       err,
						Fallback:  "direct file read also failed",
					}
				}
				// Successfully degraded to fallback
				return markdownContent, &DegradedError{
					Component: "converter",
					Err:       fmt.Errorf("primary conversion failed"),
					Fallback:  "read as markdown",
				}
			}
		} else {
			// No converter available, try direct read with retry
			markdownContent, err = i.readLocalFileWithFallback(path)
			if err != nil {
				return "", &ConversionError{
					InputPath: path,
					Err:       err,
					Hint:      "no converter available and direct read failed",
				}
			}
		}

	case converter.InputTypeText:
		// Raw text - no conversion needed
		markdownContent = path

	default:
		return "", &ValidationError{
			Field:  "input",
			Value:  path,
			Reason: "unable to process input type",
		}
	}

	return markdownContent, nil
}

// readLocalFileWithFallback reads a local file with retry and fallback logic
func (i *DocumentIngestor) readLocalFileWithFallback(path string) (string, error) {
	var content string

	// Retry reading the file
	err := RetryWithExponentialBackoff(context.Background(), 3, 500*time.Millisecond, func(attempt int) error {
		data, err := os.ReadFile(path)
		if err != nil {
			return &StorageError{
				Operation: fmt.Sprintf("read file (attempt %d)", attempt),
				Path:      path,
				Err:       err,
			}
		}
		content = string(data)
		return nil
	})

	if err != nil {
		return "", err
	}

	return content, nil
}

// saveDocumentWithRetry saves a document to storage with retry logic
func (i *DocumentIngestor) saveDocumentWithRetry(docType storage.DocumentType, content []byte, filename string) (string, error) {
	var uri string
	var saveErr error

	err := RetryStorageOperation(context.Background(), "save document", func() error {
		var err error
		uri, err = i.storageManager.SaveDocument(docType, content, filename)
		if err != nil {
			saveErr = err
			return err
		}
		return nil
	})

	if err != nil {
		return "", &StorageError{
			Operation: "final save attempt",
			Err:       saveErr,
		}
	}

	return uri, nil
}

// extractFilename extracts the filename from a path or URL
func extractFilename(path string) string {
	// Handle URLs
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		// Try to extract from URL
		parts := strings.Split(path, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
		return "downloaded_file"
	}

	// Handle local paths
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return path
}

// validatePath validates a file path to prevent path traversal
func validatePath(path string) error {
	// Check for path traversal attempts
	if strings.Contains(path, "..") {
		return &SecurityError{
			Type:    "path_traversal",
			Details: fmt.Sprintf("path contains traversal sequence: %s", path),
		}
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return &SecurityError{
			Type:    "null_byte",
			Details: "path contains null bytes",
		}
	}

	return nil
}
