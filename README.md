# VibeCheck - Production-Grade MCP CV Analysis Server

A production-grade MCP (Model Context Protocol) server for ingesting CVs and job descriptions, with intelligent analysis capabilities.

## Features

- **Document Ingestion**: Support for PDF and markdown files (pure Go implementation)
- **Content-Based Deduplication**: UUID v5 generation ensures same file gets same URI
- **Storage Management**: Persistent storage with TTL-based cleanup
- **Dual URI Schemes**: `cv://[uuid]` and `jd://[uuid]` for stored documents
- **Structured Analysis**: Prompts for CV vs job description comparison
- **HTTP/SSE Transport**: Streamable HTTP and SSE endpoints for MCP communication

## Prerequisites

- Go 1.21 or higher

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

Start the MCP server (HTTP/SSE transport only):

```bash
./vibecheck mcp-server
```

The server will start on port 8080 with the following endpoints:
- `POST /mcp` - Streamable HTTP transport (recommended)
- `GET /sse` - SSE transport (legacy)
- `GET /` - Help message

### CLI Commands

```bash
# Show help
./vibecheck --help

# Start MCP server
./vibecheck mcp-server
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
│   │   ├── resource_handler.go # cv://, jd:// handlers
│   │   ├── errors.go       # Structured error types
│   │   └── retry.go        # Retry logic with exponential backoff
│   ├── storage/
│   │   └── manager.go      # Storage with UUID v5
│   ├── converter/
│   │   └── pdf.go          # Pure Go PDF conversion
└── storage/                # Runtime storage (gitignored)
```

## Security

- **Path Traversal Prevention**: Strict path validation for file operations
- **Context Cancellation**: Operations respect context timeouts
- **Error Handling**: Structured error types with detailed context
- **Retry Logic**: Exponential backoff with jitter for transient failures

## Production Deployment

### Docker

```bash
# Build the image
docker build -t vibecheck .

# Run the container
docker run -p 8080:8080 \
  -e VIBECHECK_STORAGE_PATH=/app/storage \
  -e VIBECHECK_STORAGE_TTL=24h \
  -v $(pwd)/storage:/app/storage \
  vibecheck
```

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `VIBECHECK_STORAGE_PATH` | Storage directory | `./storage` |
| `VIBECHECK_STORAGE_TTL` | Default TTL for cleanup | `24h` |

### Health Checks

The server exposes endpoints for health monitoring:

```bash
# Check if server is responding
curl http://localhost:8080/

# Check MCP endpoint
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}'
```

### Storage Management

Regular cleanup is recommended to prevent storage bloat:

```bash
# Manual cleanup (remove files older than 24h)
curl -X POST http://localhost:8080/mcp \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"cleanup_storage","arguments":{"ttl":"24h"}}}'
```

### Monitoring

- **Port**: 8080 (HTTP)
- **Endpoints**: `/mcp` (streamable HTTP), `/sse` (SSE), `/` (health/help)
- **Logs**: Server logs to stdout/stderr (use Docker logs for containerized deployment)

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
