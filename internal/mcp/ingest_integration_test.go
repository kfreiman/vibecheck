package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kfreiman/vibecheck/internal/ingest"
	"github.com/kfreiman/vibecheck/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestIngestIntegration tests the full flow: storage creation, ingestion, and verification
func TestIngestIntegration(t *testing.T) {
	// Use a unique temp directory for this test
	tmpDir, err := os.MkdirTemp("", "vibecheck-integration-*")
	require.NoError(t, err)
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temp dir: %v", removeErr)
		}
	}()

	// Override the storage path
	originalStoragePath := os.Getenv("VIBECHECK_STORAGE_PATH")
	err = os.Setenv("VIBECHECK_STORAGE_PATH", tmpDir)
	require.NoError(t, err)
	defer func() {
		if originalStoragePath != "" {
			err = os.Setenv("VIBECHECK_STORAGE_PATH", originalStoragePath)
			if err != nil {
				t.Logf("Failed to restore VIBECHECK_STORAGE_PATH: %v", err)
			}
		} else {
			err = os.Unsetenv("VIBECHECK_STORAGE_PATH")
			if err != nil {
				t.Logf("Failed to unset VIBECHECK_STORAGE_PATH: %v", err)
			}
		}
	}()

	// Initialize storage manager with the temp directory
	config := storage.StorageConfig{
		BasePath:   tmpDir,
		DefaultTTL: 24 * time.Hour,
	}
	storageManager, err := storage.NewStorageManager(config)
	require.NoError(t, err)

	// Verify initial state
	cvCount, jdCount, err := storageManager.GetStorageStats()
	require.NoError(t, err)
	assert.Equal(t, int64(0), cvCount, "Should start with 0 CVs")
	assert.Equal(t, int64(0), jdCount, "Should start with 0 JDs")

	// Create ingestor
	ingestor := ingest.NewIngestor(storageManager, nil)

	// Create ingest tool
	ingestTool := NewIngestDocumentTool(ingestor)

	// Test 1: Ingest CV with raw text
	t.Run("IngestCVRawText", func(t *testing.T) {
		cvContent := "# Software Engineer CV\n\nName: Jane Doe\nEmail: jane@example.com\nPhone: 555-123-4567\nSkills: Go, Python, AWS\n"
		args := map[string]interface{}{
			"path": cvContent,
			"type": "cv",
		}
		argsBytes, err := json.Marshal(args)
		require.NoError(t, err)

		result, err := ingestTool.Call(context.Background(), &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{Arguments: argsBytes},
		})

		require.NoError(t, err)
		require.NotNil(t, result)

		// Check result contains URI
		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok, "expected TextContent")
		assert.Contains(t, textContent.Text, "cv://", "Should return CV URI")

		// Verify storage stats increased
		cvCount, jdCount, err := storageManager.GetStorageStats()
		require.NoError(t, err)
		assert.Equal(t, int64(1), cvCount, "Should have 1 CV")
		assert.Equal(t, int64(0), jdCount, "Should have 0 JDs")

		// Verify file exists and has correct format
		cvDir := filepath.Join(tmpDir, "cv")
		entries, err := os.ReadDir(cvDir)
		require.NoError(t, err)
		require.Len(t, entries, 1, "Should have exactly one file")

		// Read the file
		filePath := filepath.Join(cvDir, entries[0].Name())
		// Validate the constructed path doesn't contain traversal (defensive check)
		cleanPath := filepath.Clean(filePath)
		require.NotContains(t, cleanPath, "..", "File path should not contain traversal")
		content, err := os.ReadFile(cleanPath)
		require.NoError(t, err)

		// Check for frontmatter
		contentStr := string(content)
		assert.Contains(t, contentStr, "---", "Should have frontmatter")
		assert.Contains(t, contentStr, "id:", "Frontmatter should contain ID")
		assert.Contains(t, contentStr, "type: cv", "Frontmatter should have type cv")

		// Verify content is preserved (no redaction in v1)
		assert.Contains(t, contentStr, "jane@example.com", "Original email should be preserved")
		assert.Contains(t, contentStr, "555-123-4567", "Original phone should be preserved")
	})

	// Test 2: Ingest JD with raw text
	t.Run("IngestJDRawText", func(t *testing.T) {
		jdContent := "# Senior Go Developer Position\n\nRequirements: 5+ years Go, Kubernetes, AWS\nContact: hiring@company.com\n"
		args := map[string]interface{}{
			"path": jdContent,
			"type": "jd",
		}
		argsBytes, err := json.Marshal(args)
		require.NoError(t, err)

		_, err = ingestTool.Call(context.Background(), &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{Arguments: argsBytes},
		})

		require.NoError(t, err)

		// Verify storage stats
		cvCount, jdCount, err := storageManager.GetStorageStats()
		require.NoError(t, err)
		assert.Equal(t, int64(1), cvCount, "Should still have 1 CV")
		assert.Equal(t, int64(1), jdCount, "Should now have 1 JD")
	})

	// Test 3: Ingest markdown file
	t.Run("IngestMarkdownFile", func(t *testing.T) {
		// Create a temp file
		testFile := filepath.Join(tmpDir, "test_resume.md")
		fileContent := "# Test Resume\n\nName: John Smith\nEmail: john.smith@test.com\n"
		err := os.WriteFile(testFile, []byte(fileContent), 0600)
		require.NoError(t, err)

		args := map[string]interface{}{
			"path": testFile,
			"type": "cv",
		}
		argsBytes, err := json.Marshal(args)
		require.NoError(t, err)

		_, err = ingestTool.Call(context.Background(), &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{Arguments: argsBytes},
		})

		require.NoError(t, err)

		// Verify storage
		cvCount, jdCount, err := storageManager.GetStorageStats()
		require.NoError(t, err)
		assert.Equal(t, int64(2), cvCount, "Should now have 2 CVs")
		assert.Equal(t, int64(1), jdCount, "Should still have 1 JD")
	})

	// Test 4: Deduplication - same content should not create new file
	t.Run("Deduplication", func(t *testing.T) {
		sameContent := "# Duplicate CV\n\nName: Same Person\n"

		// Ingest first time
		args1 := map[string]interface{}{
			"path": sameContent,
			"type": "cv",
		}
		argsBytes1, err := json.Marshal(args1)
		require.NoError(t, err)
		_, err = ingestTool.Call(context.Background(), &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{Arguments: argsBytes1},
		})
		require.NoError(t, err)

		cvCount1, _, err := storageManager.GetStorageStats()
		require.NoError(t, err)

		// Ingest second time with same content
		_, err = ingestTool.Call(context.Background(), &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{Arguments: argsBytes1},
		})
		require.NoError(t, err)

		cvCount2, _, err := storageManager.GetStorageStats()
		require.NoError(t, err)

		// Count should not increase
		assert.Equal(t, cvCount1, cvCount2, "Same content should not create duplicate")
	})
}

