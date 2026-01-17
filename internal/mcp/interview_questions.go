package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/kfreiman/vibecheck/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// InterviewQuestionsTool generates interview questions based on CV/JD gap analysis
type InterviewQuestionsTool struct {
	storageManager *storage.StorageManager
}

// NewInterviewQuestionsTool creates a new interview questions tool
func NewInterviewQuestionsTool(storageManager *storage.StorageManager) *InterviewQuestionsTool {
	return &InterviewQuestionsTool{
		storageManager: storageManager,
	}
}

// Call implements the MCP tool interface
func (t *InterviewQuestionsTool) Call(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse arguments
	var args struct {
		CVURI string `json:"cv_uri"` // URI of ingested CV (cv://[uuid])
		JDURI string `json:"jd_uri"` // URI of ingested job description (jd://[uuid])
		Style string `json:"style"`  // Optional: "technical", "behavioral", or "comprehensive" (default)
		Count int    `json:"count"`  // Optional: number of questions (default: 5)
	}

	if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid input format: %w", err)
	}

	// Validate required parameters
	if args.CVURI == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: cv_uri parameter is required"},
			},
		}, fmt.Errorf("cv_uri is required")
	}

	if args.JDURI == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: jd_uri parameter is required"},
			},
		}, fmt.Errorf("jd_uri is required")
	}

	// Validate URIs
	cvDocType, _, err := storage.ParseURI(args.CVURI)
	if err != nil || cvDocType != storage.DocumentTypeCV {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: invalid CV URI '%s'. Must be cv:// format", args.CVURI)},
			},
		}, fmt.Errorf("invalid CV URI")
	}

	jdDocType, _, err := storage.ParseURI(args.JDURI)
	if err != nil || jdDocType != storage.DocumentTypeJD {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: invalid JD URI '%s'. Must be jd:// format", args.JDURI)},
			},
		}, fmt.Errorf("invalid JD URI")
	}

	// Check if documents exist
	if !t.storageManager.DocumentExists(args.CVURI) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: CV document not found: %s", args.CVURI)},
			},
		}, fmt.Errorf("CV document not found")
	}

	if !t.storageManager.DocumentExists(args.JDURI) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: Job description not found: %s", args.JDURI)},
			},
		}, fmt.Errorf("job description not found")
	}

	// Set defaults
	if args.Style == "" {
		args.Style = "comprehensive"
	}
	if args.Count <= 0 {
		args.Count = 5
	}

	// Validate style
	validStyles := map[string]bool{"technical": true, "behavioral": true, "comprehensive": true}
	if !validStyles[args.Style] {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: invalid style '%s'. Must be 'technical', 'behavioral', or 'comprehensive'", args.Style)},
			},
		}, fmt.Errorf("invalid style")
	}

	// Generate the prompt using the prompt template
	prompt := BuildInterviewQuestionsPrompt(args.CVURI, args.JDURI, InterviewQuestionStyle(args.Style), args.Count)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: prompt},
		},
	}, nil
}

// GenerateInterviewQuestionsHandler is a handler function for the generate_interview_questions tool
func GenerateInterviewQuestionsHandler(ctx context.Context, storageManager *storage.StorageManager) func(*mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tool := NewInterviewQuestionsTool(storageManager)
		return tool.Call(ctx, req)
	}
}
