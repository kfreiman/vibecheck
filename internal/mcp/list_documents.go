package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/kfreiman/vibecheck/internal/storage"
)

// ListDocumentsTool handles listing all stored documents
type ListDocumentsTool struct {
	storageManager *storage.StorageManager
}

// NewListDocumentsTool creates a new list documents tool
func NewListDocumentsTool(storageManager *storage.StorageManager) *ListDocumentsTool {
	return &ListDocumentsTool{
		storageManager: storageManager,
	}
}

// Call implements the MCP tool interface
func (t *ListDocumentsTool) Call(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse arguments (optional filter for document type)
	var args struct {
		Type string `json:"type"` // Optional: "cv", "jd", or empty for all
	}

	if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid input format: %w", err)
	}

	// Get all documents
	cvUUIDs, jdUUIDs, err := t.storageManager.ListAllDocuments()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error listing documents: %v", err)},
			},
		}, err
	}

	// Filter by type if specified
	var resultCVs []string
	var resultJDs []string

	switch args.Type {
	case "cv":
		resultCVs = cvUUIDs
	case "jd":
		resultJDs = jdUUIDs
	case "":
		resultCVs = cvUUIDs
		resultJDs = jdUUIDs
	default:
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: invalid type '%s'. Use 'cv', 'jd', or leave empty for all documents", args.Type)},
			},
		}, fmt.Errorf("invalid document type")
	}

	// Build response
	var responseText string

	if len(resultCVs) == 0 && len(resultJDs) == 0 {
		responseText = "No documents found in storage."
	} else {
		responseText = "Stored Documents:\n\n"

		if len(resultCVs) > 0 {
			responseText += fmt.Sprintf("CV Documents (%d):\n", len(resultCVs))
			for _, uuid := range resultCVs {
				responseText += fmt.Sprintf("- cv://%s\n", uuid)
			}
			responseText += "\n"
		}

		if len(resultJDs) > 0 {
			responseText += fmt.Sprintf("Job Descriptions (%d):\n", len(resultJDs))
			for _, uuid := range jdUUIDs {
				responseText += fmt.Sprintf("- jd://%s\n", uuid)
			}
		}
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: responseText},
		},
	}, nil
}

// ListDocumentsHandler is a handler function for the list_documents tool
func ListDocumentsHandler(ctx context.Context, storageManager *storage.StorageManager) (*mcp.CallToolResult, error) {
	tool := NewListDocumentsTool(storageManager)
	return tool.Call(ctx, &mcp.CallToolRequest{})
}
