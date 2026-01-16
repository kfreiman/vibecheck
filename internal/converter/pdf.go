package converter

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/klippa-app/go-pdfium"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
)

// PDFConverter extracts text from PDF files using go-pdfium (webassembly)
type PDFConverter struct {
	pool     pdfium.Pool
	instance pdfium.Pdfium
}

// NewPDFConverter creates a new PDFConverter
func NewPDFConverter() *PDFConverter {
	// Initialize PDFium in webassembly mode (pure Go, no native deps)
	pool, err := webassembly.Init(webassembly.Config{
		MinIdle:  1,
		MaxIdle:  1,
		MaxTotal: 1,
	})
	if err != nil {
		// If initialization fails, return a converter with nil instance
		// This will be handled by IsAvailable() returning false
		return &PDFConverter{pool: nil, instance: nil}
	}

	instance, err := pool.GetInstance(time.Second * 30)
	if err != nil {
		// If initialization fails, return a converter with nil instance
		// This will be handled by IsAvailable() returning false
		pool.Close()
		return &PDFConverter{pool: nil, instance: nil}
	}
	return &PDFConverter{pool: pool, instance: instance}
}

// IsAvailable returns true if PDFium was initialized successfully
func (c *PDFConverter) IsAvailable() bool {
	return c.instance != nil
}

// Convert extracts text from a PDF file
func (c *PDFConverter) Convert(ctx context.Context, input string) (string, error) {
	if !c.IsAvailable() {
		return "", &ConversionError{
			Hint: "PDFium instance not available",
		}
	}

	info := ParseInput(input)

	switch info.Type {
	case InputTypeFile:
		return c.convertFile(info.Path)
	case InputTypeText:
		return input, nil
	default:
		return input, nil
	}
}

// Supports checks if the converter supports the given input
func (c *PDFConverter) Supports(input string) bool {
	info := ParseInput(input)
	if info.Type == InputTypeText {
		return false
	}
	return strings.EqualFold(info.Ext, ".pdf")
}

// Close cleans up the PDFium instance and pool
func (c *PDFConverter) Close() error {
	var firstErr error
	if c.instance != nil {
		if err := c.instance.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if c.pool != nil {
		if err := c.pool.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// convertFile extracts text from a PDF file using go-pdfium
func (c *PDFConverter) convertFile(path string) (string, error) {
	// Resolve and validate path
	resolvedPath, err := filepath.Abs(path)
	if err != nil {
		return "", &FileNotFoundError{Path: path}
	}

	if _, err := os.Stat(resolvedPath); err != nil {
		return "", &FileNotFoundError{Path: path}
	}

	// Check for path traversal
	if strings.Contains(resolvedPath, "..") {
		return "", &PathValidationError{Path: path, Reason: "path traversal not allowed"}
	}

	// Read PDF file into memory
	pdfBytes, err := ioutil.ReadFile(resolvedPath)
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Path:          path,
			Hint:          "failed to read PDF file",
		}
	}

	// Open PDF document
	doc, err := c.instance.OpenDocument(&requests.OpenDocument{
		File: &pdfBytes,
	})
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Path:          path,
			Hint:          "failed to open PDF document",
		}
	}
	defer c.instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	// Get page count
	pageCount, err := c.instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{
		Document: doc.Document,
	})
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Path:          path,
			Hint:          "failed to get page count",
		}
	}

	// Extract text from all pages
	var allText strings.Builder
	for i := 0; i < pageCount.PageCount; i++ {
		// Get text from page using GetPageText (simpler API)
		pageText, err := c.instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{
				ByIndex: &requests.PageByIndex{
					Document: doc.Document,
					Index:    i,
				},
			},
		})
		if err != nil {
			return "", &ConversionError{
				OriginalError: err,
				Path:          path,
				PageNum:       i + 1,
				Hint:          "failed to extract text from page",
			}
		}

		// Add page text to result
		if pageText.Text != "" {
			if allText.Len() > 0 {
				allText.WriteString("\n\n")
			}
			allText.WriteString(pageText.Text)
		}
	}

	return allText.String(), nil
}
