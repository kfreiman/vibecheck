package converter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"codeberg.org/readeck/go-readability/v2"
	"github.com/playwright-community/playwright-go"
)

// HTMLConverter extracts text from HTML files and URLs using go-readability and playwright
type HTMLConverter struct {
	pw      *playwright.Playwright
	browser playwright.Browser
}

// NewHTMLConverter creates a new HTMLConverter with playwright initialized
func NewHTMLConverter() (*HTMLConverter, error) {
	// Initialize playwright
	pw, err := playwright.Run()
	if err != nil {
		return nil, &ConversionError{
			OriginalError: err,
			Hint:          "failed to initialize playwright",
		}
	}

	// Launch Chromium browser
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		if stopErr := pw.Stop(); stopErr != nil {
			slog.Debug("error stopping playwright", "error", stopErr)
		}
		return nil, &ConversionError{
			OriginalError: err,
			Hint:          "failed to launch chromium browser",
		}
	}

	return &HTMLConverter{
		pw:      pw,
		browser: browser,
	}, nil
}

// IsAvailable returns true if playwright was initialized successfully
func (c *HTMLConverter) IsAvailable() bool {
	return c.pw != nil && c.browser != nil
}

// Convert extracts text from HTML file or URL
func (c *HTMLConverter) Convert(ctx context.Context, input string) (string, error) {
	if !c.IsAvailable() {
		return "", &ConversionError{
			Hint: "HTML converter not available - playwright not initialized",
		}
	}

	// Check for path traversal before any processing
	if strings.Contains(input, "..") {
		return "", &PathValidationError{Path: input, Reason: "path traversal not allowed"}
	}

	// Check for URL first (including file://)
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "file://") {
		return c.convertURL(ctx, input)
	}

	// Check for HTML file extension (even if file doesn't exist yet)
	ext := strings.ToLower(filepath.Ext(input))
	if ext == ".html" || ext == ".htm" {
		return c.convertFile(ctx, input)
	}

	// Use ParseInput for other cases
	info := ParseInput(input)

	switch info.Type {
	case InputTypeFile:
		return c.convertFile(ctx, info.Path)
	case InputTypeURL:
		return c.convertURL(ctx, info.URL.String())
	default:
		return "", &ConversionError{
			Hint: fmt.Sprintf("unsupported input type: %s", info.Type),
		}
	}
}

// Supports checks if the converter supports the given input
func (c *HTMLConverter) Supports(input string) bool {
	// Check for URL first (doesn't require file existence)
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return true
	}

	// Check for file extensions (even if file doesn't exist yet)
	ext := strings.ToLower(filepath.Ext(input))
	if ext == ".html" || ext == ".htm" {
		return true
	}

	// Check if it's an existing file
	info := ParseInput(input)
	if info.Type == InputTypeText {
		return false
	}
	return info.Type == InputTypeURL
}

