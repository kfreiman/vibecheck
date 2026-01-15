package converter

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// DocumentConverter defines the interface for document conversion
type DocumentConverter interface {
	// Convert converts a document to markdown
	Convert(ctx context.Context, input string) (string, error)
	// Supports checks if the converter supports the given input
	Supports(input string) bool
	// IsAvailable checks if the converter is available
	IsAvailable() bool
}

// InputType represents the type of input
type InputType string

const (
	InputTypeFile InputType = "file"
	InputTypeURL  InputType = "url"
	InputTypeText InputType = "text"
)

// InputInfo contains parsed input information
type InputInfo struct {
	Type InputType
	Path string
	URL  *url.URL
	Ext  string
}

// ParseInput parses an input string and returns its type and info
func ParseInput(input string) InputInfo {
	info := InputInfo{}

	// Check if it's a URL
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		if parsedURL, err := url.Parse(input); err == nil {
			info.Type = InputTypeURL
			info.URL = parsedURL
			info.Ext = strings.ToLower(filepath.Ext(parsedURL.Path))
			return info
		}
	}

	// Check if it's a file path
	if _, err := os.Stat(input); err == nil {
		info.Type = InputTypeFile
		info.Path = input
		info.Ext = strings.ToLower(filepath.Ext(input))
		return info
	}

	// Default to text
	info.Type = InputTypeText
	return info
}

// IsSupportedExtension checks if the extension is supported
func IsSupportedExtension(ext string) bool {
	supported := map[string]bool{
		".pdf":  true,
		".docx": true,
		".doc":  true,
		".pptx": true,
		".ppt":  true,
		".xlsx": true,
		".xls":  true,
		".txt":  true,
		".md":   true,
		".html": true,
		".htm":  true,
	}
	return supported[strings.ToLower(ext)]
}

// IsMarkdownFile checks if the extension is a native markdown/text format
func IsMarkdownFile(ext string) bool {
	return strings.EqualFold(ext, ".md") || strings.EqualFold(ext, ".txt")
}

// DownloadFile downloads a file from URL to a temp location
func DownloadFile(ctx context.Context, url string, tempDir string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", &HTTPError{StatusCode: resp.StatusCode, URL: url}
	}

	// Create temp file
	tmpFile := filepath.Join(tempDir, "download_"+randomString(12))
	f, err := os.Create(tmpFile)
	if err != nil {
		return "", err
	}
	defer f.Close()

	// Write content
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(tmpFile)
		return "", err
	}

	return tmpFile, nil
}

// randomString generates a random string for temp files
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[i%len(letters)]
	}
	return string(b)
}
