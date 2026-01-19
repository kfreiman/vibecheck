package storage

import (
	"os"

	"github.com/spf13/afero"
)

// FileSystem is an interface for filesystem operations
type FileSystem interface {
	// MkdirAll creates a directory and any necessary parent directories
	MkdirAll(path string, perm os.FileMode) error
	// Stat returns a FileInfo describing the named file
	Stat(name string) (os.FileInfo, error)
	// ReadFile reads the file and returns its contents
	ReadFile(name string) ([]byte, error)
	// WriteFile writes data to a file
	WriteFile(name string, data []byte, perm os.FileMode) error
	// ReadDir reads a directory and returns a list of DirEntry
	ReadDir(name string) ([]os.DirEntry, error)
	// Remove removes a named file or directory
	Remove(name string) error
	// RemoveAll removes a named directory and any children it contains
	RemoveAll(path string) error
}

// osFileSystem is the default implementation that uses the actual OS filesystem
type osFileSystem struct {
	fs afero.Fs
}

func (fs *osFileSystem) MkdirAll(path string, perm os.FileMode) error {
	return fs.fs.MkdirAll(path, perm)
}

func (fs *osFileSystem) Stat(name string) (os.FileInfo, error) {
	return fs.fs.Stat(name)
}

func (fs *osFileSystem) ReadFile(name string) ([]byte, error) {
	return afero.ReadFile(fs.fs, name)
}

func (fs *osFileSystem) WriteFile(name string, data []byte, perm os.FileMode) error {
	return afero.WriteFile(fs.fs, name, data, perm)
}

func (fs *osFileSystem) ReadDir(name string) ([]os.DirEntry, error) {
	entries, err := afero.ReadDir(fs.fs, name)
	if err != nil {
		return nil, err
	}

	// Convert []os.FileInfo to []os.DirEntry
	dirEntries := make([]os.DirEntry, len(entries))
	for i, entry := range entries {
		dirEntries[i] = &aferoDirEntry{info: entry}
	}
	return dirEntries, nil
}

func (fs *osFileSystem) Remove(name string) error {
	return fs.fs.Remove(name)
}

func (fs *osFileSystem) RemoveAll(path string) error {
	return fs.fs.RemoveAll(path)
}

// aferoDirEntry implements os.DirEntry using afero.FileInfo
type aferoDirEntry struct {
	info os.FileInfo
}

func (e *aferoDirEntry) Name() string               { return e.info.Name() }
func (e *aferoDirEntry) IsDir() bool                { return e.info.IsDir() }
func (e *aferoDirEntry) Type() os.FileMode          { return e.info.Mode().Type() }
func (e *aferoDirEntry) Info() (os.FileInfo, error) { return e.info, nil }

// NewOSFileSystem returns a FileSystem that uses the actual OS filesystem
func NewOSFileSystem() FileSystem {
	return &osFileSystem{fs: afero.NewOsFs()}
}

// NewMemMapFileSystem returns a FileSystem backed by afero's in-memory filesystem
func NewMemMapFileSystem() FileSystem {
	return &osFileSystem{fs: afero.NewMemMapFs()}
}

// NewAferoFileSystem wraps an afero.Fs in the FileSystem interface
func NewAferoFileSystem(fs afero.Fs) FileSystem {
	return &osFileSystem{fs: fs}
}
