package converter

// import (
// 	"bytes"
// 	"context"
// 	"fmt"
// 	"os"
// 	"os/exec"
// 	"path/filepath"
// 	"strings"
// )

// // MarkitdownConverter implements DocumentConverter using the markitdown binary
// type MarkitdownConverter struct {
// 	binaryPath string
// 	cacheDir   string
// }

// // NewMarkitdownConverter creates a new MarkitdownConverter
// func NewMarkitdownConverter() (*MarkitdownConverter, error) {
// 	binPath, err := findMarkitdownBinary()
// 	if err != nil {
// 		return nil, err
// 	}

// 	// Create cache directory for temp files
// 	cacheDir, err := os.UserCacheDir()
// 	if err != nil {
// 		cacheDir = os.TempDir()
// 	}
// 	cacheDir = filepath.Join(cacheDir, "vibecheck")
// 	if err := os.MkdirAll(cacheDir, 0755); err != nil {
// 		cacheDir = os.TempDir()
// 	}

// 	return &MarkitdownConverter{
// 		binaryPath: binPath,
// 		cacheDir:   cacheDir,
// 	}, nil
// }

// // IsAvailable checks if the markitdown binary is available
// func (c *MarkitdownConverter) IsAvailable() bool {
// 	return c.binaryPath != ""
// }

// // Convert converts a file path or URL to markdown using markitdown
// func (c *MarkitdownConverter) Convert(ctx context.Context, input string) (string, error) {
// 	info := ParseInput(input)

// 	switch info.Type {
// 	case InputTypeURL:
// 		return c.convertURL(ctx, info.URL.String())
// 	case InputTypeFile:
// 		// If it's a supported non-markdown file, convert it
// 		if IsMarkdownFile(info.Ext) {
// 			return readFile(info.Path)
// 		}
// 		return c.convertFile(ctx, info.Path)
// 	case InputTypeText:
// 		// Already markdown/text content
// 		return input, nil
// 	default:
// 		return input, nil
// 	}
// }

// // Supports checks if this converter supports the given input
// func (c *MarkitdownConverter) Supports(input string) bool {
// 	info := ParseInput(input)
// 	if info.Type == InputTypeText {
// 		return false
// 	}
// 	if info.Type == InputTypeURL {
// 		return true
// 	}
// 	// Support all file types that markitdown can handle
// 	return IsSupportedExtension(info.Ext)
// }

// // convertFile converts a local file to markdown using markitdown
// func (c *MarkitdownConverter) convertFile(ctx context.Context, path string) (string, error) {
// 	// Resolve path
// 	resolvedPath, err := c.resolvePath(path)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to resolve path: %w", err)
// 	}

// 	// Check if file exists
// 	if _, err := os.Stat(resolvedPath); err != nil {
// 		return "", &FileNotFoundError{Path: resolvedPath}
// 	}

// 	// Validate path to prevent path traversal
// 	if err := c.validatePath(resolvedPath); err != nil {
// 		return "", err
// 	}

// 	// Run markitdown
// 	return c.runMarkitdown(ctx, resolvedPath)
// }

// // convertURL downloads and converts a URL to markdown
// func (c *MarkitdownConverter) convertURL(ctx context.Context, urlStr string) (string, error) {
// 	// Download the file first
// 	tmpPath, err := DownloadFile(ctx, urlStr, c.cacheDir)
// 	if err != nil {
// 		return "", fmt.Errorf("failed to download URL: %w", err)
// 	}
// 	defer os.Remove(tmpPath)

// 	// Convert the downloaded file
// 	return c.runMarkitdown(ctx, tmpPath)
// }

// // runMarkitdown executes the markitdown binary with context support
// func (c *MarkitdownConverter) runMarkitdown(ctx context.Context, path string) (string, error) {
// 	cmd := exec.CommandContext(ctx, c.binaryPath, path)
// 	var stdout, stderr bytes.Buffer
// 	cmd.Stdout = &stdout
// 	cmd.Stderr = &stderr

// 	if err := cmd.Run(); err != nil {
// 		// Check if context was cancelled
// 		if ctx.Err() == context.DeadlineExceeded {
// 			return "", &ConversionError{
// 				OriginalError: ctx.Err(),
// 				Stderr:        stderr.String(),
// 				Hint:          "conversion timed out",
// 			}
// 		}

// 		// Return detailed error with stderr
// 		return "", &ConversionError{
// 			OriginalError: err,
// 			Stderr:        stderr.String(),
// 			Path:          path,
// 		}
// 	}

// 	return strings.TrimSpace(stdout.String()), nil
// }

// // resolvePath resolves a file path to absolute path
// func (c *MarkitdownConverter) resolvePath(path string) (string, error) {
// 	if filepath.IsAbs(path) {
// 		return path, nil
// 	}

// 	// Check if file exists relative to current directory
// 	if _, err := os.Stat(path); err == nil {
// 		return filepath.Abs(path)
// 	}

// 	// Try relative to current working directory
// 	absPath := filepath.Join(".", path)
// 	if _, err := os.Stat(absPath); err == nil {
// 		return filepath.Abs(absPath)
// 	}

// 	return path, &FileNotFoundError{Path: path}
// }

// // validatePath validates that the path doesn't contain path traversal attacks
// func (c *MarkitdownConverter) validatePath(path string) error {
// 	absPath, err := filepath.Abs(path)
// 	if err != nil {
// 		return &PathValidationError{Path: path, Reason: "failed to resolve absolute path"}
// 	}

// 	// Check for path traversal attempts
// 	if strings.Contains(absPath, "..") {
// 		return &PathValidationError{Path: path, Reason: "path traversal not allowed"}
// 	}

// 	return nil
// }

// // findMarkitdownBinary locates the markitdown binary in common locations
// func findMarkitdownBinary() (string, error) {
// 	// Check if markitdown command is available in PATH
// 	if path, err := exec.LookPath("markitdown"); err == nil {
// 		return path, nil
// 	}

// 	// Check common installation paths
// 	paths := []string{
// 		"/usr/local/bin/markitdown",
// 		"/usr/bin/markitdown",
// 		"/opt/homebrew/bin/markitdown",
// 		"/opt/bin/markitdown",
// 	}

// 	for _, path := range paths {
// 		if _, err := os.Stat(path); err == nil {
// 			return path, nil
// 		}
// 	}

// 	return "", &BinaryNotFoundError{}
// }

// // readFile reads a file from disk
// func readFile(path string) (string, error) {
// 	data, err := os.ReadFile(path)
// 	if err != nil {
// 		return "", &FileNotFoundError{Path: path}
// 	}
// 	return string(data), nil
// }

// // FileNotFoundError represents a file not found error
// type FileNotFoundError struct {
// 	Path string
// }

// func (e *FileNotFoundError) Error() string {
// 	return fmt.Sprintf("file not found: %s", e.Path)
// }

// // PathValidationError represents a path validation error
// type PathValidationError struct {
// 	Path   string
// 	Reason string
// }

// func (e *PathValidationError) Error() string {
// 	return fmt.Sprintf("path validation failed for %s: %s", e.Path, e.Reason)
// }

// // BinaryNotFoundError represents a missing binary error
// type BinaryNotFoundError struct{}

// func (e *BinaryNotFoundError) Error() string {
// 	return `markitdown binary not found. Please install it:

//   pip install markitdown
//   or visit: https://github.com/microsoft/markitdown`
// }
