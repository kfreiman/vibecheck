package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kfreiman/vibecheck/internal/converter"
	"github.com/kfreiman/vibecheck/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/samber/slog-zerolog"
)

var logger = slog.New(slogzerolog.Option{}.NewZerologHandler())

// CVResourceHandler handles CV resource requests
type CVResourceHandler struct{}

// ReadResource processes resource requests for CV data
func (h *CVResourceHandler) ReadResource(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	uri := req.Params.URI

	// Handle file:// scheme only
	if !strings.HasPrefix(uri, "file://") {
		return nil, mcp.ResourceNotFoundError(uri)
	}

	// Parse path from URI
	path := strings.TrimPrefix(uri, "file://")
	path, err := filepath.Abs(path)
	if err != nil {
		logger.ErrorContext(ctx, "failed to parse file URI",
			"error", err,
			"uri", uri,
		)
		return nil, mcp.ResourceNotFoundError(uri)
	}

	// Check if it's a directory - list CV files
	if stat, err := os.Stat(path); err == nil && stat.IsDir() {
		cvFiles, err := FindCVFiles(path)
		if err != nil {
			logger.ErrorContext(ctx, "failed to find CV files",
				"error", err,
				"path", path,
			)
			return nil, mcp.ResourceNotFoundError(uri)
		}

		var list strings.Builder
		list.WriteString("# CV Files\n\n")
		for _, f := range cvFiles {
			fmt.Fprintf(&list, "- file://%s\n", f)
		}
		list.WriteString("\nUse file:///path/to/file.md to read a specific CV.\n")

		logger.DebugContext(ctx, "listed CV files in directory",
			"path", path,
			"count", len(cvFiles),
		)

		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{
				URI:      uri,
				MIMEType: "text/markdown",
				Text:     list.String(),
			}},
		}, nil
	}

	// Read single file
	content, err := ReadCVFile(path)
	if err != nil {
		logger.ErrorContext(ctx, "failed to read CV file",
			"error", err,
			"path", path,
		)
		return nil, mcp.ResourceNotFoundError(uri)
	}

	logger.DebugContext(ctx, "read CV file",
		"path", path,
		"size", len(content),
	)

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     content,
		}},
	}, nil
}

