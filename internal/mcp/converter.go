package mcp

import (
	"context"
	"net/url"
	"path/filepath"
	"strings"
)

// DocumentConverter defines the interface for converting documents to markdown
type DocumentConverter interface {
	// Convert takes a file path or URL and returns markdown content
	Convert(ctx context.Context, input string) (string, error)
	// Supports checks if this converter supports the given input type
	Supports(input string) bool
}

// ConversionResult represents the result of a conversion operation
type ConversionResult struct {
	Content  string
	Source   string
	MimeType string
}

// InputType represents the type of input being converted
type InputType int

const (
	InputTypeUnknown InputType = iota
	InputTypeFile
	InputTypeURL
	InputTypeText
)

// InputInfo contains parsed information about the input
type InputInfo struct {
	Type     InputType
	Path     string
	URL      *url.URL
	Ext      string
	IsRemote bool
}

// ParseInput analyzes the input to determine its type and characteristics
func ParseInput(input string) InputInfo {
	input = strings.TrimSpace(input)

	// Check if it's a URL first
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		if parsedURL, err := url.Parse(input); err == nil {
			return InputInfo{
				Type:     InputTypeURL,
				URL:      parsedURL,
				Path:     input,
				IsRemote: true,
			}
		}
	}

	// Check if it looks like a file path
	if isFilePath(input) {
		ext := strings.ToLower(filepath.Ext(input))
		return InputInfo{
			Type:     InputTypeFile,
			Path:     input,
			Ext:      ext,
			IsRemote: false,
		}
	}

	// Default to text content
	return InputInfo{
		Type:     InputTypeText,
		Path:     input,
		IsRemote: false,
	}
}

// isFilePath checks if input is likely a file path
func isFilePath(path string) bool {
	if strings.HasPrefix(path, "./") || strings.HasPrefix(path, "/") || strings.HasPrefix(path, "../") {
		return true
	}
	if strings.Contains(path, string(filepath.Separator)) {
		return true
	}
	if strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".txt") {
		return true
	}
	// Additional extensions that indicate file paths
	if strings.HasSuffix(path, ".pdf") || strings.HasSuffix(path, ".docx") ||
		strings.HasSuffix(path, ".doc") || strings.HasSuffix(path, ".pptx") ||
		strings.HasSuffix(path, ".xlsx") {
		return true
	}
	return false
}

// SupportedExtensions returns all file extensions supported by markitdown
func SupportedExtensions() []string {
	return []string{".pdf", ".docx", ".doc", ".pptx", ".xlsx", ".md", ".txt"}
}

// IsSupportedExtension checks if the extension is supported
func IsSupportedExtension(ext string) bool {
	for _, supported := range SupportedExtensions() {
		if strings.EqualFold(ext, supported) {
			return true
		}
	}
	return false
}
