package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	"github.com/kfreiman/vibecheck/internal/analysis"
	"github.com/kfreiman/vibecheck/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// AnalyzeTool handles structured CV/JD comparison using bleve BM25 analysis
type AnalyzeTool struct {
	storageManager *storage.StorageManager
	engine         *analysis.AnalysisEngine
	logger         *slog.Logger
}

// NewAnalyzeTool creates a new analyze tool
func NewAnalyzeTool(sm *storage.StorageManager) *AnalyzeTool {
	return &AnalyzeTool{
		storageManager: sm,
		engine:         analysis.NewAnalysisEngine(),
		logger:         slog.Default(),
	}
}

// WithLogger sets the logger for the tool
func (t *AnalyzeTool) WithLogger(logger *slog.Logger) *AnalyzeTool {
	t.logger = logger
	return t
}

// AnalyzeResult represents the structured analysis output
type AnalyzeResult struct {
	MatchPercentage int      `json:"match_percentage"`
	SkillCoverage   float64  `json:"skill_coverage"`
	TopSkills       []string `json:"top_skills"`
	MissingSkills   []string `json:"missing_skills"`
	AnalysisSummary string   `json:"analysis_summary"`
}

// Call implements the MCP tool interface
func (t *AnalyzeTool) Call(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse arguments
	var args struct {
		CvURI string `json:"cv_uri"`
		JdURI string `json:"jd_uri"`
	}

	if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: invalid JSON format - %v", err)},
			},
		}, fmt.Errorf("invalid arguments: %w", err)
	}

	// Validate required parameters
	if args.CvURI == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: 'cv_uri' parameter is required"},
			},
		}, &ValidationError{Field: "cv_uri", Reason: "required parameter missing"}
	}
	if args.JdURI == "" {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: "Error: 'jd_uri' parameter is required"},
			},
		}, &ValidationError{Field: "jd_uri", Reason: "required parameter missing"}
	}

	// Validate URI formats
	cvDocType, _, err := storage.ParseURI(args.CvURI)
	if err != nil || cvDocType != storage.DocumentTypeCV {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: cv_uri - invalid CV URI format (must be cv://), details: %v", err)},
			},
		}, &ValidationError{Field: "cv_uri", Value: args.CvURI, Reason: "must be cv:// format"}
	}

	jdDocType, _, parseErr := storage.ParseURI(args.JdURI)
	if parseErr != nil || jdDocType != storage.DocumentTypeJD {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: jd_uri - invalid JD URI format (must be jd://), details: %v", parseErr)},
			},
		}, &ValidationError{Field: "jd_uri", Value: args.JdURI, Reason: "must be jd:// format"}
	}

	// Check if documents exist
	if !t.storageManager.DocumentExists(args.CvURI) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: CV document not found: %s", args.CvURI)},
			},
		}, &ValidationError{Field: "cv_uri", Value: args.CvURI, Reason: "document not found"}
	}
	if !t.storageManager.DocumentExists(args.JdURI) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: JD document not found: %s", args.JdURI)},
			},
		}, &ValidationError{Field: "jd_uri", Value: args.JdURI, Reason: "document not found"}
	}

	// Read documents from storage
	cvContent, err := t.storageManager.ReadDocument(args.CvURI)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: failed to read CV document: %v", err)},
			},
		}, err
	}

	jdContent, err := t.storageManager.ReadDocument(args.JdURI)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: failed to read JD document: %v", err)},
			},
		}, err
	}

	// Strip frontmatter (YAML format between --- markers)
	cvClean := stripFrontmatter(string(cvContent))
	jdClean := stripFrontmatter(string(jdContent))

	// Perform BM25 analysis
	analysisResult, err := t.engine.Analyze(ctx, cvClean, jdClean)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: analysis failed: %v", err)},
			},
		}, err
	}

	// Build analysis summary
	summary := t.buildSummary(analysisResult)

	// Create structured result
	result := AnalyzeResult{
		MatchPercentage: analysisResult.MatchPercentage,
		SkillCoverage:   analysisResult.SkillCoverage,
		TopSkills:       analysisResult.TopSkills,
		MissingSkills:   analysisResult.MissingSkills,
		AnalysisSummary: summary,
	}

	// Return as structured JSON
	jsonData, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error: failed to format result: %v", err)},
			},
		}, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(jsonData)},
		},
	}, nil
}

// stripFrontmatter removes YAML frontmatter (--- delimited) from content
func stripFrontmatter(content string) string {
	// Remove YAML frontmatter between --- markers
	frontmatterPattern := regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)
	cleaned := frontmatterPattern.ReplaceAllString(content, "")
	return strings.TrimSpace(cleaned)
}

// buildSummary creates a human-readable summary of the analysis
func (t *AnalyzeTool) buildSummary(result *analysis.AnalysisResult) string {
	var sb strings.Builder

	sb.WriteString("CV/Job Description Analysis Report\n")
	sb.WriteString("==================================\n\n")

	sb.WriteString(fmt.Sprintf("Match Percentage: %d%%\n", result.MatchPercentage))
	sb.WriteString(fmt.Sprintf("Skill Coverage: %.1f%%\n", result.SkillCoverage*100))
	sb.WriteString(fmt.Sprintf("Common Terms: %d\n", len(result.CommonTerms)))
	sb.WriteString(fmt.Sprintf("Missing Skills: %d\n\n", len(result.MissingSkills)))

	if len(result.TopSkills) > 0 {
		sb.WriteString("Top Matching Skills:\n")
		for i, skill := range result.TopSkills {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, skill))
		}
		sb.WriteString("\n")
	}

	if len(result.MissingSkills) > 0 {
		sb.WriteString("Missing Skills (gaps to address):\n")
		for i, skill := range result.MissingSkills {
			sb.WriteString(fmt.Sprintf("  %d. %s\n", i+1, skill))
		}
		sb.WriteString("\n")
	}

	if len(result.CommonTerms) > 0 {
		sb.WriteString("Detailed Term Analysis:\n")
		for i, ts := range result.CommonTerms {
			if i < 5 { // Show top 5 common terms
				sb.WriteString(fmt.Sprintf("  %s (score: %.2f)\n", ts.Term, ts.Score))
			}
		}
	}

	return sb.String()
}