// Close cleans up the playwright browser and instance
func (c *HTMLConverter) Close() error {
	var firstErr error
	if c.browser != nil {
		if err := c.browser.Close(); err != nil {
			firstErr = err
		}
	}
	if c.pw != nil {
		if err := c.pw.Stop(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// convertFile extracts text from an HTML file
func (c *HTMLConverter) convertFile(ctx context.Context, path string) (string, error) {
	// Check for path traversal before resolving the path
	if strings.Contains(path, "..") {
		return "", &PathValidationError{Path: path, Reason: "path traversal not allowed"}
	}

	// Check for null bytes
	if strings.Contains(path, "\x00") {
		return "", &PathValidationError{Path: path, Reason: "null bytes not allowed"}
	}

	// Resolve and validate path
	resolvedPath, absErr := filepath.Abs(path)
	if absErr != nil {
		return "", &FileNotFoundError{Path: path}
	}

	// Ensure the resolved path is still within expected bounds (no symlink attacks)
	// For file operations, we rely on the OS to prevent actual traversal
	if _, statErr := os.Stat(resolvedPath); statErr != nil {
		return "", &FileNotFoundError{Path: path}
	}

	// Read HTML file
	// #nosec G304 - path has been validated for traversal and null bytes
	htmlBytes, err := os.ReadFile(resolvedPath)
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Path:          path,
			Hint:          "failed to read HTML file",
		}
	}

	htmlContent := string(htmlBytes)

	// Try go-readability first for static HTML
	parsedURL, parseErr := url.Parse("file://" + resolvedPath)
	if parseErr != nil {
		return "", &ConversionError{
			OriginalError: parseErr,
			Path:          path,
			Hint:          "failed to parse file URL",
		}
	}
	article, err := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if err == nil && article.Node != nil {
		// Successfully extracted content with go-readability
		var buf bytes.Buffer
		if err := article.RenderText(&buf); err == nil {
			content := buf.String()
			if content != "" {
				return content, nil
			}
		}
	}

	// Fallback to playwright for dynamic content or if go-readability fails
	return c.renderWithPlaywright(ctx, "file://"+resolvedPath)
}

// convertURL extracts text from an HTML URL
func (c *HTMLConverter) convertURL(ctx context.Context, urlStr string) (string, error) {
	// Handle file:// URLs specially
	if filePath, ok := strings.CutPrefix(urlStr, "file://"); ok {
		return c.convertFile(context.Background(), filePath)
	}

	// Validate URL before making HTTP request
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Hint:          fmt.Sprintf("failed to parse URL: %s", urlStr),
		}
	}

	// Check for potentially dangerous URLs (e.g., file://, gopher://, etc.)
	// Only allow http and https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", &ConversionError{
			Hint: fmt.Sprintf("unsupported URL scheme: %s", parsedURL.Scheme),
		}
	}

	// First, try to download and parse with go-readability (faster, no browser)
	// #nosec G107 - URL has been validated for http/https scheme only
	resp, err := http.Get(urlStr)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				slog.Debug("error closing response body", "error", closeErr)
			}
		}()
		body, err := io.ReadAll(resp.Body)
		if err == nil {
			article, err := readability.FromReader(bytes.NewReader(body), parsedURL)
			if err == nil && article.Node != nil {
				var buf bytes.Buffer
				if err := article.RenderText(&buf); err == nil {
					content := buf.String()
					if content != "" {
						return content, nil
					}
				}
			}
		}
	}

	// Fallback to playwright for dynamic content or if download/go-readability fails
	return c.renderWithPlaywright(ctx, urlStr)
}

// renderWithPlaywright uses playwright to render the page and extract content
func (c *HTMLConverter) renderWithPlaywright(_ context.Context, urlStr string) (string, error) {
	// Create a new page
	page, err := c.browser.NewPage()
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Hint:          "failed to create new page",
		}
	}
	defer func() {
		if closeErr := page.Close(); closeErr != nil {
			slog.Debug("error closing page", "error", closeErr)
		}
	}()

	// Navigate to the URL with timeout
	_, err = page.Goto(urlStr, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateNetworkidle,
		Timeout:   playwright.Float(30000),
	})
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Hint:          fmt.Sprintf("failed to navigate to %s", urlStr),
		}
	}

	// Wait for page to be fully loaded
	if waitErr := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); waitErr != nil {
		// Continue even if wait fails - page might already be loaded
		slog.Debug("wait for load state failed, continuing anyway", "error", waitErr)
	}

	// Get the full HTML content
	html, err := page.Content()
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Hint:          "failed to get page content",
		}
	}

	// Try to extract main content using go-readability
	parsedURL, parseErr := url.Parse(urlStr)
	if parseErr != nil {
		return "", &ConversionError{
			OriginalError: parseErr,
			Hint:          fmt.Sprintf("failed to parse URL %s", urlStr),
		}
	}
	article, err := readability.FromReader(strings.NewReader(html), parsedURL)
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Hint:          "failed to parse HTML with go-readability",
		}
	}

	if article.Node == nil {
		return "", &ConversionError{
			Hint: "no readable content found in page",
		}
	}

	var buf bytes.Buffer
	if err := article.RenderText(&buf); err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Hint:          "failed to render readable content",
		}
	}

	content := buf.String()
	if content == "" {
		return "", &ConversionError{
			Hint: "no readable content found in page",
		}
	}

	return content, nil
}
