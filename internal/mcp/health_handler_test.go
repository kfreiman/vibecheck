package mcp

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kfreiman/vibecheck/internal/storage"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServer_LivenessHandler(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := &Server{logger: logger}

	req := httptest.NewRequest("GET", "/health/live", nil)
	w := httptest.NewRecorder()

	server.LivenessHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"healthy"`)
	assert.Contains(t, w.Body.String(), `"service":"vibecheck-mcp"`)
	assert.Contains(t, w.Body.String(), `"timestamp"`)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestServer_ReadinessHandler_StorageAccessible(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-test-*")
	require.NoError(t, err)
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temp dir: %v", removeErr)
		}
	}()

	sm, err := storage.NewStorageManager(storage.StorageConfig{
		BasePath: tmpDir,
	})
	require.NoError(t, err)

	// Create a mock HTTP server for langextract health check
	langextractServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer langextractServer.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := &Server{
		storageManager: sm,
		logger:         logger,
		config: Config{
			LangExtractHost: langextractServer.Listener.Addr().String(),
		},
	}

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	server.ReadinessHandler(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"healthy"`)
	assert.Contains(t, w.Body.String(), `"storage":"accessible"`)
	assert.Contains(t, w.Body.String(), `"langextract":"accessible"`)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestServer_ReadinessHandler_StorageInaccessible(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-test-*")
	require.NoError(t, err)
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temp dir: %v", removeErr)
		}
	}()

	sm, err := storage.NewStorageManager(storage.StorageConfig{
		BasePath: tmpDir,
	})
	require.NoError(t, err)

	// Remove the cv directory to make storage inaccessible
	cvPath := filepath.Join(tmpDir, "cv")
	err = os.RemoveAll(cvPath)
	require.NoError(t, err)

	// Create a mock HTTP server for langextract health check
	langextractServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy"}`))
	}))
	defer langextractServer.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := &Server{
		storageManager: sm,
		logger:         logger,
		config: Config{
			LangExtractHost: langextractServer.Listener.Addr().String(),
		},
	}

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	server.ReadinessHandler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"unhealthy"`)
	assert.Contains(t, w.Body.String(), `"storage":"inaccessible"`)
	assert.Contains(t, w.Body.String(), `"langextract":"accessible"`)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}

func TestServer_ReadinessHandler_LangExtractInaccessible(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "health-test-*")
	require.NoError(t, err)
	defer func() {
		if removeErr := os.RemoveAll(tmpDir); removeErr != nil {
			t.Logf("Failed to remove temp dir: %v", removeErr)
		}
	}()

	sm, err := storage.NewStorageManager(storage.StorageConfig{
		BasePath: tmpDir,
	})
	require.NoError(t, err)

	// Create a mock HTTP server that returns 503 for langextract
	langextractServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status":"unhealthy"}`))
	}))
	defer langextractServer.Close()

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	server := &Server{
		storageManager: sm,
		logger:         logger,
		config: Config{
			LangExtractHost: langextractServer.Listener.Addr().String(),
		},
	}

	req := httptest.NewRequest("GET", "/health/ready", nil)
	w := httptest.NewRecorder()
	server.ReadinessHandler(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), `"status":"unhealthy"`)
	assert.Contains(t, w.Body.String(), `"storage":"accessible"`)
	assert.Contains(t, w.Body.String(), `"langextract":"inaccessible"`)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
}
