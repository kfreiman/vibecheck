package ingest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kfreiman/vibecheck/internal/converter"
	"github.com/kfreiman/vibecheck/internal/storage"
)

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		shouldErr bool
	}{
		{
			name:      "valid relative path",
			path:      "test.md",
			shouldErr: false,
		},
		{
			name:      "valid path with subdirectory",
			path:      "docs/test.md",
			shouldErr: false,
		},
		{
			name:      "path traversal attempt",
			path:      "../../../etc/passwd",
			shouldErr: true,
		},
		{
			name:      "path with null byte",
			path:      "test.md\x00malicious",
			shouldErr: true,
		},
		{
			name:      "URL should pass",
			path:      "https://example.com/test.pdf",
			shouldErr: false,
		},
		{
			name:      "text content should pass",
			path:      "Some text content",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePath(tt.path)
			if tt.shouldErr {
				assert.Error(t, err)
				var secErr *SecurityError
				assert.ErrorAs(t, err, &secErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestExtractFilename(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected string
	}{
		{
			name:     "simple filename",
			path:     "test.md",
			expected: "test.md",
		},
		{
			name:     "path with directory",
			path:     "docs/test.md",
			expected: "test.md",
		},
		{
			name:     "full path",
			path:     "/home/user/test.md",
			expected: "test.md",
		},
		{
			name:     "URL with filename",
			path:     "https://example.com/test.pdf",
			expected: "test.pdf",
		},
		{
			name:     "URL without explicit filename",
			path:     "https://example.com/",
			expected: "",
		},
		{
			name:     "raw text content",
			path:     "Some text content with multiple lines",
			expected: "Some text content with multiple lines",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFilename(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDocumentIngestor_Ingest(t *testing.T) {
	// Create temp storage directory
	tmpDir, err := os.MkdirTemp("", "ingest-test-*")
	require.NoError(t, err)
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temp dir: %v", removeErr)
		}
	}()

	// Create storage manager
	config := storage.StorageConfig{
		BasePath:   tmpDir,
		DefaultTTL: 24 * time.Hour,
	}
	storageManager, err := storage.NewStorageManager(config)
	require.NoError(t, err)

	// Create converter
	documentConverter := converter.NewPDFConverter()

	// Create ingestor
	ingestor := NewIngestor(storageManager, documentConverter)

	// Test case 1: Ingest raw text as CV
	t.Run("IngestRawTextCV", func(t *testing.T) {
		cvContent := "# Test CV\n\nName: John Doe\nEmail: john@example.com\n"
		uri, err := ingestor.Ingest(context.Background(), cvContent, "cv")
		require.NoError(t, err)
		assert.Contains(t, uri, "cv://")

		// Verify document exists
		exists := storageManager.DocumentExists(uri)
		assert.True(t, exists, "Document should exist in storage")
	})

	// Test case 2: Ingest raw text as JD
	t.Run("IngestRawTextJD", func(t *testing.T) {
		jdContent := "# Job Description\n\nLooking for Go developer\n"
		uri, err := ingestor.Ingest(context.Background(), jdContent, "jd")
		require.NoError(t, err)
		assert.Contains(t, uri, "jd://")

		// Verify document exists
		exists := storageManager.DocumentExists(uri)
		assert.True(t, exists, "Document should exist in storage")
	})

	// Test case 3: Ingest markdown file
	t.Run("IngestMarkdownFile", func(t *testing.T) {
		// Create a test file
		testFile := filepath.Join(tmpDir, "test_cv.md")
		cvContent := "# Test CV\n\nName: Jane Smith\n"
		err := os.WriteFile(testFile, []byte(cvContent), 0600)
		require.NoError(t, err)

		uri, err := ingestor.Ingest(context.Background(), testFile, "cv")
		require.NoError(t, err)
		assert.Contains(t, uri, "cv://")

		// Verify document exists
		exists := storageManager.DocumentExists(uri)
		assert.True(t, exists, "Document should exist in storage")
	})

	// Test case 4: Invalid document type
	t.Run("InvalidDocumentType", func(t *testing.T) {
		cvContent := "# Test CV\n\nName: John Doe\n"
		_, err := ingestor.Ingest(context.Background(), cvContent, "invalid")
		require.Error(t, err)

		var valErr *ValidationError
		assert.ErrorAs(t, err, &valErr)
		assert.Equal(t, "type", valErr.Field)
	})

	// Test case 5: Path traversal attempt
	t.Run("PathTraversalAttempt", func(t *testing.T) {
		_, err := ingestor.Ingest(context.Background(), "../../../etc/passwd", "cv")
		require.Error(t, err)

		var secErr *SecurityError
		assert.ErrorAs(t, err, &secErr)
		assert.Equal(t, "path_traversal", secErr.Type)
	})
}

func TestDocumentIngestor_Deduplication(t *testing.T) {
	// Create temp storage directory
	tmpDir, err := os.MkdirTemp("", "ingest-dedup-test-*")
	require.NoError(t, err)
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temp dir: %v", removeErr)
		}
	}()

	// Create storage manager
	config := storage.StorageConfig{
		BasePath:   tmpDir,
		DefaultTTL: 24 * time.Hour,
	}
	storageManager, err := storage.NewStorageManager(config)
	require.NoError(t, err)

	// Create ingestor
	ingestor := NewIngestor(storageManager, nil)

	// Ingest same content twice
	sameContent := "# Duplicate CV\n\nName: Same Person\n"

	// First ingestion
	uri1, err := ingestor.Ingest(context.Background(), sameContent, "cv")
	require.NoError(t, err)

	// Get stats after first ingestion
	cvCount1, _, err := storageManager.GetStorageStats()
	require.NoError(t, err)
	assert.Equal(t, int64(1), cvCount1, "Should have 1 CV after first ingestion")

	// Second ingestion with same content
	uri2, err := ingestor.Ingest(context.Background(), sameContent, "cv")
	require.NoError(t, err)

	// Get stats after second ingestion
	cvCount2, _, err := storageManager.GetStorageStats()
	require.NoError(t, err)

	// Count should not increase (deduplication)
	assert.Equal(t, int64(1), cvCount2, "Should still have 1 CV (deduplication)")

	// URIs should be the same
	assert.Equal(t, uri1, uri2, "Same content should return same URI")
}

func TestDocumentIngestor_EmptyPath(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ingest-empty-test-*")
	require.NoError(t, err)
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temp dir: %v", removeErr)
		}
	}()

	config := storage.StorageConfig{BasePath: tmpDir}
	storageManager, err := storage.NewStorageManager(config)
	require.NoError(t, err)

	ingestor := NewIngestor(storageManager, nil)

	// Empty path is valid - it's treated as raw text
	uri, err := ingestor.Ingest(context.Background(), "", "cv")
	require.NoError(t, err)
	assert.Contains(t, uri, "cv://")
}
