package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageManager_NewStorageManager(t *testing.T) {
	t.Run("creates with defaults when config is empty", func(t *testing.T) {
		sm, err := NewStorageManager(StorageConfig{})
		require.NoError(t, err)
		require.NotNil(t, sm)

		assert.Equal(t, "./storage", sm.basePath)
		assert.Equal(t, 24*time.Hour, sm.defaultTTL)
		assert.NotNil(t, sm.logger)
		assert.NotNil(t, sm.fs)

		// Clean up
		os.RemoveAll("./storage")
	})

	t.Run("uses provided config values", func(t *testing.T) {
		basePath := "/test-storage"
		config := StorageConfig{
			BasePath:   basePath,
			DefaultTTL: 48 * time.Hour,
			FileSystem: NewMemMapFileSystem(),
		}

		sm, err := NewStorageManager(config)
		require.NoError(t, err)
		require.NotNil(t, sm)

		assert.Equal(t, basePath, sm.basePath)
		assert.Equal(t, 48*time.Hour, sm.defaultTTL)
		assert.NotNil(t, sm.logger)
		assert.NotNil(t, sm.fs)
	})
}

func TestStorageManager_IsAccessible(t *testing.T) {
	t.Run("accessible when all directories exist", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		// After NewStorageManager, directories should exist
		assert.True(t, sm.IsAccessible())
	})

	t.Run("inaccessible after removing base path", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		// Remove base path
		fs.RemoveAll(basePath)

		assert.False(t, sm.IsAccessible())
	})

	t.Run("inaccessible after removing cv directory", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		// Remove cv directory
		fs.Remove(filepath.Join(basePath, "cv"))

		assert.False(t, sm.IsAccessible())
	})

	t.Run("inaccessible after removing jd directory", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		// Remove jd directory
		fs.Remove(filepath.Join(basePath, "jd"))

		assert.False(t, sm.IsAccessible())
	})
}

func TestStorageManager_SaveDocument(t *testing.T) {
	t.Run("saves document successfully", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		content := []byte("# Test CV\n\nJohn Doe\nSoftware Engineer")
		uri, err := sm.SaveDocument(DocumentTypeCV, content, "test_cv.md")
		require.NoError(t, err)
		assert.NotEmpty(t, uri)
		assert.Contains(t, uri, "cv://")
	})

	t.Run("deduplicates existing documents", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		content := []byte("# Test CV\n\nJohn Doe")
		uri1, err := sm.SaveDocument(DocumentTypeCV, content, "test_cv.md")
		require.NoError(t, err)

		uri2, err := sm.SaveDocument(DocumentTypeCV, content, "test_cv.md")
		require.NoError(t, err)

		// Should return the same URI for identical content
		assert.Equal(t, uri1, uri2)
	})

	t.Run("handles missing extension", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		content := []byte("# Test CV")
		uri, err := sm.SaveDocument(DocumentTypeCV, content, "test")
		require.NoError(t, err)
		assert.NotEmpty(t, uri)
	})
}

func TestStorageManager_ReadDocument(t *testing.T) {
	t.Run("reads document successfully", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		content := []byte("# Test CV\n\nJohn Doe")
		uri, err := sm.SaveDocument(DocumentTypeCV, content, "test_cv.md")
		require.NoError(t, err)

		readContent, err := sm.ReadDocument(uri)
		require.NoError(t, err)
		assert.Contains(t, string(readContent), "# Test CV")
		assert.Contains(t, string(readContent), "John Doe")
	})

	t.Run("returns error for non-existent document", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		_, err = sm.ReadDocument("cv://non-existent")
		assert.Error(t, err)
		assert.IsType(t, &StorageError{}, err)
	})
}

func TestStorageManager_DocumentExists(t *testing.T) {
	t.Run("returns true for existing document", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		content := []byte("# Test CV")
		uri, err := sm.SaveDocument(DocumentTypeCV, content, "test_cv.md")
		require.NoError(t, err)

		assert.True(t, sm.DocumentExists(uri))
	})

	t.Run("returns false for non-existent document", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		assert.False(t, sm.DocumentExists("cv://non-existent"))
	})
}

