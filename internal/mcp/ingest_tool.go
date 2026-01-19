package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/kfreiman/vibecheck/internal/ingest"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// IngestDocumentTool handles document ingestion
type IngestDocumentTool struct {
	ingestor ingest.Ingestor
	logger   *slog.Logger
}

// NewIngestDocumentTool creates a new ingest document tool
func NewIngestDocumentTool(ingestor ingest.Ingestor) *IngestDocumentTool {
	return &IngestDocumentTool{
		ingestor: ingestor,
		logger:   slog.Default(),
	}
}

// WithLogger sets the logger for the tool
func (t *IngestDocumentTool) WithLogger(logger *slog.Logger) *IngestDocumentTool {
	t.logger = logger
	return t
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

	// Ingest the document
	uri, err := t.ingestor.Ingest(ctx, args.Path, args.Type)
	if err != nil {
		// Check if this is a degraded error (non-critical)
		if degradedErr, ok := err.(*ingest.DegradedError); ok {
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

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(`Document ingested successfully!

URI: %s
Original filename: %s

Use this URI in the analyze_fit prompt to analyze the document.`, uri, args.Type)},
		},
	}, nil
}