// StartMCPServer starts an MCP server using HTTP/SSE transport
func StartMCPServer() error {
	ctx := context.Background()

	// Initialize storage manager
	storageManager, err := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   getEnvOrDefault("VIBECHECK_STORAGE_PATH", "./storage"),
		DefaultTTL: getEnvDurationOrDefault("VIBECHECK_STORAGE_TTL", 24*time.Hour),
	})
	if err != nil {
		logger.ErrorContext(ctx, "failed to initialize storage manager",
			"error", err,
		)
		return fmt.Errorf("failed to initialize storage: %w", err)
	}

	// Initialize document converter (pure Go PDF extraction)
	documentConverter := converter.NewPDFConverter()

	// Create handlers
	cvHandler := &CVResourceHandler{}
	storageResourceHandler := NewStorageResourceHandler(storageManager)
	ingestTool := NewIngestDocumentTool(storageManager, documentConverter)
	cleanupTool := NewCleanupStorageTool(storageManager)
	listDocumentsTool := NewListDocumentsTool(storageManager)
	cvCheckTool, _ := NewCVCheckTool()
	analyzeFitPrompt := NewAnalyzeFitPrompt(storageManager)
	interviewQuestionsTool := NewInterviewQuestionsTool(storageManager)

	// Create MCP server
	impl := &mcp.Implementation{
		Name:    "VibeCheckServer",
		Version: "2.0.0",
	}
	opts := &mcp.ServerOptions{
		Instructions: `VibeCheck Server - CV Analysis Tool (v1 - Portfolio/Demo Version)

This server provides CV and job description management with intelligent analysis.

**Note:** This is v1 for portfolio/demo purposes only. Do not use with sensitive personal data.

## Transport

This server uses HTTP/SSE transport only. Connect via:
- POST /mcp  - Streamable HTTP transport (recommended)
- GET  /sse  - SSE transport (legacy)

Stdio transport is not supported.

## Resources

### File Resources (file://)
- file:///path/to/cv.md: Read a CV markdown file
- file:///path/to/cv/: List all CV files in a directory

### Storage Resources (cv://, jd://)
- cv://[uuid]: Access an ingested CV document
- jd://[uuid]: Access an ingested job description

## Tools

### ingest_document
Ingest a CV or job description into storage.
Parameters:
- path: File path or URL to document (PDF, MD)
- type: Document type ("cv" or "jd")

Example: {"path": "./resume.pdf", "type": "cv"}

Returns a URI (cv://[uuid] or jd://[uuid]) for later use.

### cleanup_storage
Remove old documents from storage based on TTL.
Parameters:
- ttl: Time to live (e.g., "24h" or 24 for hours)

Example: {"ttl": "48h"}

### list_documents
List all stored documents (CVs and job descriptions) by their UUIDs.
Parameters:
- type: Optional filter - "cv" for CVs only, "jd" for job descriptions only, or empty for all

Example: {"type": "cv"}

Returns structured list of document URIs (cv://[uuid] or jd://[uuid]).

### cv_check
Compare a CV against a job description (legacy method).
Parameters:
- cv: CV content or file path
- job: Job description content or file path

### generate_interview_questions
Generate targeted interview questions based on CV and job description gap analysis.
Parameters:
- cv_uri: URI of ingested CV (cv://[uuid])
- jd_uri: URI of ingested job description (jd://[uuid])
- style: Optional - "technical", "behavioral", or "comprehensive" (default)
- count: Optional - number of questions to generate (default: 5)

Example: {"cv_uri": "cv://550e8400-e29b...", "jd_uri": "jd://550e8400-e29b...", "style": "technical", "count": 5}

Returns a prompt for generating interview questions focused on:
- Skills/experience gaps between CV and JD
- Areas needing clarification
- Technical and behavioral question balance

## Prompts

### cv_analysis
Analyze CV vs job description using raw content or URLs.
Parameters (choose one for CV and one for job):
- cv_content: Raw CV markdown text
- cv_url: URL to CV document
- cv_filepath: Path to CV file
- job_content: Raw job description text
- job_url: URL to job posting
- job_filepath: Path to job file

### analyze_fit
Analyze fit between ingested CV and job description.
Parameters:
- cv_uri: URI of ingested CV (cv://[uuid])
- jd_uri: URI of ingested job description (jd://[uuid])

Example: {"cv_uri": "cv://550e8400-e29b...", "jd_uri": "jd://550e8400-e29b..."}

Returns structured analysis with:
- Match percentage
- Technical gap analysis
- Evidence-based questions
- Key strengths
- Recommendations

## Environment Variables

- VIBECHECK_STORAGE_PATH: Storage directory (default: ./storage)
- VIBECHECK_STORAGE_TTL: Default TTL for cleanup (default: 24h)
`,
	}
	server := mcp.NewServer(impl, opts)

	// Register file-based resources
	server.AddResource(&mcp.Resource{
		URI:         "file:///cv",
		Name:        "CV Directory",
		Description: "List all CV markdown files",
		MIMEType:    "text/markdown",
	}, cvHandler.ReadResource)

	// Register storage resources
	for _, resource := range storageResourceHandler.ListResources() {
		server.AddResource(resource, storageResourceHandler.ReadResource)
	}

	// Register resource templates
	for _, template := range storageResourceHandler.ListResourceTemplates() {
		server.AddResourceTemplate(&template, storageResourceHandler.ReadResource)
	}

	// Register tools

	// ingest_document tool
	server.AddTool(&mcp.Tool{
		Name:        "ingest_document",
		Description: "Ingest a CV or job description into storage. Supports local paths, URLs, and various document formats (PDF, DOCX, MD). Returns a URI for later use.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path, URL, or raw markdown content to ingest",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Document type: 'cv' or 'jd'",
					"enum":        []string{"cv", "jd"},
					"default":     "cv",
				},
			},
			"required": []string{"path"},
		},
	}, ingestTool.Call)

	// cleanup_storage tool
	server.AddTool(&mcp.Tool{
		Name:        "cleanup_storage",
		Description: "Remove documents older than the specified TTL from storage. Useful for maintaining storage hygiene.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"ttl": map[string]interface{}{
					"type":        "string",
					"description": "Time to live (e.g., '24h', '7d', or hours as number). Uses default TTL if not specified.",
				},
			},
			"required": []string{},
		},
	}, cleanupTool.Call)

	// list_documents tool
	server.AddTool(&mcp.Tool{
		Name:        "list_documents",
		Description: "List all stored documents (CVs and job descriptions) by their UUIDs. Returns structured data with document URIs.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"type": map[string]interface{}{
					"type":        "string",
					"description": "Optional filter: 'cv' for CVs only, 'jd' for job descriptions only, or empty for all documents",
					"enum":        []string{"cv", "jd"},
				},
			},
			"required": []string{},
		},
	}, listDocumentsTool.Call)

	// cv_check tool (legacy)
	if cvCheckTool != nil {
		server.AddTool(&mcp.Tool{
			Name:        "cv_check",
			Description: "Compare a CV against a job description. Accepts file paths or raw text for both parameters.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"cv": map[string]interface{}{
						"type":        "string",
						"description": "CV content (file path or raw text)",
					},
					"job": map[string]interface{}{
						"type":        "string",
						"description": "Job description content (file path or raw text)",
					},
				},
				"required": []string{"cv", "job"},
			},
		}, cvCheckTool.Call)
	}

	// generate_interview_questions tool
	server.AddTool(&mcp.Tool{
		Name:        "generate_interview_questions",
		Description: "Generate targeted interview questions based on CV and job description gap analysis. Returns a prompt for generating questions focused on skills gaps, areas needing clarification, and technical/behavioral balance.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"cv_uri": map[string]interface{}{
					"type":        "string",
					"description": "URI of ingested CV (cv://[uuid])",
				},
				"jd_uri": map[string]interface{}{
					"type":        "string",
					"description": "URI of ingested job description (jd://[uuid])",
				},
				"style": map[string]interface{}{
					"type":        "string",
					"description": "Question style: 'technical', 'behavioral', or 'comprehensive' (default)",
					"enum":        []string{"technical", "behavioral", "comprehensive"},
					"default":     "comprehensive",
				},
				"count": map[string]interface{}{
					"type":        "integer",
					"description": "Number of questions to generate (default: 5)",
					"minimum":     1,
					"maximum":     20,
					"default":     5,
				},
			},
			"required": []string{"cv_uri", "jd_uri"},
		},
	}, interviewQuestionsTool.Call)

	// Register prompts

	// cv_analysis prompt (existing)
	server.AddPrompt(&mcp.Prompt{
		Name:        "cv_analysis",
		Description: "CV vs Job Description Analysis Prompt",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        CVContentParam,
				Title:       "CV Content",
				Description: "Raw CV content (markdown text)",
				Required:    false,
			},
			{
				Name:        CVURLParam,
				Title:       "CV URL",
				Description: "URL to CV document (PDF) - will be converted to markdown",
				Required:    false,
			},
			{
				Name:        CVFilepathParam,
				Title:       "CV Filepath",
				Description: "Path to CV file (PDF, MD) - will be converted to markdown",
				Required:    false,
			},
			{
				Name:        JobContentParam,
				Title:       "Job Content",
				Description: "Raw job description content (markdown text)",
				Required:    false,
			},
			{
				Name:        JobURLParam,
				Title:       "Job URL",
				Description: "URL to job posting (PDF) - will be converted to markdown",
				Required:    false,
			},
			{
				Name:        JobFilepathParam,
				Title:       "Job Filepath",
				Description: "Path to job description file (PDF, MD) - will be converted to markdown",
				Required:    false,
			},
		},
	}, CVPromptHandler)

	// analyze_fit prompt (new)
	server.AddPrompt(&mcp.Prompt{
		Name:        "analyze_fit",
		Description: "Analyze fit between ingested CV and job description with structured output",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "cv_uri",
				Title:       "CV URI",
				Description: "URI of ingested CV (cv://[uuid])",
				Required:    true,
			},
			{
				Name:        "jd_uri",
				Title:       "Job Description URI",
				Description: "URI of ingested job description (jd://[uuid])",
				Required:    true,
			},
		},
	}, analyzeFitPrompt.Handle)

	// Create HTTP streaming handler
	httpHandler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{
		JSONResponse: true,
		Logger:       nil,
	})

	// Also support SSE for compatibility
	sseHandler := mcp.NewSSEHandler(func(req *http.Request) *mcp.Server {
		return server
	}, &mcp.SSEOptions{})

	// Set up HTTP server with routes
	mux := http.NewServeMux()
	mux.Handle("/mcp", httpHandler)
	mux.Handle("/sse", sseHandler)
	mux.HandleFunc("/health/live", LivenessHandler)
	mux.HandleFunc("/health/ready", ReadinessHandlerFunc(storageManager))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintf(w, "VibeCheck MCP Server\n\n")
		fmt.Fprintf(w, "Endpoints:\n")
		fmt.Fprintf(w, "  POST /mcp          - Streamable HTTP transport (recommended)\n")
		fmt.Fprintf(w, "  GET  /sse          - SSE transport\n")
		fmt.Fprintf(w, "  GET  /health/live  - Liveness probe\n")
		fmt.Fprintf(w, "  GET  /health/ready - Readiness probe\n")
		fmt.Fprintf(w, "  GET  /             - This help message\n\n")
		fmt.Fprintf(w, "Server: %s %s\n", impl.Name, impl.Version)
	})
	// Start HTTP server
	logger.InfoContext(ctx, "starting MCP server",
		"port", 8080,
		"endpoints", []string{"/mcp", "/sse", "/health/live", "/health/ready", "/"},
	)
	return http.ListenAndServe(":8080", mux)
}

// getEnvOrDefault returns an environment variable or a default value
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvDurationOrDefault returns an environment variable as duration or default
func getEnvDurationOrDefault(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
