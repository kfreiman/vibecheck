package converter

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
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
		pw.Stop()
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
		if err := c.browser.Close(); err != nil && firstErr == nil {
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

	// Resolve and validate path
	resolvedPath, err := filepath.Abs(path)
	if err != nil {
		return "", &FileNotFoundError{Path: path}
	}

	if _, err := os.Stat(resolvedPath); err != nil {
		return "", &FileNotFoundError{Path: path}
	}

	// Read HTML file
	htmlBytes, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Path:          path,
			Hint:          "failed to read HTML file",
		}
	}

	htmlContent := string(htmlBytes)

	// Try go-readability first for static HTML
	parsedURL, _ := url.Parse("file://" + resolvedPath)
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
	if strings.HasPrefix(urlStr, "file://") {
		filePath := strings.TrimPrefix(urlStr, "file://")
		return c.convertFile(ctx, filePath)
	}

	// First, try to download and parse with go-readability (faster, no browser)
	resp, err := http.Get(urlStr)
	if err == nil && resp.StatusCode == http.StatusOK {
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if err == nil {
			parsedURL, _ := url.Parse(urlStr)
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
func (c *HTMLConverter) renderWithPlaywright(ctx context.Context, urlStr string) (string, error) {
	// Create a new page
	page, err := c.browser.NewPage()
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Hint:          "failed to create new page",
		}
	}
	defer page.Close()

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
	err = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	})
	if err != nil {
		// Continue even if wait fails - page might already be loaded
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
	parsedURL, _ := url.Parse(urlStr)
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