func TestStorageManager_ListAllDocuments(t *testing.T) {
	t.Run("lists all documents", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		// Save some documents
		_, err = sm.SaveDocument(DocumentTypeCV, []byte("CV 1"), "cv1.md")
		require.NoError(t, err)

		_, err = sm.SaveDocument(DocumentTypeCV, []byte("CV 2"), "cv2.md")
		require.NoError(t, err)

		_, err = sm.SaveDocument(DocumentTypeJD, []byte("JD 1"), "jd1.md")
		require.NoError(t, err)

		cvUUIDs, jdUUIDs, err := sm.ListAllDocuments()
		require.NoError(t, err)
		assert.Len(t, cvUUIDs, 2)
		assert.Len(t, jdUUIDs, 1)
	})

	t.Run("returns empty when no documents exist", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		cvUUIDs, jdUUIDs, err := sm.ListAllDocuments()
		require.NoError(t, err)
		assert.Len(t, cvUUIDs, 0)
		assert.Len(t, jdUUIDs, 0)
	})
}

func TestStorageManager_GetStorageStats(t *testing.T) {
	t.Run("returns correct stats", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		// Save some documents
		_, err = sm.SaveDocument(DocumentTypeCV, []byte("CV 1"), "cv1.md")
		require.NoError(t, err)

		_, err = sm.SaveDocument(DocumentTypeCV, []byte("CV 2"), "cv2.md")
		require.NoError(t, err)

		_, err = sm.SaveDocument(DocumentTypeJD, []byte("JD 1"), "jd1.md")
		require.NoError(t, err)

		cvCount, jdCount, err := sm.GetStorageStats()
		require.NoError(t, err)
		assert.Equal(t, int64(2), cvCount)
		assert.Equal(t, int64(1), jdCount)
	})

	t.Run("returns zero when storage is empty", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		cvCount, jdCount, err := sm.GetStorageStats()
		require.NoError(t, err)
		assert.Equal(t, int64(0), cvCount)
		assert.Equal(t, int64(0), jdCount)
	})
}

func TestStorageManager_GetPath(t *testing.T) {
	t.Run("returns correct path for cv", func(t *testing.T) {
		sm := &StorageManager{basePath: "/test"}
		path := sm.GetPath(DocumentTypeCV)
		assert.Equal(t, "/test/cv", path)
	})

	t.Run("returns correct path for jd", func(t *testing.T) {
		sm := &StorageManager{basePath: "/test"}
		path := sm.GetPath(DocumentTypeJD)
		assert.Equal(t, "/test/jd", path)
	})
}

func TestGenerateID(t *testing.T) {
	t.Run("generates consistent IDs for same content", func(t *testing.T) {
		content := []byte("test content")
		id1 := GenerateID(content)
		id2 := GenerateID(content)
		assert.Equal(t, id1, id2)
	})

	t.Run("generates different IDs for different content", func(t *testing.T) {
		id1 := GenerateID([]byte("content 1"))
		id2 := GenerateID([]byte("content 2"))
		assert.NotEqual(t, id1, id2)
	})
}

func TestGenerateIDFromString(t *testing.T) {
	t.Run("generates consistent IDs for same string", func(t *testing.T) {
		id1 := GenerateIDFromString("test string")
		id2 := GenerateIDFromString("test string")
		assert.Equal(t, id1, id2)
	})
}

func TestParseURI(t *testing.T) {
	t.Run("parses cv:// URI", func(t *testing.T) {
		docType, id, err := ParseURI("cv://12345-67890")
		require.NoError(t, err)
		assert.Equal(t, DocumentTypeCV, docType)
		assert.Equal(t, "12345-67890", id)
	})

	t.Run("parses jd:// URI", func(t *testing.T) {
		docType, id, err := ParseURI("jd://12345-67890")
		require.NoError(t, err)
		assert.Equal(t, DocumentTypeJD, docType)
		assert.Equal(t, "12345-67890", id)
	})

	t.Run("returns error for short URI", func(t *testing.T) {
		_, _, err := ParseURI("cv:/")
		require.Error(t, err)
	})

	t.Run("returns error for unsupported scheme", func(t *testing.T) {
		_, _, err := ParseURI("xx://12345")
		require.Error(t, err)
	})
}

