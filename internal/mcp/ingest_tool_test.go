package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kfreiman/vibecheck/internal/converter"
	"github.com/kfreiman/vibecheck/internal/storage"
)

func TestIngestDocumentTool_Call(t *testing.T) {
	// Create temp storage directory
	tmpDir, err := os.MkdirTemp("", "vibecheck-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create storage manager
	config := storage.StorageConfig{
		BasePath:   tmpDir,
		DefaultTTL: 24 * time.Hour,
	}
	storageManager, err := storage.NewStorageManager(config)
	require.NoError(t, err)

	// Create converter (optional - will be nil if markitdown not found)
	documentConverter := converter.NewPDFConverter()

	// Create ingest tool
	ingestTool := NewIngestDocumentTool(storageManager, documentConverter)

	// Test case 1: Ingest markdown file
	t.Run("IngestMarkdownFile", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tmpDir, "test_cv.md")
		cvContent := "# Test CV\n\nName: John Doe\nEmail: john@example.com\nPhone: +1234567890\n"
		err := os.WriteFile(testFile, []byte(cvContent), 0644)
		require.NoError(t, err)

		args := map[string]interface{}{
			"path": testFile,
			"type": "cv",
		}
		argsBytes, _ := json.Marshal(args)

		request := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			},
		}

		result, err := ingestTool.Call(context.Background(), request)
		require.NoError(t, err, "Expected no error from ingest tool")
		require.NotNil(t, result)

		// Check that result contains URI
		textContent := result.Content[0].(*mcp.TextContent).Text
		assert.Contains(t, textContent, "URI:", "Result should contain URI")
		assert.Contains(t, textContent, "cv://", "URI should start with cv://")

		// Extract URI from result text
		// Format: "URI: cv://<uuid>"
		// Find the URI line
		var uri string
		lines := []string{
			"URI: cv://",
			"cv://",
		}
		for _, line := range lines {
			if idx := len(line); idx > 0 {
				for i := 0; i < len(textContent)-idx; i++ {
					if textContent[i:i+idx] == line {
						// Extract until end of line or space
						end := i + idx
						for end < len(textContent) && textContent[end] != '\n' && textContent[end] != ' ' {
							end++
						}
						uri = textContent[i:end]
						break
					}
				}
			}
		}

		// Verify document was actually saved
		if uri != "" {
			exists := storageManager.DocumentExists(uri)
			assert.True(t, exists, "Document should exist in storage after ingestion")
		}
	})

	// Test case 2: Ingest raw text
	t.Run("IngestRawText", func(t *testing.T) {
		cvContent := "# Test CV from raw text\n\nName: Jane Smith\n"

		args := map[string]interface{}{
			"path": cvContent,
			"type": "cv",
		}
		argsBytes, _ := json.Marshal(args)

		request := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			},
		}

		result, err := ingestTool.Call(context.Background(), request)
		require.NoError(t, err, "Expected no error from ingest tool")
		require.NotNil(t, result)

		textContent := result.Content[0].(*mcp.TextContent).Text
		assert.Contains(t, textContent, "URI:", "Result should contain URI")

		// Check storage stats
		cvCount, _, err := storageManager.GetStorageStats()
		require.NoError(t, err)
		assert.Greater(t, cvCount, int64(0), "Should have at least one CV in storage")
	})

	// Test case 3: Ingest JD
	t.Run("IngestJD", func(t *testing.T) {
		jdContent := "# Job Description\n\nLooking for Go developer\n"

		args := map[string]interface{}{
			"path": jdContent,
			"type": "jd",
		}
		argsBytes, _ := json.Marshal(args)

		request := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			},
		}

		result, err := ingestTool.Call(context.Background(), request)
		require.NoError(t, err, "Expected no error from ingest tool")
		require.NotNil(t, result)

		textContent := result.Content[0].(*mcp.TextContent).Text
		assert.Contains(t, textContent, "jd://", "JD URI should start with jd://")

		// Check storage stats
		_, jdCount, err := storageManager.GetStorageStats()
		require.NoError(t, err)
		assert.Greater(t, jdCount, int64(0), "Should have at least one JD in storage")
	})
}

func TestStorageAfterIngest(t *testing.T) {
	// This test reproduces the exact bug report scenario
	tmpDir, err := os.MkdirTemp("", "vibecheck-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := storage.StorageConfig{BasePath: tmpDir}
	storageManager, err := storage.NewStorageManager(config)
	require.NoError(t, err)

	// Get initial stats
	initialCV, initialJD, err := storageManager.GetStorageStats()
	require.NoError(t, err)
	assert.Equal(t, int64(0), initialCV, "Should start with 0 CVs")
	assert.Equal(t, int64(0), initialJD, "Should start with 0 JDs")

	// Create ingest tool
	ingestTool := NewIngestDocumentTool(storageManager, nil)

	// Ingest a CV
	cvContent := "# CV Test\n\nSoftware Engineer with 5 years experience"
	args := map[string]interface{}{
		"path": cvContent,
		"type": "cv",
	}
	argsBytes, _ := json.Marshal(args)

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: argsBytes,
		},
	}

	result, err := ingestTool.Call(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)

	// Get stats after ingestion
	afterCV, afterJD, err := storageManager.GetStorageStats()
	require.NoError(t, err)

	// This is the bug: afterCV should be 1 but might still be 0
	t.Logf("Before: CV=%d, JD=%d", initialCV, initialJD)
	t.Logf("After: CV=%d, JD=%d", afterCV, afterJD)

	assert.Equal(t, int64(1), afterCV, "Should have 1 CV in storage after ingestion")
	assert.Equal(t, int64(0), afterJD, "Should still have 0 JDs")

	// List files in storage
	cvDir := filepath.Join(tmpDir, "cv")
	entries, err := os.ReadDir(cvDir)
	require.NoError(t, err)
	t.Logf("Files in CV dir: %v", entries)
}
