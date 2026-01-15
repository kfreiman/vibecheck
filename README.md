# VibeCheck - Production-Grade MCP CV Analysis Server

A production-grade MCP (Model Context Protocol) server for ingesting CVs and job descriptions, with intelligent analysis capabilities.

## Features

- **Document Ingestion**: Support for PDF, DOCX, PPTX, XLSX, and markdown files
- **Content-Based Deduplication**: UUID v5 generation ensures same file gets same URI
- **PII Redaction**: Automatic filtering of emails and phone numbers
- **Storage Management**: Persistent storage with TTL-based cleanup
- **Dual URI Schemes**: `cv://[uuid]` and `jd://[uuid]` for stored documents
- **Structured Analysis**: Prompts for CV vs job description comparison

## Prerequisites

### Go

- Go 1.21 or higher

### Python & MarkItDown

The server uses Microsoft's MarkItDown for document conversion:

```bash
# Install Python (if not installed)
# macOS: brew install python
# Ubuntu: sudo apt install python3 python3-pip

# Install MarkItDown
pip install markitdown

# Verify installation
markitdown --help
```

## Installation

### From Source

```bash
# Clone the repository
git clone <repository-url>
cd vibecheck

# Install dependencies
go mod tidy

# Build the binary
make build

# Initialize storage directories
make init-storage
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VIBECHECK_STORAGE_PATH` | Storage directory | `./storage` |
| `VIBECHECK_STORAGE_TTL` | Default TTL for cleanup | `24h` |

## Usage

### MCP Server

Start the MCP server with stdio transport:

```bash
./vibecheck mcp-server
```

### CLI Commands

```bash
# Show help
./vibecheck --help

# Start MCP server
./vichbeck mcp-server
```

### Development

```bash
# Run with hot reload
make dev
```

## MCP Tools

### ingest_document

Ingest a CV or job description into storage.

```json
{
  "name": "ingest_document",
  "arguments": {
    "path": "./resume.pdf",
    "type": "cv"
  }
}
```

**Parameters:**

- `path`: File path, URL, or raw markdown content
- `type`: `"cv"` or `"jd"` (default: `"cv"`)

**Returns:** A URI (e.g., `cv://550e8400-e29b-41d4-a716-446655440000`)

### cleanup_storage

Remove old documents from storage.

```json
{
  "name": "cleanup_storage",
  "arguments": {
    "ttl": "48h"
  }
}
```

**Parameters:**

- `ttl`: Time to live (e.g., `"24h"`, `"7d"`, or hours as number)

## MCP Prompts

### analyze_fit

Analyze the fit between an ingested CV and job description.

```json
{
  "name": "analyze_fit",
  "arguments": {
    "cv_uri": "cv://550e8400-e29b-41d4-a716-446655440000",
    "jd_uri": "jd://550e8400-e29b-41d4-a716-446655440000"
  }
}
```

**Returns:** Structured analysis with:

- Match percentage (0-100%)
- Technical gap analysis
- Evidence-based questions
- Key strengths
- Recommendations

## Storage Structure

```
storage/
├── cv/
│   └── [uuid].md       # Ingested CV documents
└── jd/
    └── [uuid].md       # Ingested job descriptions
```

Each file includes YAML frontmatter:

```yaml
---
id: 550e8400-e29b-41d4-a716-446655440000
original_filename: resume.pdf
ingested_at: 2024-01-13T10:30:00Z
type: cv
---
```

## Architecture

```
vibecheck/
├── cmd/
│   └── mcp_server.go       # MCP server entry point
├── internal/
│   ├── mcp/
│   │   ├── server.go       # MCP server implementation
│   │   ├── ingest_tool.go  # ingest_document tool
│   │   ├── cleanup_tool.go # cleanup_storage tool
│   │   ├── analyze_prompt.go # analyze_fit prompt
│   │   └── resource_handler.go # cv://, jd:// handlers
│   ├── storage/
│   │   └── manager.go      # Storage with UUID v5
│   ├── converter/
│   │   └── markitdown.go   # MarkItDown wrapper
│   └── redaction/
│       └── redact.go       # PII redaction filter
└── storage/                # Runtime storage (gitignored)
```

## Security

- **Path Traversal Prevention**: Strict path validation
- **PII Redaction**: Automatic filtering of emails and phone numbers
- **Context Cancellation**: Python process killed on timeout
- **Error Handling**: Detailed error messages with stderr capture

## Testing

```bash
# Run all tests
make test

# Run with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## License

MIT
