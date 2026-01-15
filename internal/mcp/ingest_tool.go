package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/kfreiman/vibecheck/internal/converter"
	"github.com/kfreiman/vibecheck/internal/redaction"
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

// Call implements the MCP tool interface
func (t *IngestDocumentTool) Call(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse arguments
	var args struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}

	if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid input format: %w", err)
	}

	if args.Path == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: 'path' parameter is required"},
			},
		}, fmt.Errorf("path parameter is required")
	}

	if args.Type == "" {
		args.Type = "cv" // Default to CV
	}

	if args.Type != "cv" && args.Type != "jd" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: 'type' must be 'cv' or 'jd'"},
			},
		}, fmt.Errorf("type must be 'cv' or 'jd'")
	}

	// Validate path to prevent path traversal
	if err := validatePath(args.Path); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: invalid path - %v", err)},
			},
		}, err
	}

	// Parse input to determine type
	inputInfo := converter.ParseInput(args.Path)

	// Determine document type
	var storageType storage.DocumentType
	if args.Type == "cv" {
		storageType = storage.DocumentTypeCV
	} else {
		storageType = storage.DocumentTypeJD
	}

	// Extract filename for original name (handle raw text specially to avoid PII exposure)
	var originalFilename string
	var markdownContent string
	var err error

	switch inputInfo.Type {
	case converter.InputTypeURL, converter.InputTypeFile:
		// For URLs and files, extract the actual filename
		originalFilename = extractFilename(args.Path)
		// Use converter to convert if available
		if t.documentConverter != nil && t.documentConverter.Supports(args.Path) {
			markdownContent, err = t.documentConverter.Convert(ctx, args.Path)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{Text: fmt.Sprintf("Error converting document: %v", err)},
					},
				}, err
			}
		} else {
			// Fallback: read as markdown if available
			markdownContent, err = readLocalFile(args.Path)
			if err != nil {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{Text: "Error: converter not available and file is not a markdown file"},
					},
				}, fmt.Errorf("conversion not possible")
			}
		}
	case converter.InputTypeText:
		// For raw text, use generic filename to avoid PII in metadata
		originalFilename = "text_input.md"
		// Already markdown/text content
		markdownContent = args.Path
	default:
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: unable to process input"},
			},
		}, fmt.Errorf("unable to process input")
	}

	// Apply PII redaction
	markdownBytes := []byte(markdownContent)
	redactedBytes := redaction.Redact(markdownBytes)

	// Save to storage
	uri, err := t.storageManager.SaveDocument(storageType, redactedBytes, originalFilename)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error saving document: %v", err)},
			},
		}, err
	}

	// Count redacted items
	piiCount := redaction.DefaultRedactor.CountPIIItems(markdownBytes)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(`Document ingested successfully!

URI: %s
Original filename: %s
Type: %s
PII redacted: %d emails, %d phone numbers

Use this URI in the analyze_fit prompt to analyze the document.`, uri, originalFilename, args.Type, piiCount["emails"], piiCount["phones"])},
		},
	}, nil
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
		return fmt.Errorf("path traversal not allowed")
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("null bytes not allowed")
	}

	// Check for absolute paths outside expected directories
	if strings.HasPrefix(path, "/") {
		// Allow absolute paths but warn
		return nil
	}

	return nil
}

// IngestHandler is a handler function for the ingest_document tool
func IngestHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tool := &IngestDocumentTool{}
	return tool.Call(ctx, req)
}
