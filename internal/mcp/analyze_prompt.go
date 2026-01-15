package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/kfreiman/vibecheck/internal/storage"
)

// AnalyzeFitPrompt handles the analyze_fit prompt
type AnalyzeFitPrompt struct {
	storageManager *storage.StorageManager
}

// NewAnalyzeFitPrompt creates a new analyze fit prompt
func NewAnalyzeFitPrompt(storageManager *storage.StorageManager) *AnalyzeFitPrompt {
	return &AnalyzeFitPrompt{
		storageManager: storageManager,
	}
}

// Handle implements the prompt handler interface
func (p *AnalyzeFitPrompt) Handle(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	args := req.Params.Arguments

	cvURI := args["cv_uri"]
	jdURI := args["jd_uri"]

	if cvURI == "" {
		return nil, fmt.Errorf("cv_uri parameter is required")
	}
	if jdURI == "" {
		return nil, fmt.Errorf("jd_uri parameter is required")
	}

	// Validate URIs
	cvDocType, _, err := storage.ParseURI(cvURI)
	if err != nil || cvDocType != storage.DocumentTypeCV {
		return nil, fmt.Errorf("invalid CV URI: must be cv:// format")
	}

	jdDocType, _, err := storage.ParseURI(jdURI)
	if err != nil || jdDocType != storage.DocumentTypeJD {
		return nil, fmt.Errorf("invalid JD URI: must be jd:// format")
	}

	// Check if documents exist
	if !p.storageManager.DocumentExists(cvURI) {
		return nil, fmt.Errorf("CV document not found: %s", cvURI)
	}
	if !p.storageManager.DocumentExists(jdURI) {
		return nil, fmt.Errorf("Job description not found: %s", jdURI)
	}

	// Build the analysis prompt
	prompt := BuildAnalyzeFitPrompt(cvURI, jdURI)

	return &mcp.GetPromptResult{
		Description: "Analyze CV and Job Description fit",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: prompt,
				},
			},
		},
	}, nil
}

// AnalyzeFitHandler is a handler function for the analyze_fit prompt
func AnalyzeFitHandler(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	prompt := &AnalyzeFitPrompt{}
	return prompt.Handle(ctx, req)
}

// BuildAnalyzeFitPrompt creates a comprehensive analysis prompt
func BuildAnalyzeFitPrompt(cvURI, jdURI string) string {
	return fmt.Sprintf(`You are an expert career advisor and technical recruiter. Your task is to analyze the match between a candidate's CV and a job description, providing a structured assessment.

## Resources Available

Please use the following MCP resources to access the documents:
- CV: %s
- Job Description: %s

## Analysis Requirements

Provide your analysis in the following STRUCTURED format:

### 1. Match Percentage (0-100%%)
Provide an overall match score with a brief rationale.

### 2. Technical Gap Analysis
List specific skills/technologies from the job description that are:
- **Missing from CV**: Skills mentioned in the job but not found in the CV
- **Partial Match**: Skills mentioned but with limited experience demonstrated
- **Strong Match**: Skills well-demonstrated in the CV

### 3. Evidence-Based Questions
List 3-5 questions that the hiring manager should ask the candidate to clarify:
- Gaps in experience
- Technical claims made in the CV
- Potential concerns based on the job requirements

### 4. Key Strengths
Highlight the candidate's strongest qualifications for this role based on the CV.

### 5. Recommendations
Provide 2-3 specific suggestions for:
- How the candidate can improve their CV for this role
- Skills to highlight or reframe
- Experience to emphasize

## Important Notes

- Use the cv:// and jd:// resources to read the actual content
- Be specific and cite evidence from the documents
- Detect the natural language of the job description and respond in that language
- Focus on objective analysis rather than subjective opinions

## Response Format

Your response should be clear and well-organized with markdown formatting. Use headings for each section above.`, cvURI, jdURI)
}

// BuildQuickAnalysisPrompt creates a concise analysis prompt for quick reviews
func BuildQuickAnalysisPrompt(cvURI, jdURI string) string {
	return fmt.Sprintf(`Quick analysis: Compare CV (%s) against job description (%s).

Provide:
1. Match score (0-100%%)
2. Top 3 matching skills
3. Top 3 missing skills
4. One sentence recommendation

Use cv:// and jd:// resources.`, cvURI, jdURI)
}

// AnalysisOutput represents structured analysis results from LLM
type AnalysisOutput struct {
	MatchPercentage    int      `json:"match_percentage"`
	TechnicalGaps      []string `json:"technical_gaps"`
	EvidenceQuestions  []string `json:"evidence_questions"`
	KeyStrengths       []string `json:"key_strengths"`
	Recommendations    []string `json:"recommendations"`
}

// ExtractSections extracts structured sections from LLM response
func ExtractSections(response string) map[string]string {
	sections := make(map[string]string)

	// Define section markers
	markers := []string{
		"### 1. Match Percentage",
		"### 2. Technical Gap Analysis",
		"### 3. Evidence-Based Questions",
		"### 4. Key Strengths",
		"### 5. Recommendations",
	}

	for i, marker := range markers {
		start := strings.Index(response, marker)
		if start == -1 {
			continue
		}

		var end int
		if i < len(markers)-1 {
			nextMarker := strings.Index(response[start+len(marker):], markers[i+1])
			if nextMarker == -1 {
				end = len(response)
			} else {
				end = start + len(marker) + nextMarker
			}
		} else {
			end = len(response)
		}

		sectionName := strings.TrimPrefix(marker, "### ")
		sections[sectionName] = strings.TrimSpace(response[start:end])
	}

	return sections
}
