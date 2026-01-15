package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/kfreiman/vibecheck/internal/storage"
)

// CleanupStorageTool handles storage cleanup
type CleanupStorageTool struct {
	storageManager *storage.StorageManager
}

// NewCleanupStorageTool creates a new cleanup storage tool
func NewCleanupStorageTool(storageManager *storage.StorageManager) *CleanupStorageTool {
	return &CleanupStorageTool{
		storageManager: storageManager,
	}
}

// Call implements the MCP tool interface
func (t *CleanupStorageTool) Call(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse arguments
	var args struct {
		TTL string `json:"ttl"` // TTL in duration string (e.g., "24h") or hours as number
	}

	if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
		return nil, fmt.Errorf("invalid input format: %w", err)
	}

	// Parse TTL
	var ttl time.Duration
	if args.TTL != "" {
		// Try parsing as duration string first
		var err error
		ttl, err = time.ParseDuration(args.TTL)
		if err != nil {
			// Try parsing as hours (number)
			if hours, err := strconv.Atoi(args.TTL); err == nil {
				ttl = time.Duration(hours) * time.Hour
			} else {
				return &mcp.CallToolResult{
					Content: []mcp.Content{
						&mcp.TextContent{Text: "Error: invalid TTL format. Use duration string (e.g., '24h') or hours as number"},
					},
				}, fmt.Errorf("invalid TTL format")
			}
		}
	}

	// Get storage stats before cleanup
	cvCountBefore, jdCountBefore, err := t.storageManager.GetStorageStats()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error getting storage stats: %v", err)},
			},
		}, err
	}

	// Perform cleanup
	removed, err := t.storageManager.Cleanup(ttl)
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error during cleanup: %v", err)},
			},
		}, err
	}

	// Get storage stats after cleanup
	cvCountAfter, jdCountAfter, _ := t.storageManager.GetStorageStats()

	ttlDisplay := "default (24h)"
	if ttl > 0 {
		ttlDisplay = ttl.String()
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(`Storage cleanup completed!

TTL used: %s
Files removed: %d

Storage statistics:
- CVs before: %d, after: %d
- Job descriptions before: %d, after: %d

Files cleaned: %d total`, ttlDisplay, removed, cvCountBefore, cvCountAfter, jdCountBefore, jdCountAfter, removed)},
		},
	}, nil
}

// CleanupHandler is a handler function for the cleanup_storage tool
func CleanupHandler(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	tool := &CleanupStorageTool{}
	return tool.Call(ctx, req)
}

// CleanupOldFiles removes old files without MCP tool interface
func CleanupOldFiles(storageManager *storage.StorageManager, ttl time.Duration) (int64, error) {
	return storageManager.Cleanup(ttl)
}

// StartCleanupRoutine starts a background routine for periodic cleanup
func StartCleanupRoutine(storageManager *storage.StorageManager, interval, ttl time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			if removed, err := storageManager.Cleanup(ttl); err == nil {
				// Log cleanup results (in production, use proper logging)
				fmt.Printf("Storage cleanup: removed %d files\n", removed)
			}
		}
	}()
}

// GetStorageStatsHandler returns storage statistics
func GetStorageStatsHandler(ctx context.Context, storageManager *storage.StorageManager) (*mcp.CallToolResult, error) {
	cvCount, jdCount, err := storageManager.GetStorageStats()
	if err != nil {
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error getting storage stats: %v", err)},
			},
		}, err
	}

	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf(`Storage Statistics

CV Documents: %d
Job Descriptions: %d
Total: %d

Storage directories:
- ./storage/cv
- ./storage/jd`, cvCount, jdCount, cvCount+jdCount)},
		},
	}, nil
}
