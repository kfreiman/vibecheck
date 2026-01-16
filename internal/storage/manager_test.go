package storage

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStorageManager_IsAccessible(t *testing.T) {
	t.Run("accessible when all directories exist", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "storage-access-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		sm, err := NewStorageManager(StorageConfig{
			BasePath: tmpDir,
		})
		require.NoError(t, err)

		assert.True(t, sm.IsAccessible())
	})

	t.Run("inaccessible when base path missing", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "storage-access-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		sm, err := NewStorageManager(StorageConfig{
			BasePath: tmpDir,
		})
		require.NoError(t, err)

		// Remove base directory
		os.RemoveAll(tmpDir)

		assert.False(t, sm.IsAccessible())
	})

	t.Run("inaccessible when cv directory missing", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "storage-access-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		sm, err := NewStorageManager(StorageConfig{
			BasePath: tmpDir,
		})
		require.NoError(t, err)

		// Remove cv directory
		cvPath := filepath.Join(tmpDir, "cv")
		os.RemoveAll(cvPath)

		assert.False(t, sm.IsAccessible())
	})

	t.Run("inaccessible when jd directory missing", func(t *testing.T) {
		tmpDir, err := os.MkdirTemp("", "storage-access-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		sm, err := NewStorageManager(StorageConfig{
			BasePath: tmpDir,
		})
		require.NoError(t, err)

		// Remove jd directory
		jdPath := filepath.Join(tmpDir, "jd")
		os.RemoveAll(jdPath)

		assert.False(t, sm.IsAccessible())
	})
}
