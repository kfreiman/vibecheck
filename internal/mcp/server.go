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
	"github.com/kfreiman/vibecheck/internal/ingest"
	"github.com/kfreiman/vibecheck/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CVResourceHandler handles CV resource requests
type CVResourceHandler struct {
	logger *slog.Logger
}

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
		h.logger.ErrorContext(ctx, "failed to parse file URI",
			"error", err,
			"uri", uri,
		)
		return nil, mcp.ResourceNotFoundError(uri)
	}

	// Check if it's a directory - list CV files
	if stat, err := os.Stat(path); err == nil && stat.IsDir() {
		cvFiles, err := FindCVFiles(path)
		if err != nil {
			h.logger.ErrorContext(ctx, "failed to find CV files",
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

		h.logger.DebugContext(ctx, "listed CV files in directory",
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
		h.logger.ErrorContext(ctx, "failed to read CV file",
			"error", err,
			"path", path,
		)
		return nil, mcp.ResourceNotFoundError(uri)
	}

	h.logger.DebugContext(ctx, "read CV file",
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

// Server encapsulates the MCP server with all its dependencies
type Server struct {
	mcpServer         *mcp.Server
	storageManager    *storage.StorageManager
	documentConverter converter.DocumentConverter
	logger            *slog.Logger
	config            Config
}

// NewServer creates a new MCP server with the given configuration
func NewServer(cfg Config, logger *slog.Logger) (*Server, error) {
	// Parse storage TTL from string to time.Duration
	ttl, err := time.ParseDuration(cfg.StorageTTL)
	if err != nil {
		logger.ErrorContext(context.Background(), "failed to parse storage TTL",
			"error", err,
			"ttl", cfg.StorageTTL,
		)
		return nil, fmt.Errorf("parse TTL: %w", err)
	}

	// Initialize storage manager
	storageManager, err := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   cfg.StoragePath,
		DefaultTTL: ttl,
	})
	if err != nil {
		logger.ErrorContext(context.Background(), "failed to initialize storage manager",
			"error", err,
		)
		return nil, fmt.Errorf("storage init: %w", err)
	}

	// Initialize document converter
	documentConverter := converter.NewPDFConverter()

	// Create server instance
	s := &Server{
		storageManager:    storageManager,
		documentConverter: documentConverter,
		logger:            logger,
		config:            cfg,
	}

	// Create MCP server implementation
	impl := &mcp.Implementation{
		Name:    "VibeCheckServer",
		Version: "2.0.0",
	}

	s.mcpServer = mcp.NewServer(impl, &mcp.ServerOptions{
		Instructions: ServerInstructions,
	})

	// Register all handlers
	s.registerHandlers()

	return s, nil
}

// registerHandlers registers all resources, tools, and prompts on the MCP server
func (s *Server) registerHandlers() {
	// Register resources
	s.registerResources()

	// Register tools
	s.registerTools()

	// Register prompts
	s.registerPrompts()
}

// registerResources registers all resource handlers
func (s *Server) registerResources() {
	// CV file resource handler
	cvHandler := &CVResourceHandler{logger: s.logger}
	s.mcpServer.AddResource(ResourceDefinitions[0], cvHandler.ReadResource)

	// Storage resource handler
	storageHandler := NewStorageResourceHandler(s.storageManager)

	// Register individual storage resources
	for _, resource := range storageHandler.ListResources() {
		s.mcpServer.AddResource(resource, storageHandler.ReadResource)
	}

	// Register resource templates
	for _, template := range storageHandler.ListResourceTemplates() {
		s.mcpServer.AddResourceTemplate(&template, storageHandler.ReadResource)
	}
}

// registerTools registers all tool handlers
func (s *Server) registerTools() {
	// Create ingestor
	ingestor := ingest.NewIngestor(s.storageManager, s.documentConverter).WithLogger(s.logger)

	// ingest_document tool
	ingestTool := NewIngestDocumentTool(ingestor).WithLogger(s.logger)
	s.mcpServer.AddTool(ToolDefinitions["ingest_document"], ingestTool.Call)

	// cleanup_storage tool
	cleanupTool := NewCleanupStorageTool(s.storageManager).WithLogger(s.logger)
	s.mcpServer.AddTool(ToolDefinitions["cleanup_storage"], cleanupTool.Call)

	// list_documents tool
	listDocumentsTool := NewListDocumentsTool(s.storageManager).WithLogger(s.logger)
	s.mcpServer.AddTool(ToolDefinitions["list_documents"], listDocumentsTool.Call)

	// generate_interview_questions tool
	interviewQuestionsTool := NewInterviewQuestionsTool(s.storageManager).WithLogger(s.logger)
	s.mcpServer.AddTool(ToolDefinitions["generate_interview_questions"], interviewQuestionsTool.Call)

	// analyze_cv_jd tool
	analyzeTool := NewAnalyzeTool(s.storageManager).WithLogger(s.logger)
	s.mcpServer.AddTool(ToolDefinitions["analyze_cv_jd"], analyzeTool.Call)
}

// registerPrompts registers all prompt handlers
func (s *Server) registerPrompts() {
	// analyze_fit prompt
	analyzeFitPrompt := NewAnalyzeFitPrompt(s.storageManager).WithLogger(s.logger)
	for _, promptDef := range PromptDefinitions {
		s.mcpServer.AddPrompt(promptDef, analyzeFitPrompt.Handle)
	}
}

// ListenAndServe starts the HTTP server and begins handling requests
func (s *Server) ListenAndServe() error {
	// Create HTTP streaming handler
	httpHandler := mcp.NewStreamableHTTPHandler(func(req *http.Request) *mcp.Server {
		return s.mcpServer
	}, &mcp.StreamableHTTPOptions{
		JSONResponse: true,
		Logger:       nil,
	})

	// Set up HTTP server with routes
	mux := http.NewServeMux()
	mux.Handle("/mcp", httpHandler)
	mux.HandleFunc("/health/live", s.livenessHandler)
	mux.HandleFunc("/health/ready", s.readinessHandler)
	mux.HandleFunc("/", s.indexHandler)

	// Start HTTP server
	addr := fmt.Sprintf(":%d", s.config.Port)
	s.logger.InfoContext(context.Background(), "starting MCP server",
		"port", s.config.Port,
		"endpoints", []string{"/mcp", "/health/live", "/health/ready", "/"},
	)
	return http.ListenAndServe(addr, mux)
}

// livenessHandler checks if the server is running and accepting requests
func (s *Server) livenessHandler(w http.ResponseWriter, r *http.Request) {
	LivenessHandlerWithLogger(w, r, s.logger)
}

// readinessHandler checks if the server is ready to handle requests
func (s *Server) readinessHandler(w http.ResponseWriter, r *http.Request) {
	ReadinessHandlerWithLogger(w, r, s.storageManager, s.logger)
}

// indexHandler returns the server information page
func (s *Server) indexHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "VibeCheck MCP Server\n\n")
	fmt.Fprintf(w, "Endpoints:\n")
	fmt.Fprintf(w, "  POST /mcp          - Streamable HTTP transport (recommended)\n")
	fmt.Fprintf(w, "  GET  /health/live  - Liveness probe\n")
	fmt.Fprintf(w, "  GET  /health/ready - Readiness probe\n")
	fmt.Fprintf(w, "  GET  /             - This help message\n\n")
	fmt.Fprintf(w, "Server: %s %s\n", "VibeCheckServer", "2.0.0")
}
