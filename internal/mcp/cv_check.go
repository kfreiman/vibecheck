package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// Prompt parameter names - content-based
	CVContentParam  = "cv_content"
	JobContentParam = "job_content"
	// Prompt parameter names - URL-based
	CVURLParam  = "cv_url"
	JobURLParam = "job_url"
	// Prompt parameter names - filepath-based
	CVFilepathParam  = "cv_filepath"
	JobFilepathParam = "job_filepath"
)

// CVCheckTool handles the cv_check tool implementation
type CVCheckTool struct {
	converter DocumentConverter
}

// NewCVCheckTool creates a new CVCheckTool with the default converter
func NewCVCheckTool() (*CVCheckTool, error) {
	converter, err := NewMarkitdownConverter()
	if err != nil {
		return nil, err
	}
	return &CVCheckTool{converter: converter}, nil
}

// NewCVCheckToolWithConverter creates a CVCheckTool with a custom converter (for testing)
func NewCVCheckToolWithConverter(converter DocumentConverter) *CVCheckTool {
	return &CVCheckTool{converter: converter}
}

// Call executes the cv_check tool
func (t *CVCheckTool) Call(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse the arguments
	var args struct {
		CV  string `json:"cv"`
		Job string `json:"job"`
	}

	if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid input format: %w", err)
	}

	if args.CV == "" || args.Job == "" {
		return nil, fmt.Errorf("both 'cv' and 'job' parameters are required")
	}

	// Validate and process inputs
	cvContent, jobContent, err := t.ProcessInputs(args.CV, args.Job)
	if err != nil {
		return nil, err
	}

	// Return the analysis prompt directly instead of using MCP sampling
	analysisPrompt := BuildAnalysisPrompt(cvContent, jobContent)

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: analysisPrompt},
		},
	}, nil
}

// BuildAnalysisPrompt creates a structured analysis prompt for the LLM
func BuildAnalysisPrompt(cvContent, jobContent string) string {
	return fmt.Sprintf(`You are an expert career advisor specializing in CV and job description analysis. Your task is to compare the candidate's CV against the job description and provide a comprehensive analysis.

**CV Content:**
%s

**Job Description:**
%s

**Analysis Requirements:**

Analyze the match between the CV and job description in these specific areas:

1. **Key Skills Match** (0-100%%):
   - Identify which required skills are present in the CV
   - Note any missing critical skills
   - Highlight transferable skills

2. **Experience Alignment** (0-100%%):
   - Compare years of experience required vs. demonstrated
   - Match between job level (senior/mid/junior) and CV experience
   - Relevant industry experience

3. **Missing Requirements**:
   - List specific requirements from job description not found in CV
   - Prioritize by importance (must-have vs. nice-to-have)

4. **Recommendations**:
   - Suggest specific CV improvements (quantify achievements, reframe experiences)
   - Identify areas to emphasize for this specific role
   - Recommend additional skills/experience to gain

5. **Overall Match Score** (0-100%%):
   - Based on all factors above
   - Provide rationale for the score

Format your response clearly with markdown headings and bullet points for readability. Be specific and actionable in your recommendations.

Detect natural language of the job description and use it as language to reply in your analysis.`, cvContent, jobContent)
}

// ProcessInputs handles validation and auto-detection of inputs
func (t *CVCheckTool) ProcessInputs(cvInput, jobInput string) (cvContent, jobContent string, err error) {
	// Try to read as file paths first
	cvIsFile := t.isFilePath(cvInput)
	jobIsFile := t.isFilePath(jobInput)

	if cvIsFile {
		cvContent, err = t.readFileContent(cvInput)
		if err != nil {
			return "", "", fmt.Errorf("failed to read CV file: %w", err)
		}
	} else {
		cvContent = cvInput
	}

	if jobIsFile {
		jobContent, err = t.readFileContent(jobInput)
		if err != nil {
			return "", "", fmt.Errorf("failed to read job file: %w", err)
		}
	} else {
		jobContent = jobInput
	}

	// Validate content is not empty
	if strings.TrimSpace(cvContent) == "" {
		return "", "", fmt.Errorf("CV content is empty")
	}
	if strings.TrimSpace(jobContent) == "" {
		return "", "", fmt.Errorf("job content is empty")
	}

	return cvContent, jobContent, nil
}

// isFilePath checks if input is likely a file path
func (t *CVCheckTool) isFilePath(path string) bool {
	// Simple heuristic: check for common file path patterns
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "../") {
		return true
	}
	if strings.Contains(path, string(filepath.Separator)) {
		return true
	}
	if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".txt") {
		return true
	}
	return false
}

// readFileContent reads content from a file path
func (t *CVCheckTool) readFileContent(path string) (string, error) {
	content, err := ReadCVFile(path)
	if err != nil {
		return "", err
	}
	return content, nil
}

// CVCheckHandler implements the tool handler interface for mcp
func CVCheckHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tool := &CVCheckTool{}
	return tool.Call(ctx, req)
}

// CVPromptHandler implements the prompt handler interface for mcp
func CVPromptHandler(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	// Create converter for document conversion
	converter, err := NewMarkitdownConverter()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize document converter: %w", err)
	}

	// Parse arguments
	params := req.Params

	// Get CV content from any of the available input methods
	cvContent, err := resolvePromptInput(ctx, converter, params.Arguments, CVContentParam, CVURLParam, CVFilepathParam)
	if err != nil {
		return nil, fmt.Errorf("failed to get CV content: %w", err)
	}

	// Get job content from any of the available input methods
	jobContent, err := resolvePromptInput(ctx, converter, params.Arguments, JobContentParam, JobURLParam, JobFilepathParam)
	if err != nil {
		return nil, fmt.Errorf("failed to get job content: %w", err)
	}

	// Build the analysis prompt
	analysisPrompt := BuildAnalysisPrompt(cvContent, jobContent)

	// Return the prompt result
	return &mcp.GetPromptResult{
		Description: "CV vs Job Description Analysis Prompt",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: analysisPrompt,
				},
			},
		},
	}, nil
}

// resolvePromptInput extracts content from prompt arguments using multiple input methods
func resolvePromptInput(ctx context.Context, converter DocumentConverter, args map[string]string, contentParam, urlParam, filepathParam string) (string, error) {
	// Priority: content > filepath > URL

	// Try content first
	if content := args[contentParam]; content != "" {
		return content, nil
	}

	// Try filepath
	if filepath := args[filepathParam]; filepath != "" {
		return converter.Convert(ctx, filepath)
	}

	// Try URL
	if url := args[urlParam]; url != "" {
		return converter.Convert(ctx, url)
	}

	return "", fmt.Errorf("at least one of '%s', '%s', or '%s' must be provided", contentParam, urlParam, filepathParam)
}
