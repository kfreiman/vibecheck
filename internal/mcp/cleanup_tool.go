package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/kfreiman/vibecheck/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// CleanupStorageTool handles storage cleanup
type CleanupStorageTool struct {
	storageManager *storage.StorageManager
	logger         *slog.Logger
}

// NewCleanupStorageTool creates a new cleanup storage tool
func NewCleanupStorageTool(storageManager *storage.StorageManager) *CleanupStorageTool {
	return &CleanupStorageTool{
		storageManager: storageManager,
		logger:         slog.Default(),
	}
}

// WithLogger sets the logger for the tool
func (t *CleanupStorageTool) WithLogger(logger *slog.Logger) *CleanupStorageTool {
	t.logger = logger
	return t
}

// Call implements the MCP tool interface
func (t *CleanupStorageTool) Call(ctx context.Context, request *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Parse arguments
	var args struct {
		TTL string `json:"ttl"` // TTL in duration string (e.g., "24h") or hours as number
	}

	if err := json.Unmarshal(request.Params.Arguments, &args); err != nil {
		t.logger.ErrorContext(ctx, "failed to parse cleanup arguments",
			"error", err,
			"operation", "cleanup_storage",
		)
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
				t.logger.ErrorContext(ctx, "invalid TTL format",
					"error", err,
					"ttl_input", args.TTL,
					"operation", "cleanup_storage",
				)
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
		t.logger.ErrorContext(ctx, "failed to get storage stats before cleanup",
			"error", err,
			"operation", "cleanup_storage",
		)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error getting storage stats: %v", err)},
			},
		}, err
	}

	// Perform cleanup
	removed, err := t.storageManager.Cleanup(ttl)
	if err != nil {
		t.logger.ErrorContext(ctx, "cleanup operation failed",
			"error", err,
			"ttl", ttl,
			"operation", "cleanup_storage",
		)
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

	t.logger.InfoContext(ctx, "storage cleanup completed via tool",
		"ttl", ttlDisplay,
		"removed", removed,
		"cv_before", cvCountBefore,
		"cv_after", cvCountAfter,
		"jd_before", jdCountBefore,
		"jd_after", jdCountAfter,
	)

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
				slog.Default().InfoContext(context.Background(), "periodic storage cleanup completed",
					"removed", removed,
					"interval", interval,
					"ttl", ttl,
				)
			}
		}
	}()
}

// GetStorageStatsHandler returns storage statistics
func GetStorageStatsHandler(ctx context.Context, storageManager *storage.StorageManager) (*mcp.CallToolResult, error) {
	cvCount, jdCount, err := storageManager.GetStorageStats()
	if err != nil {
		slog.Default().ErrorContext(ctx, "failed to get storage stats",
			"error", err,
			"operation", "get_storage_stats",
		)
		return &mcp.CallToolResult{
			Content: []mcp.Content{
				&mcp.TextContent{Text: fmt.Sprintf("Error getting storage stats: %v", err)},
			},
		}, err
	}

	slog.Default().DebugContext(ctx, "storage stats retrieved via handler",
		"cv_count", cvCount,
		"jd_count", jdCount,
	)

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