// TestMCPServerIngest tests the server's ability to handle ingest requests
func TestMCPServerIngest(t *testing.T) {
	// Create temp storage
	tmpDir, err := os.MkdirTemp("", "vibecheck-server-test-*")
	require.NoError(t, err)
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temp dir: %v", removeErr)
		}
	}()

	// Set env var for server to use this dir
	err = os.Setenv("VIBECHECK_STORAGE_PATH", tmpDir)
	require.NoError(t, err)
	defer func() {
		err = os.Unsetenv("VIBECHECK_STORAGE_PATH")
		if err != nil {
			t.Logf("Failed to unset VIBECHECK_STORAGE_PATH: %v", err)
		}
	}()

	// We can't easily test the full server with HTTP transport in a unit test,
	// but we can verify that the handlers are properly initialized
	storageManager, err := storage.NewStorageManager(storage.StorageConfig{
		BasePath: tmpDir,
	})
	require.NoError(t, err)

	// Create ingestor
	ingestor := ingest.NewIngestor(storageManager, nil)

	// Create handlers like the server does
	ingestTool := NewIngestDocumentTool(ingestor)
	cleanupTool := NewCleanupStorageTool(storageManager)
	storageHandler := NewStorageResourceHandler(storageManager)

	// Verify handlers are not nil
	require.NotNil(t, ingestTool)
	require.NotNil(t, cleanupTool)
	require.NotNil(t, storageHandler)

	// Ingest a document first so we have resources to list
	cvContent := "# Test CV\n\nName: Test User\nEmail: test@example.com\n"
	args := map[string]interface{}{
		"path": cvContent,
		"type": "cv",
	}
	argsBytes, err := json.Marshal(args)
	require.NoError(t, err)

	_, err = ingestTool.Call(context.Background(), &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: argsBytes},
	})
	require.NoError(t, err)

	// Verify storage resources can be listed
	resources := storageHandler.ListResources()
	assert.NotEmpty(t, resources, "Should have some resources after ingestion")

	// Verify resource templates exist
	templates := storageHandler.ListResourceTemplates()
	assert.Len(t, templates, 2, "Should have 2 templates (cv:// and jd://)")
}
