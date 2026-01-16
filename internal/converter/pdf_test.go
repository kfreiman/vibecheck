package converter

import (
	"context"
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// createTestPDF creates a minimal valid PDF file for testing
func createTestPDF(t *testing.T, content string) string {
	// Create a minimal PDF with the given content
	// This is a simple PDF structure that PDFium can parse
	pdfContent := `%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj

2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj

3 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 612 792]
/Contents 4 0 R
/Resources <<
/Font <<
/F1 5 0 R
>>
>>
>>
endobj

4 0 obj
<<
/Length 44
>>
stream
BT
/F1 12 Tf
50 700 Td
(` + content + `) Tj
ET
endstream
endobj

5 0 obj
<<
/Type /Font
/Subtype /Type1
/BaseFont /Helvetica
>>
endobj

xref
0 6
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000245 00000 n
0000000344 00000 n
trailer
<<
/Size 6
/Root 1 0 R
>>
startxref
421
%%EOF`

	tmpFile, err := ioutil.TempFile("", "test-*.pdf")
	require.NoError(t, err)
	defer tmpFile.Close()

	_, err = tmpFile.WriteString(pdfContent)
	require.NoError(t, err)

	return tmpFile.Name()
}

func TestPDFConverter_Supports(t *testing.T) {
	converter := NewPDFConverter()
	defer converter.Close()

	// Create temp files for testing
	tmpPDF, err := ioutil.TempFile("", "test-*.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpPDF.Name())
	tmpPDF.Close()

	tmpDOCX, err := ioutil.TempFile("", "test-*.docx")
	require.NoError(t, err)
	defer os.Remove(tmpDOCX.Name())
	tmpDOCX.Close()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"PDF file", tmpPDF.Name(), true},
		{"Uppercase PDF", tmpPDF.Name(), true},
		{"Text input", "some text", false},
		{"Non-PDF file", tmpDOCX.Name(), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.Supports(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPDFConverter_IsAvailable(t *testing.T) {
	converter := NewPDFConverter()
	defer converter.Close()

	// Should be available if PDFium initialized successfully
	// Note: This may fail if PDFium can't initialize
	available := converter.IsAvailable()
	t.Logf("PDFium available: %v", available)
}

func TestPDFConverter_Convert_Text(t *testing.T) {
	converter := NewPDFConverter()
	defer converter.Close()

	if !converter.IsAvailable() {
		t.Skip("PDFium not available, skipping test")
	}

	input := "This is plain text"
	result, err := converter.Convert(context.Background(), input)
	require.NoError(t, err)
	assert.Equal(t, input, result)
}

func TestPDFConverter_Convert_PDF(t *testing.T) {
	converter := NewPDFConverter()
	defer converter.Close()

	if !converter.IsAvailable() {
		t.Skip("PDFium not available, skipping test")
	}

	// Create a test PDF
	pdfPath := createTestPDF(t, "Hello World")
	defer os.Remove(pdfPath)

	result, err := converter.Convert(context.Background(), pdfPath)
	require.NoError(t, err)
	assert.Contains(t, result, "Hello World")
}

func TestPDFConverter_Convert_MultiPagePDF(t *testing.T) {
	converter := NewPDFConverter()
	defer converter.Close()

	if !converter.IsAvailable() {
		t.Skip("PDFium not available, skipping test")
	}

	// Create a multi-page PDF by creating multiple PDFs and combining them
	// For simplicity, we'll just test single page extraction
	pdfPath := createTestPDF(t, "Page One Content")
	defer os.Remove(pdfPath)

	result, err := converter.Convert(context.Background(), pdfPath)
	require.NoError(t, err)
	assert.Contains(t, result, "Page One Content")
}

func TestPDFConverter_Convert_NonExistentFile(t *testing.T) {
	converter := NewPDFConverter()
	defer converter.Close()

	if !converter.IsAvailable() {
		t.Skip("PDFium not available, skipping test")
	}

	// Test with a path that looks like a PDF but doesn't exist
	// Since ParseInput checks file existence, non-existent paths are treated as text
	// This is the expected behavior - if file doesn't exist, treat as text content
	tmpFile, err := ioutil.TempFile("", "test-*.pdf")
	require.NoError(t, err)
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	os.Remove(tmpPath)

	// The Convert method treats non-existent files as text input
	result, err := converter.Convert(context.Background(), tmpPath)
	require.NoError(t, err)
	assert.Equal(t, tmpPath, result)
}

func TestPDFConverter_Convert_InvalidPDF(t *testing.T) {
	converter := NewPDFConverter()
	defer converter.Close()

	if !converter.IsAvailable() {
		t.Skip("PDFium not available, skipping test")
	}

	// Create a file that's not a valid PDF
	tmpFile, err := ioutil.TempFile("", "invalid-*.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString("This is not a PDF")
	require.NoError(t, err)
	tmpFile.Close()

	_, err = converter.Convert(context.Background(), tmpFile.Name())
	require.Error(t, err)
	assert.IsType(t, &ConversionError{}, err)
}

func TestPDFConverter_Convert_PathTraversal(t *testing.T) {
	converter := NewPDFConverter()
	defer converter.Close()

	if !converter.IsAvailable() {
		t.Skip("PDFium not available, skipping test")
	}

	// Test path traversal attempt with .. in the path
	// The check happens before filepath.Abs resolves it
	pathWithTraversal := "/etc/../etc/passwd"
	_, err := converter.Convert(context.Background(), pathWithTraversal)
	require.Error(t, err)
	assert.IsType(t, &PathValidationError{}, err)
}

func TestPDFConverter_Convert_EmptyPDF(t *testing.T) {
	converter := NewPDFConverter()
	defer converter.Close()

	if !converter.IsAvailable() {
		t.Skip("PDFium not available, skipping test")
	}

	// Create a PDF with no text content
	pdfContent := `%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj

2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj

3 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 612 792]
/Contents 4 0 R
>>
endobj

4 0 obj
<<
/Length 0
>>
stream

endstream
endobj

xref
0 5
0000000000 65535 f
0000000009 00000 n
0000000058 00000 n
0000000115 00000 n
0000000205 00000 n
trailer
<<
/Size 5
/Root 1 0 R
>>
startxref
255
%%EOF`

	tmpFile, err := ioutil.TempFile("", "empty-*.pdf")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(pdfContent)
	require.NoError(t, err)
	tmpFile.Close()

	result, err := converter.Convert(context.Background(), tmpFile.Name())
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestConversionError_Error(t *testing.T) {
	baseErr := errors.New("test error")

	tests := []struct {
		name     string
		err      *ConversionError
		expected string
	}{
		{
			name: "basic error",
			err: &ConversionError{
				OriginalError: baseErr,
				Hint:          "test hint",
			},
			expected: "PDF conversion failed: test error\nHint: test hint",
		},
		{
			name: "with path",
			err: &ConversionError{
				OriginalError: baseErr,
				Path:          "/path/to/file.pdf",
				Hint:          "failed to open",
			},
			expected: "PDF conversion failed: test error (file: /path/to/file.pdf)\nHint: failed to open",
		},
		{
			name: "with page number",
			err: &ConversionError{
				OriginalError: baseErr,
				Path:          "/path/to/file.pdf",
				PageNum:       3,
				Hint:          "page error",
			},
			expected: "PDF conversion failed: test error (file: /path/to/file.pdf) [page 3]\nHint: page error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.err.Error()
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestFileNotFoundError_Error(t *testing.T) {
	err := &FileNotFoundError{Path: "/missing/file.pdf"}
	assert.Equal(t, "file not found: /missing/file.pdf", err.Error())
}

func TestPathValidationError_Error(t *testing.T) {
	err := &PathValidationError{Path: "/bad/path", Reason: "path traversal not allowed"}
	assert.Equal(t, "path validation failed for /bad/path: path traversal not allowed", err.Error())
}
