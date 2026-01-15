package storage

import (
	"crypto/sha1"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
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
	BasePath    string
	DefaultTTL  time.Duration
}

// StorageManager handles document storage with UUID-based naming
type StorageManager struct {
	basePath   string
	defaultTTL time.Duration
}

// NewStorageManager creates a new storage manager
func NewStorageManager(config StorageConfig) (*StorageManager, error) {
	// Set defaults
	if config.BasePath == "" {
		config.BasePath = "./storage"
	}
	if config.DefaultTTL == 0 {
		config.DefaultTTL = 24 * time.Hour
	}

	// Create directory structure
	cvPath := filepath.Join(config.BasePath, string(DocumentTypeCV))
	jdPath := filepath.Join(config.BasePath, string(DocumentTypeJD))

	for _, path := range []string{cvPath, jdPath} {
		if err := os.MkdirAll(path, 0755); err != nil {
			return nil, &StorageError{
				Operation: "init - create directory",
				Path:      path,
				Err:       err,
			}
		}
	}

	return &StorageManager{
		basePath:   config.BasePath,
		defaultTTL: config.DefaultTTL,
	}, nil
}

// GetPath returns the storage path for a document type
func (sm *StorageManager) GetPath(docType DocumentType) string {
	return filepath.Join(sm.basePath, string(docType))
}

// GenerateID generates a UUID v5 based on content hash
func GenerateID(content []byte) string {
	hash := sha1.Sum(content)
	// Use a namespace UUID for CV/JD documents
	namespace := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8") // DNS namespace as base
	return uuid.NewSHA1(namespace, hash[:]).String()
}

// GenerateIDFromString generates a UUID v5 from a string
func GenerateIDFromString(content string) string {
	return GenerateID([]byte(content))
}

// SaveDocument saves a document to storage and returns its URI
func (sm *StorageManager) SaveDocument(docType DocumentType, content []byte, originalFilename string) (string, error) {
	id := GenerateID(content)

	// Check if file already exists (deduplication)
	ext := filepath.Ext(originalFilename)
	if ext == "" {
		ext = ".md"
	}
	filename := id + ext
	path := filepath.Join(sm.GetPath(docType), filename)

	// Check if exists
	if _, err := os.Stat(path); err == nil {
		// File already exists, return existing URI
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

	if err := os.WriteFile(path, []byte(fullContent), 0644); err != nil {
		return "", &StorageError{
			Operation: "save document",
			Path:      path,
			Err:       err,
		}
	}

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
	entries, err := os.ReadDir(dir)
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
	path, err := sm.GetDocumentPath(uri)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, &StorageError{
			Operation: "read document",
			Path:      path,
			Err:       err,
		}
	}

	return content, nil
}

// DocumentExists checks if a document exists in storage
func (sm *StorageManager) DocumentExists(uri string) bool {
	path, err := sm.GetDocumentPath(uri)
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// Cleanup removes documents older than the specified TTL
func (sm *StorageManager) Cleanup(ttl time.Duration) (int64, error) {
	if ttl == 0 {
		ttl = sm.defaultTTL
	}

	cutoff := time.Now().Add(-ttl)
	var removed int64

	for _, docType := range []DocumentType{DocumentTypeCV, DocumentTypeJD} {
		dir := sm.GetPath(docType)
		entries, err := os.ReadDir(dir)
		if err != nil {
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
				if err := os.Remove(filepath.Join(dir, entry.Name())); err == nil {
					removed++
				}
			}
		}
	}

	return removed, nil
}

// GetStorageStats returns statistics about the storage
func (sm *StorageManager) GetStorageStats() (cvCount, jdCount int64, err error) {
	for _, docType := range []DocumentType{DocumentTypeCV, DocumentTypeJD} {
		dir := sm.GetPath(docType)
		entries, err := os.ReadDir(dir)
		if err != nil {
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

	return cvCount, jdCount, nil
}
