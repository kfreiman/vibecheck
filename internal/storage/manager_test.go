package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateID(t *testing.T) {
	content1 := []byte("test content")
	content2 := []byte("test content")
	content3 := []byte("different content")

	id1 := GenerateID(content1)
	id2 := GenerateID(content2)
	id3 := GenerateID(content3)

	// Same content should produce same ID
	assert.Equal(t, id1, id2, "same content should produce same ID")

	// Different content should produce different ID
	assert.NotEqual(t, id1, id3, "different content should produce different ID")

	// ID should be valid UUID format
	assert.Len(t, id1, 36, "UUID should be 36 characters")
}

func TestGenerateIDFromString(t *testing.T) {
	str := "test string"
	id := GenerateIDFromString(str)

	assert.Len(t, id, 36, "UUID should be 36 characters")
}

func TestStorageManager_SaveAndRead(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "vibecheck-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create storage manager
	config := StorageConfig{
		BasePath:    tmpDir,
		DefaultTTL:  24 * time.Hour,
	}
	sm, err := NewStorageManager(config)
	require.NoError(t, err)

	// Test saving a document
	content := []byte("# Test CV\n\nName: John Doe\n")
	uri, err := sm.SaveDocument(DocumentTypeCV, content, "test_cv.md")
	require.NoError(t, err)

	assert.True(t, uri[:5] == "cv://", "URI should start with cv://")

	// Test reading the document
	readContent, err := sm.ReadDocument(uri)
	require.NoError(t, err)
	assert.Contains(t, string(readContent), "# Test CV")
	assert.Contains(t, string(readContent), "Name: John Doe")
}

func TestStorageManager_Deduplication(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vibecheck-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := StorageConfig{BasePath: tmpDir}
	sm, err := NewStorageManager(config)
	require.NoError(t, err)

	content := []byte("same content")

	// Save same content twice
	uri1, err := sm.SaveDocument(DocumentTypeCV, content, "file1.md")
	require.NoError(t, err)

	uri2, err := sm.SaveDocument(DocumentTypeCV, content, "file2.md")
	require.NoError(t, err)

	// Should return same URI (deduplication)
	assert.Equal(t, uri1, uri2, "same content should produce same URI")
}

func TestStorageManager_DifferentTypes(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vibecheck-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := StorageConfig{BasePath: tmpDir}
	sm, err := NewStorageManager(config)
	require.NoError(t, err)

	cvContent := []byte("CV content")
	jdContent := []byte("Job description")

	cvURI, err := sm.SaveDocument(DocumentTypeCV, cvContent, "cv.md")
	require.NoError(t, err)

	jdURI, err := sm.SaveDocument(DocumentTypeJD, jdContent, "jd.md")
	require.NoError(t, err)

	assert.Contains(t, cvURI, "cv://")
	assert.Contains(t, jdURI, "jd://")
}

func TestParseURI(t *testing.T) {
	tests := []struct {
		name        string
		uri         string
		wantType    DocumentType
		wantID      string
		wantErr     bool
	}{
		{
			name:     "valid CV URI",
			uri:      "cv://550e8400-e29b-41d4-a716-446655440000",
			wantType: DocumentTypeCV,
			wantID:   "550e8400-e29b-41d4-a716-446655440000",
			wantErr:  false,
		},
		{
			name:     "valid JD URI",
			uri:      "jd://550e8400-e29b-41d4-a716-446655440000",
			wantType: DocumentTypeJD,
			wantID:   "550e8400-e29b-41d4-a716-446655440000",
			wantErr:  false,
		},
		{
			name:    "invalid scheme",
			uri:     "file://path/to/file",
			wantErr: true,
		},
		{
			name:    "too short",
			uri:     "cv://",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			docType, id, err := ParseURI(tt.uri)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantType, docType)
			assert.Equal(t, tt.wantID, id)
		})
	}
}

func TestStorageManager_Cleanup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vibecheck-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := StorageConfig{BasePath: tmpDir}
	sm, err := NewStorageManager(config)
	require.NoError(t, err)

	// Save a document
	_, err = sm.SaveDocument(DocumentTypeCV, []byte("test"), "test.md")
	require.NoError(t, err)

	// Set file modification time to past
	cvPath := filepath.Join(tmpDir, "cv")
	entries, err := os.ReadDir(cvPath)
	require.NoError(t, err)
	for _, entry := range entries {
		if !entry.IsDir() {
			path := filepath.Join(cvPath, entry.Name())
			// Set mod time to 48 hours ago
			pastTime := time.Now().Add(-48 * time.Hour)
			os.Chtimes(path, pastTime, pastTime)
		}
	}

	// Cleanup with 24h TTL
	removed, err := sm.Cleanup(24 * time.Hour)
	require.NoError(t, err)
	assert.Equal(t, int64(1), removed)
}

func TestStorageManager_GetStorageStats(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vibecheck-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := StorageConfig{BasePath: tmpDir}
	sm, err := NewStorageManager(config)
	require.NoError(t, err)

	// Initially empty
	cvCount, jdCount, err := sm.GetStorageStats()
	require.NoError(t, err)
	assert.Equal(t, int64(0), cvCount)
	assert.Equal(t, int64(0), jdCount)

	// Add CVs
	_, err = sm.SaveDocument(DocumentTypeCV, []byte("cv1"), "cv1.md")
	require.NoError(t, err)
	_, err = sm.SaveDocument(DocumentTypeCV, []byte("cv2"), "cv2.md")
	require.NoError(t, err)

	// Add JDs
	_, err = sm.SaveDocument(DocumentTypeJD, []byte("jd1"), "jd1.md")
	require.NoError(t, err)

	cvCount, jdCount, err = sm.GetStorageStats()
	require.NoError(t, err)
	assert.Equal(t, int64(2), cvCount)
	assert.Equal(t, int64(1), jdCount)
}

func TestStorageManager_DocumentExists(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "vibecheck-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := StorageConfig{BasePath: tmpDir}
	sm, err := NewStorageManager(config)
	require.NoError(t, err)

	uri, err := sm.SaveDocument(DocumentTypeCV, []byte("test"), "test.md")
	require.NoError(t, err)

	assert.True(t, sm.DocumentExists(uri))
	assert.False(t, sm.DocumentExists("cv://00000000-0000-0000-0000-000000000000"))
}
