package mcp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// MarkitdownConverter implements DocumentConverter using the markitdown binary
type MarkitdownConverter struct {
	binaryPath string
	cacheDir   string
}

// NewMarkitdownConverter creates a new MarkitdownConverter
func NewMarkitdownConverter() (*MarkitdownConverter, error) {
	binPath, err := findMarkitdownBinary()
	if err != nil {
		return nil, err
	}

	// Create cache directory for temp files
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		cacheDir = os.TempDir()
	}
	cacheDir = filepath.Join(cacheDir, "vibecheck")
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		cacheDir = os.TempDir()
	}

	return &MarkitdownConverter{
		binaryPath: binPath,
		cacheDir:   cacheDir,
	}, nil
}

// Convert converts a file path or URL to markdown using markitdown
func (c *MarkitdownConverter) Convert(ctx context.Context, input string) (string, error) {
	info := ParseInput(input)

	switch info.Type {
	case InputTypeURL:
		return c.convertURL(ctx, info.URL.String())
	case InputTypeFile:
		// If it's a supported non-markdown file, convert it
		if IsMarkdownFile(info.Ext) {
			return ReadCVFile(info.Path)
		}
		return c.convertFile(ctx, info.Path)
	case InputTypeText:
		// Already markdown/text content
		return input, nil
	default:
		return input, nil
	}
}

// Supports checks if this converter supports the given input
func (c *MarkitdownConverter) Supports(input string) bool {
	info := ParseInput(input)
	if info.Type == InputTypeText {
		return false
	}
	if info.Type == InputTypeURL {
		return true
	}
	// Support all file types that markitdown can handle
	return IsSupportedExtension(info.Ext)
}

// IsMarkdownFile checks if the extension is a native markdown/text format
func IsMarkdownFile(ext string) bool {
	return strings.EqualFold(ext, ".md") || strings.EqualFold(ext, ".txt")
}

// convertFile converts a local file to markdown using markitdown
func (c *MarkitdownConverter) convertFile(ctx context.Context, path string) (string, error) {
	// Resolve path
	resolvedPath, err := c.resolvePath(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if file exists
	if _, err := os.Stat(resolvedPath); err != nil {
		return "", fmt.Errorf("file not found: %s", resolvedPath)
	}

	// Run markitdown
	return c.runMarkitdown(ctx, resolvedPath)
}

// convertURL downloads and converts a URL to markdown
func (c *MarkitdownConverter) convertURL(ctx context.Context, urlStr string) (string, error) {
	// Download the file first
	tmpPath, err := c.downloadFile(ctx, urlStr)
	if err != nil {
		return "", fmt.Errorf("failed to download URL: %w", err)
	}
	defer os.Remove(tmpPath)

	// Convert the downloaded file
	return c.runMarkitdown(ctx, tmpPath)
}

// downloadFile downloads a file from URL to a temp location
func (c *MarkitdownConverter) downloadFile(ctx context.Context, urlStr string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", urlStr, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	// Create temp file
	tmpFile := filepath.Join(c.cacheDir, "download_"+randomString(12))
	f, err := os.Create(tmpFile)
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer f.Close()

	// Write content
	_, err = io.Copy(f, resp.Body)
	if err != nil {
		os.Remove(tmpFile)
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}

	return tmpFile, nil
}

// runMarkitdown executes the markitdown binary
func (c *MarkitdownConverter) runMarkitdown(ctx context.Context, path string) (string, error) {
	cmd := exec.CommandContext(ctx, c.binaryPath, path)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("markitdown failed: %w (stderr: %s)", err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// resolvePath resolves a file path to absolute path
func (c *MarkitdownConverter) resolvePath(path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	// Check if file exists relative to current directory
	if _, err := os.Stat(path); err == nil {
		return filepath.Abs(path)
	}

	// Try relative to current working directory
	absPath := filepath.Join(".", path)
	if _, err := os.Stat(absPath); err == nil {
		return filepath.Abs(absPath)
	}

	return path, fmt.Errorf("file not found: %s", path)
}

// findMarkitdownBinary locates the markitdown binary in common locations
func findMarkitdownBinary() (string, error) {
	// Check if markitdown command is available in PATH
	if path, err := exec.LookPath("markitdown"); err == nil {
		return path, nil
	}

	// Check common installation paths
	paths := []string{
		"/usr/local/bin/markitdown",
		"/usr/bin/markitdown",
		"/opt/homebrew/bin/markitdown",
		"/opt/bin/markitdown",
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf(
		"markitdown binary not found in PATH. Please install it:\n" +
			"  pip install markitdown\n" +
			"  or visit: https://github.com/microsoft/markitdown",
	)
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

// ConverterErrors contains error types for conversion failures
type ConverterErrors struct {
	BinaryNotFound   error
	ConversionFailed error
	FileNotFound     error
	NetworkError     error
}

// NewConverterErrors creates a new ConverterErrors instance
func NewConverterErrors() *ConverterErrors {
	return &ConverterErrors{}
}

// WrapBinaryNotFound wraps a binary not found error
func (e *ConverterErrors) WrapBinaryNotFound(err error) error {
	return fmt.Errorf("markitdown not available: %w", err)
}

// WrapConversionFailed wraps a conversion failure error
func (e *ConverterErrors) WrapConversionFailed(input string, err error) error {
	return fmt.Errorf("conversion failed for '%s': %w", input, err)
}

// WrapFileNotFound wraps a file not found error
func (e *ConverterErrors) WrapFileNotFound(path string) error {
	return fmt.Errorf("file not found: %s", path)
}

// WrapNetworkError wraps a network error
func (e *ConverterErrors) WrapNetworkError(url string, err error) error {
	return fmt.Errorf("network error fetching '%s': %w", url, err)
}
