package converter

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInput_URL(t *testing.T) {
	input := "https://example.com/document.pdf"
	info := ParseInput(input)

	assert.Equal(t, InputTypeURL, info.Type)
	assert.NotNil(t, info.URL)
	assert.Equal(t, "example.com", info.URL.Host)
	assert.Equal(t, ".pdf", info.Ext)
}

func TestParseInput_LocalFile(t *testing.T) {
	// Create a temp file for testing
	tmpFile, err := os.CreateTemp("", "vibecheck-test-*.docx")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	input := tmpFile.Name()
	info := ParseInput(input)

	assert.Equal(t, InputTypeFile, info.Type)
	assert.Equal(t, input, info.Path)
	assert.Equal(t, ".docx", info.Ext)
}

func TestParseInput_Text(t *testing.T) {
	input := "This is raw text content"
	info := ParseInput(input)

	assert.Equal(t, InputTypeText, info.Type)
	assert.Empty(t, info.Path)
}

func TestIsSupportedExtension(t *testing.T) {
	tests := []struct {
		ext      string
		supported bool
	}{
		{".pdf", true},
		{".docx", true},
		{".pptx", true},
		{".xlsx", true},
		{".md", true},
		{".txt", true},
		{".html", true},
		{".xyz", false},
		{".exe", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			assert.Equal(t, tt.supported, IsSupportedExtension(tt.ext))
		})
	}
}

func TestIsMarkdownFile(t *testing.T) {
	tests := []struct {
		ext     string
		isMarkdown bool
	}{
		{".md", true},
		{".txt", true},
		{".MD", true},
		{".TXT", true},
		{".pdf", false},
		{".docx", false},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			assert.Equal(t, tt.isMarkdown, IsMarkdownFile(tt.ext))
		})
	}
}

func TestHTTPError(t *testing.T) {
	err := &HTTPError{StatusCode: 404, URL: "https://example.com/test.pdf"}

	assert.Equal(t, "Not Found", err.Error())
	assert.Equal(t, 404, err.StatusCode)
	assert.Equal(t, "https://example.com/test.pdf", err.URL)
}
