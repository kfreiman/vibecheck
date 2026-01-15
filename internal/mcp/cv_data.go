package mcp

import (
	"io"
	"os"
	"path/filepath"
)

// ReadCVFile reads a CV markdown file from the filesystem
func ReadCVFile(path string) (string, error) {
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

	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

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
