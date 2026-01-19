package storage

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"time"
)

// StorageError represents a storage-related failure
type StorageError struct {
	Operation string
	Path      string
	Err       error
}

func (e *StorageError) Error() string {
	msg := fmt.Sprintf("storage error during %s", e.Operation)
	if e.Path != "" {
		msg += fmt.Sprintf(" (path: %s)", e.Path)
	}
	if e.Err != nil {
		msg += fmt.Sprintf(": %v", e.Err)
	}
	return msg
}

func (e *StorageError) Unwrap() error {
	return e.Err
}

// IsRetryable indicates if this storage error is retryable
func (e *StorageError) IsRetryable() bool {
	// Storage errors are typically transient (I/O issues, locks, permissions)
	return true
}

// DocumentType represents the type of document being stored
type DocumentType string

const (
	DocumentTypeCV DocumentType = "cv"
	DocumentTypeJD DocumentType = "jd"
)

// StorageConfig holds configuration for the storage manager
type StorageConfig struct {
	BasePath   string
	DefaultTTL time.Duration
	Logger     *slog.Logger // Optional: custom logger (defaults to slog zerolog)
	FileSystem FileSystem   // Optional: custom filesystem (defaults to OS filesystem)
}

// StorageManager handles document storage with UUID-based naming
type StorageManager struct {
	basePath   string
	defaultTTL time.Duration
	logger     *slog.Logger
	fs         FileSystem
}

// NewStorageManager creates a new storage manager
func NewStorageManager(config StorageConfig) (*StorageManager, error) {
	ctx := context.Background()

	// Set defaults
	if config.BasePath == "" {
		config.BasePath = "./storage"
	}
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 24 * time.Hour
	}

	if config.FileSystem == nil {
		config.FileSystem = NewOSFileSystem()
	}

	if config.Logger == nil {
		config.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}

	// Create directory structure
	cvPath := filepath.Join(config.BasePath, string(DocumentTypeCV))
	jdPath := filepath.Join(config.BasePath, string(DocumentTypeJD))

	for _, path := range []string{cvPath, jdPath} {
		if err := config.FileSystem.MkdirAll(path, 0755); err != nil {
			config.Logger.ErrorContext(ctx, "failed to create storage directory",
				"error", err,
				"path", path,
				"operation", "init",
			)
			return nil, &StorageError{
				Operation: "init - create directory",
				Path:      path,
				Err:       err,
			}
		}
	}

	config.Logger.InfoContext(ctx, "storage manager initialized",
		"base_path", config.BasePath,
		"default_ttl", config.DefaultTTL,
	)

	return &StorageManager{
		basePath:   config.BasePath,
		defaultTTL: config.DefaultTTL,
		logger:     config.Logger,
		fs:         config.FileSystem,
	}, nil
}

// GetPath returns the storage path for a document type
func (sm *StorageManager) GetPath(docType DocumentType) string {
	return filepath.Join(sm.basePath, string(docType))
}

// GenerateID generates a content-based identifier using SHA-256
// Returns a hex-encoded hash for deterministic content identification
func GenerateID(content []byte) string {
	hash := sha256.Sum256(content)
	// Return hex-encoded hash for deterministic, unique content identification
	return fmt.Sprintf("%x", hash)
}

// GenerateIDFromString generates a UUID v5 from a string
func GenerateIDFromString(content string) string {
	return GenerateID([]byte(content))
}

