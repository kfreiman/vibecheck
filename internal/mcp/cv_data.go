package mcp

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// ReadCVFile reads a CV markdown file from the filesystem
func ReadCVFile(path string) (string, error) {
	// Validate path for security
	if strings.Contains(path, "..") {
		return "", &SecurityError{
			Type:    "path_traversal",
			Details: fmt.Sprintf("path contains traversal sequence: %s", path),
		}
	}

	if strings.Contains(path, "\x00") {
		return "", &SecurityError{
			Type:    "null_byte",
			Details: "path contains null bytes",
		}
	}

	// If path is just a filename, look in current directory
	if !filepath.IsAbs(path) {
		// Check if file exists relative to current directory
		if _, err := os.Stat(path); err == nil {
			// Use as-is
		} else {
			// Try current working directory
			absPath, err := filepath.Abs(path)
			if err != nil {
				return "", err
			}
			path = absPath
		}
	}

	// #nosec G304 - path has been validated for traversal and null bytes
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Debug("error closing CV file", "error", closeErr, "path", path)
		}
	}()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}

	return string(bytes), nil
}

// FindCVFiles finds all CV markdown files in a directory
func FindCVFiles(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var cvFiles []string
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		// Look for files with .md extension or containing "cv" in name
		if filepath.Ext(name) == ".md" ||
			(len(name) >= 2 && (name[:2] == "cv" || name[:2] == "CV")) {
			cvFiles = append(cvFiles, filepath.Join(dir, name))
		}
	}

	return cvFiles, nil
}
