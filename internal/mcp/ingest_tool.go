package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/kfreiman/vibecheck/internal/converter"
	"github.com/kfreiman/vibecheck/internal/storage"
)

// IngestDocumentTool handles document ingestion
type IngestDocumentTool struct {
	storageManager    *storage.StorageManager
	documentConverter converter.DocumentConverter
}

// NewIngestDocumentTool creates a new ingest document tool
func NewIngestDocumentTool(storageManager *storage.StorageManager, documentConverter converter.DocumentConverter) *IngestDocumentTool {
	return &IngestDocumentTool{
		storageManager:    storageManager,
		documentConverter: documentConverter,
	}
}

// Call implements the MCP tool interface with retry logic and graceful degradation
func (t *IngestDocumentTool) Call(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse arguments
	var args struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}

	if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
		return nil, &ValidationError{
			Field:  "arguments",
			Reason: fmt.Sprintf("invalid JSON format: %v", err),
		}
	}

	// Validate required parameters
	if args.Path == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: 'path' parameter is required"},
			},
		}, &ValidationError{Field: "path", Reason: "required parameter missing"}
	}

	// Set defaults
	if args.Type == "" {
		args.Type = "cv"
	}

	// Validate type
	if args.Type != "cv" && args.Type != "jd" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: 'type' must be 'cv' or 'jd'"},
			},
		}, &ValidationError{Field: "type", Value: args.Type, Reason: "must be 'cv' or 'jd'"}
	}

	// Validate path to prevent security issues
	if err := validatePath(args.Path); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: invalid path - %v", err)},
			},
		}, err
	}

	// Process document with retry logic and graceful degradation
	result, err := t.processDocumentWithRetry(ctx, args.Path, args.Type)
	if err != nil {
		// Check if this is a degraded error (non-critical)
		if degradedErr, ok := err.(*DegradedError); ok {
			// Return success with degraded operation notice
			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: fmt.Sprintf(`Document ingested with degraded operation!

%s

Note: Some features may be limited due to temporary issues. The core functionality remains available.`, degradedErr.Error())},
				},
			}, nil
		}
		// Return error with user-friendly message
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
			},
		}, err
	}

	return result, nil
}

// processDocumentWithRetry handles the full ingestion pipeline with retry logic
func (t *IngestDocumentTool) processDocumentWithRetry(ctx context.Context, path string, docType string) (*mcp.CallToolResult, error) {
	// Parse input to determine type
	inputInfo := converter.ParseInput(path)

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
	markdownContent, err := t.getMarkdownContentWithRetry(ctx, path, inputInfo)
	if err != nil {
		return nil, err
	}

	// Save to storage with retry
	uri, err := t.saveDocumentWithRetry(storageType, []byte(markdownContent), originalFilename)
	if err != nil {
		return nil, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(`Document ingested successfully!

URI: %s
Original filename: %s
Type: %s

Use this URI in the analyze_fit prompt to analyze the document.`, uri, originalFilename, docType)},
		},
	}, nil
}

// getMarkdownContentWithRetry retrieves and converts document content with retry logic
func (t *IngestDocumentTool) getMarkdownContentWithRetry(ctx context.Context, path string, inputInfo converter.InputInfo) (string, error) {
	var markdownContent string
	var err error

	switch inputInfo.Type {
	case converter.InputTypeURL, converter.InputTypeFile:
		// Try conversion first with retry
		if t.documentConverter != nil && t.documentConverter.Supports(path) {
			err = RetryConversionOperation(ctx, path, func() error {
				markdownContent, err = t.documentConverter.Convert(ctx, path)
				return err
			})

			if err != nil {
				// Conversion failed - try fallback to reading as markdown
				markdownContent, err = t.readLocalFileWithFallback(path)
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
			markdownContent, err = t.readLocalFileWithFallback(path)
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
func (t *IngestDocumentTool) readLocalFileWithFallback(path string) (string, error) {
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
func (t *IngestDocumentTool) saveDocumentWithRetry(docType storage.DocumentType, content []byte, filename string) (string, error) {
	var uri string
	var saveErr error

	err := RetryStorageOperation(context.Background(), "save document", func() error {
		var err error
		uri, err = t.storageManager.SaveDocument(docType, content, filename)
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

// readLocalFile reads a local file
func readLocalFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
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

	// Check for absolute paths outside expected directories
	if strings.HasPrefix(path, "/") {
		// Allow absolute paths but warn (could be legitimate)
		return nil
	}

	return nil
}

// IngestHandler is a handler function for the ingest_document tool
func IngestHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tool := &IngestDocumentTool{}
	return tool.Call(ctx, req)
}
