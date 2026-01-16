package mcp

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kfreiman/vibecheck/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLivenessHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()

	LivenessHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"healthy"`)
	assert.Contains(t, w.Body.String(), `"service":"vibecheck-mcp"`)
	assert.Contains(t, w.Body.String(), `"timestamp"`)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestReadinessHandler_StorageAccessible(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	sm, err := storage.NewStorageManager(storage.StorageConfig{
		BasePath: tmpDir,
	})
	require.NoError(t, err)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	handler := ReadinessHandlerFunc(sm)
	handler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"healthy"`)
	assert.Contains(t, w.Body.String(), `"storage":"accessible"`)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestReadinessHandler_StorageInaccessible(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	sm, err := storage.NewStorageManager(storage.StorageConfig{
		BasePath: tmpDir,
	})
	require.NoError(t, err)

	// Remove the cv directory to make storage inaccessible
	cvPath := filepath.Join(tmpDir, "cv")
	os.RemoveAll(cvPath)

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	handler := ReadinessHandlerFunc(sm)
	handler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"unhealthy"`)
	assert.Contains(t, w.Body.String(), `"storage":"inaccessible"`)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}
