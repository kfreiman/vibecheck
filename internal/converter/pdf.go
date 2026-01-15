package converter

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PDFConverter extracts text from PDF files using pure Go
type PDFConverter struct{}

// NewPDFConverter creates a new PDFConverter
func NewPDFConverter() *PDFConverter {
	return &PDFConverter{}
}

// IsAvailable always returns true - pure Go has no external deps
func (c *PDFConverter) IsAvailable() bool {
	return true
}

// Convert extracts text from a PDF file
func (c *PDFConverter) Convert(ctx context.Context, input string) (string, error) {
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

// convertFile extracts text from a PDF file
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

	// Open PDF
	f, reader, err := pdf.Open(resolvedPath)
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Path:          path,
			Hint:          "failed to open PDF",
		}
	}
	defer f.Close()

	// Extract text from all pages
	text, err := reader.GetPlainText()
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Path:          path,
			Hint:          "failed to extract text from PDF",
		}
	}

	content, err := io.ReadAll(text)
	if err != nil {
		return "", &ConversionError{
			OriginalError: err,
			Path:          path,
			Hint:          "failed to read PDF text",
		}
	}

	return string(content), nil
}