// SaveDocument saves a document to storage and returns its URI
func (sm *StorageManager) SaveDocument(docType DocumentType, content []byte, originalFilename string) (string, error) {
	ctx := context.Background()
	id := GenerateID(content)

	// Check if file already exists (deduplication)
	ext := filepath.Ext(originalFilename)
	if ext == "" {
		ext = ".md"
	}
	filename := id + ext
	path := filepath.Join(sm.GetPath(docType), filename)

	// Check if exists
	if _, err := sm.fs.Stat(path); err == nil {
		// File already exists, return existing URI
		sm.logger.DebugContext(ctx, "document already exists (deduplication)",
			"doc_type", docType,
			"id", id,
			"path", path,
		)
		return fmt.Sprintf("%s://%s", docType, id), nil
	}

	// Write file with frontmatter
	frontmatter := fmt.Sprintf(`---
id: %s
original_filename: %s
ingested_at: %s
type: %s
---
`, id, originalFilename, time.Now().UTC().Format(time.RFC3339), docType)

	fullContent := frontmatter + string(content)

	if err := sm.fs.WriteFile(path, []byte(fullContent), 0644); err != nil {
		sm.logger.ErrorContext(ctx, "failed to save document",
			"error", err,
			"doc_type", docType,
			"id", id,
			"path", path,
			"operation", "save",
		)
		return "", &StorageError{
			Operation: "save document",
			Path:      path,
			Err:       err,
		}
	}

	sm.logger.InfoContext(ctx, "document saved",
		"doc_type", docType,
		"id", id,
		"path", path,
		"filename", originalFilename,
	)

	return fmt.Sprintf("%s://%s", docType, id), nil
}

// SaveDocumentWithRedaction saves a document with PII redaction applied
func (sm *StorageManager) SaveDocumentWithRedaction(docType DocumentType, content []byte, originalFilename string, redactFunc func([]byte) []byte) (string, error) {
	redactedContent := redactFunc(content)
	return sm.SaveDocument(docType, redactedContent, originalFilename)
}

// GetDocumentPath returns the file path for a given URI
func (sm *StorageManager) GetDocumentPath(uri string) (string, error) {
	docType, id, err := ParseURI(uri)
	if err != nil {
		return "", err
	}

	// Find file with any extension
	dir := sm.GetPath(docType)
	entries, err := sm.fs.ReadDir(dir)
	if err != nil {
		return "", &StorageError{
			Operation: "read directory",
			Path:      dir,
			Err:       err,
		}
	}

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) != "" {
			// Check if filename starts with the ID
			nameWithoutExt := entry.Name()[:len(id)]
			if nameWithoutExt == id || len(entry.Name()) >= len(id) && entry.Name()[:len(id)] == id {
				return filepath.Join(dir, entry.Name()), nil
			}
		}
	}

	return "", &StorageError{
		Operation: "find document",
		Err:       fmt.Errorf("document not found: %s", uri),
	}
}

// ParseURI parses a URI into document type and ID
func ParseURI(uri string) (DocumentType, string, error) {
	if len(uri) < 6 {
		return "", "", &StorageError{
			Operation: "parse URI",
			Err:       fmt.Errorf("URI too short: %s", uri),
		}
	}

	scheme := uri[:5]
	var docType DocumentType

	switch scheme {
	case "cv://":
		docType = DocumentTypeCV
	case "jd://":
		docType = DocumentTypeJD
	default:
		return "", "", &StorageError{
			Operation: "parse URI",
			Err:       fmt.Errorf("unsupported URI scheme: %s", scheme),
		}
	}

	id := uri[5:]
	return docType, id, nil
}

// ReadDocument reads a document from storage
func (sm *StorageManager) ReadDocument(uri string) ([]byte, error) {
	ctx := context.Background()
	path, err := sm.GetDocumentPath(uri)
	if err != nil {
		sm.logger.ErrorContext(ctx, "failed to get document path",
			"error", err,
			"uri", uri,
			"operation", "read",
		)
		return nil, err
	}

	content, err := sm.fs.ReadFile(path)
	if err != nil {
		sm.logger.ErrorContext(ctx, "failed to read document",
			"error", err,
			"path", path,
			"uri", uri,
			"operation", "read",
		)
		return nil, &StorageError{
			Operation: "read document",
			Path:      path,
			Err:       err,
		}
	}

	sm.logger.DebugContext(ctx, "document read",
		"uri", uri,
		"path", path,
	)

	return content, nil
}

// DocumentExists checks if a document exists in storage
func (sm *StorageManager) DocumentExists(uri string) bool {
	path, err := sm.GetDocumentPath(uri)
	if err != nil {
		return false
	}
	_, err = sm.fs.Stat(path)
	return err == nil
}

