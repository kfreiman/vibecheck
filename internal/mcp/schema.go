package mcp

import "github.com/modelcontextprotocol/go-sdk/mcp"

// ServerInstructions contains the MCP server instructions for clients
const ServerInstructions = `VibeCheck Server - CV Analysis Tool (v1 - Portfolio/Demo Version)

This server provides CV and job description management with intelligent analysis.

**Note:** This is v1 for portfolio/demo purposes only. Do not use with sensitive personal data.

## Transport

This server uses streamable HTTP transport only. Connect via:
- POST /mcp  - Streamable HTTP transport (recommended)

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

### analyze_cv_jd
Structured CV/Job Description analysis with BM25 match scoring.
Parameters:
- cv_uri: URI of ingested CV (cv://[uuid])
- jd_uri: URI of ingested job description (jd://[uuid])

Example: {"cv_uri": "cv://550e8400-e29b...", "jd_uri": "jd://550e8400-e29b..."}

Returns structured JSON with:
- match_percentage: 0-100% based on BM25 scoring
- skill_coverage: Ratio of JD terms present in CV
- top_skills: Common terms with highest scores
- missing_skills: JD terms not found in CV
- analysis_summary: Human-readable report

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
- VIBECHECK_PORT: HTTP server port (default: 8080)
- VIBECHECK_DEBUG: Enable debug logging (default: false)
`

// ToolDefinitions contains the MCP tool definitions
var ToolDefinitions = map[string]*mcp.Tool{
	"ingest_document": {
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
	},
	"cleanup_storage": {
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
	},
	"list_documents": {
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
	},
	"generate_interview_questions": {
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
	},
	"analyze_cv_jd": {
		Name:        "analyze_cv_jd",
		Description: "Structured CV/Job Description analysis with BM25 match scoring. Returns match percentage, skill coverage, and gap analysis.",
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
			},
			"required": []string{"cv_uri", "jd_uri"},
		},
	},
}

// PromptDefinitions contains the MCP prompt definitions
var PromptDefinitions = []*mcp.Prompt{
	{
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
	},
}

// ResourceDefinitions contains the MCP resource definitions
var ResourceDefinitions = []*mcp.Resource{
	{
		URI:         "file:///cv",
		Name:        "CV Directory",
		Description: "List all CV markdown files",
		MIMEType:    "text/markdown",
	},
}

// ResourceTemplateDefinitions contains the MCP resource template definitions
var ResourceTemplateDefinitions = []mcp.ResourceTemplate{
	{
		URITemplate: "cv://{id}",
		Name:        "CV Document",
		Description: "Access a stored CV document by its UUID",
		MIMEType:    "text/markdown",
	},
	{
		URITemplate: "jd://{id}",
		Name:        "Job Description",
		Description: "Access a stored job description by its UUID",
		MIMEType:    "text/markdown",
	},
}