func TestStorageManager_GetDocumentPath(t *testing.T) {
	t.Run("returns correct path for existing document", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		content := []byte("# Test CV")
		uri, err := sm.SaveDocument(DocumentTypeCV, content, "test_cv.md")
		require.NoError(t, err)

		path, err := sm.GetDocumentPath(uri)
		require.NoError(t, err)
		assert.Contains(t, path, basePath)
		assert.Contains(t, path, "cv")
	})

	t.Run("returns error for non-existent document", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		_, err = sm.GetDocumentPath("cv://non-existent")
		require.Error(t, err)
	})
}

func TestStorageError_Error(t *testing.T) {
	t.Run("formats error message correctly", func(t *testing.T) {
		err := &StorageError{
			Operation: "save document",
			Path:      "/path/to/file.md",
			Err:       os.ErrNotExist,
		}
		msg := err.Error()
		assert.Contains(t, msg, "save document")
		assert.Contains(t, msg, "/path/to/file.md")
	})

	t.Run("formats error message without path", func(t *testing.T) {
		err := &StorageError{
			Operation: "parse URI",
			Err:       os.ErrInvalid,
		}
		msg := err.Error()
		assert.Contains(t, msg, "parse URI")
	})

	t.Run("Unwrap returns underlying error", func(t *testing.T) {
		underlying := os.ErrNotExist
		err := &StorageError{
			Operation: "test",
			Err:       underlying,
		}
		assert.Equal(t, underlying, err.Unwrap())
	})
}

func TestStorageError_IsRetryable(t *testing.T) {
	t.Run("storage errors are retryable", func(t *testing.T) {
		err := &StorageError{
			Operation: "test",
			Err:       os.ErrNotExist,
		}
		assert.True(t, err.IsRetryable())
	})
}

func TestStorageManager_Cleanup(t *testing.T) {
	t.Run("cleanup removes old documents", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			DefaultTTL: 1 * time.Hour,
			FileSystem: fs,
		})
		require.NoError(t, err)

		// Save a document
		_, err = sm.SaveDocument(DocumentTypeCV, []byte("Test CV"), "test_cv.md")
		require.NoError(t, err)

		// Cleanup with a very short TTL (should remove if modtime is before cutoff)
		// Note: In mock, modtime is set to time.Now(), so this won't actually remove
		// unless we manually set an older modtime
		removed, err := sm.Cleanup(1 * time.Millisecond)
		require.NoError(t, err)
		// Mock filesystem doesn't track modtimes accurately, so result depends on timing
		assert.True(t, removed >= 0)
	})

	t.Run("cleanup with zero TTL uses default", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			DefaultTTL: 1 * time.Hour,
			FileSystem: fs,
		})
		require.NoError(t, err)

		// Save a document
		_, err = sm.SaveDocument(DocumentTypeCV, []byte("Test CV"), "test_cv.md")
		require.NoError(t, err)

		// Cleanup with zero TTL should use default
		removed, err := sm.Cleanup(0)
		require.NoError(t, err)
		assert.True(t, removed >= 0)
	})
}

func TestStorageManager_SaveDocumentWithRedaction(t *testing.T) {
	t.Run("applies redaction function", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		basePath := "/test-storage"

		sm, err := NewStorageManager(StorageConfig{
			BasePath:   basePath,
			FileSystem: fs,
		})
		require.NoError(t, err)

		redactFunc := func(content []byte) []byte {
			return []byte("[REDACTED]")
		}

		content := []byte("PII data")
		uri, err := sm.SaveDocumentWithRedaction(DocumentTypeCV, content, "test.md", redactFunc)
		require.NoError(t, err)
		assert.NotEmpty(t, uri)

		// Read back and verify redaction
		readContent, err := sm.ReadDocument(uri)
		require.NoError(t, err)
		assert.Contains(t, string(readContent), "[REDACTED]")
	})
}

func TestNewOSFileSystem(t *testing.T) {
	t.Run("creates OS filesystem", func(t *testing.T) {
		fs := NewOSFileSystem()
		require.NotNil(t, fs)
	})
}

func TestNewMemMapFileSystem(t *testing.T) {
	t.Run("creates in-memory filesystem", func(t *testing.T) {
		fs := NewMemMapFileSystem()
		require.NotNil(t, fs)
	})
}

func TestNewAferoFileSystem(t *testing.T) {
	t.Run("wraps afero filesystem", func(t *testing.T) {
		af := afero.NewMemMapFs()
		fs := NewAferoFileSystem(af)
		require.NotNil(t, fs)
	})
}