// Cleanup removes documents older than the specified TTL
func (sm *StorageManager) Cleanup(ttl time.Duration) (int64, error) {
	ctx := context.Background()
	if ttl == 0 {
		ttl = sm.defaultTTL
	}

	cutoff := time.Now().Add(-ttl)
	var removed int64

	for _, docType := range []DocumentType{DocumentTypeCV, DocumentTypeJD} {
		dir := sm.GetPath(docType)
		entries, err := sm.fs.ReadDir(dir)
		if err != nil {
			sm.logger.ErrorContext(ctx, "failed to read directory for cleanup",
				"error", err,
				"dir", dir,
				"doc_type", docType,
			)
			continue
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			info, err := entry.Info()
			if err != nil {
				continue
			}

			if info.ModTime().Before(cutoff) {
				if err := sm.fs.Remove(filepath.Join(dir, entry.Name())); err == nil {
					removed++
				}
			}
		}
	}

	sm.logger.InfoContext(ctx, "storage cleanup completed",
		"removed", removed,
		"ttl", ttl,
	)

	return removed, nil
}

// GetStorageStats returns statistics about the storage
func (sm *StorageManager) GetStorageStats() (cvCount, jdCount int64, err error) {
	ctx := context.Background()
	for _, docType := range []DocumentType{DocumentTypeCV, DocumentTypeJD} {
		dir := sm.GetPath(docType)
		entries, err := sm.fs.ReadDir(dir)
		if err != nil {
			sm.logger.ErrorContext(ctx, "failed to read directory for stats",
				"error", err,
				"dir", dir,
				"doc_type", docType,
			)
			return 0, 0, err
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				switch docType {
				case DocumentTypeCV:
					cvCount++
				case DocumentTypeJD:
					jdCount++
				}
			}
		}
	}

	sm.logger.DebugContext(ctx, "storage stats retrieved",
		"cv_count", cvCount,
		"jd_count", jdCount,
	)

	return cvCount, jdCount, nil
}

// IsAccessible checks if storage is accessible and directories exist
func (sm *StorageManager) IsAccessible() bool {
	// Check base path exists
	if _, err := sm.fs.Stat(sm.basePath); err != nil {
		return false
	}
	// Check cv directory
	cvPath := sm.GetPath(DocumentTypeCV)
	if _, err := sm.fs.Stat(cvPath); err != nil {
		return false
	}
	// Check jd directory
	jdPath := sm.GetPath(DocumentTypeJD)
	if _, err := sm.fs.Stat(jdPath); err != nil {
		return false
	}
	return true
}

// ListAllDocuments returns all stored document UUIDs by type
func (sm *StorageManager) ListAllDocuments() (cvUUIDs, jdUUIDs []string, err error) {
	ctx := context.Background()
	for _, docType := range []DocumentType{DocumentTypeCV, DocumentTypeJD} {
		dir := sm.GetPath(docType)
		entries, err := sm.fs.ReadDir(dir)
		if err != nil {
			sm.logger.ErrorContext(ctx, "failed to read directory for listing",
				"error", err,
				"dir", dir,
				"doc_type", docType,
			)
			return nil, nil, &StorageError{
				Operation: "list documents",
				Path:      dir,
				Err:       err,
			}
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				// Extract UUID from filename (remove extension)
				name := entry.Name()
				ext := filepath.Ext(name)
				if ext != "" {
					uuid := name[:len(name)-len(ext)]
					switch docType {
					case DocumentTypeCV:
						cvUUIDs = append(cvUUIDs, uuid)
					case DocumentTypeJD:
						jdUUIDs = append(jdUUIDs, uuid)
					}
				}
			}
		}
	}

	sm.logger.DebugContext(ctx, "listed all documents",
		"cv_count", len(cvUUIDs),
		"jd_count", len(jdUUIDs),
	)

	return cvUUIDs, jdUUIDs, nil
}
