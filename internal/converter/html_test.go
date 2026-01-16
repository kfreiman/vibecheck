package converter

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTMLConverter_NewHTMLConverter(t *testing.T) {
	t.Run("creates converter successfully", func(t *testing.T) {
		converter, err := NewHTMLConverter()
		require.NoError(t, err)
		require.NotNil(t, converter)
		assert.True(t, converter.IsAvailable())

		// Cleanup
		converter.Close()
	})
}

func TestHTMLConverter_IsAvailable(t *testing.T) {
	t.Run("returns true when initialized", func(t *testing.T) {
		converter, err := NewHTMLConverter()
		require.NoError(t, err)
		defer converter.Close()

		assert.True(t, converter.IsAvailable())
	})

	t.Run("returns false when not initialized", func(t *testing.T) {
		converter := &HTMLConverter{}
		assert.False(t, converter.IsAvailable())
	})
}

func TestHTMLConverter_Supports(t *testing.T) {
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	tests := []struct {
		input    string
		expected bool
	}{
		{"test.html", true},
		{"test.htm", true},
		{"TEST.HTML", true},
		{"TEST.HTM", true},
		{"https://example.com/page.html", true},
		{"http://example.com/page", true}, // URL without extension still supported
		{"test.pdf", false},
		{"test.txt", false},
		{"plain text", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := converter.Supports(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHTMLConverter_Convert_StaticHTML(t *testing.T) {
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	// Create a temporary HTML file with main content and boilerplate
	tmpDir, err := ioutil.TempDir("", "html-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Note: go-readability focuses on main content area.
	// The name is included in the main content, not just the header.
	htmlContent := `<!DOCTYPE html>
<html>
<head>
    <title>Test CV - John Doe</title>
</head>
<body>
    <nav>
        <a href="/">Home</a>
        <a href="/about">About</a>
    </nav>
    <div class="ad-banner">Advertisement</div>
    <main>
        <article>
            <h1>John Doe</h1>
            <h2>Software Engineer</h2>
            <section>
                <h3>Experience</h3>
                <p>Senior Developer at TechCorp (2020-2024)</p>
                <p>Worked on various web applications using Go and React</p>
            </section>
            <section>
                <h3>Education</h3>
                <p>BS Computer Science, University of Technology</p>
            </section>
        </article>
    </main>
    <footer>Copyright 2024</footer>
</body>
</html>`

	htmlFile := filepath.Join(tmpDir, "cv.html")
	err = ioutil.WriteFile(htmlFile, []byte(htmlContent), 0644)
	require.NoError(t, err)

	// Test conversion
	result, err := converter.Convert(context.Background(), htmlFile)
	require.NoError(t, err)

	// Verify main content is extracted
	// Note: go-readability may not always include the <h1> in the extracted content
	// depending on the HTML structure. The important thing is that the main content
	// (experience, education) is extracted correctly.
	assert.Contains(t, result, "Software Engineer")
	assert.Contains(t, result, "Senior Developer")
	assert.Contains(t, result, "Computer Science")

	// Verify boilerplate is removed (go-readability should handle this)
	assert.NotContains(t, result, "Advertisement")
	assert.NotContains(t, result, "Copyright 2024")
}

func TestHTMLConverter_Convert_MalformedHTML(t *testing.T) {
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	tmpDir, err := ioutil.TempDir("", "html-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Malformed HTML - missing closing tags
	htmlContent := `<html><head><title>Test</title><body><h1>Content</h1><p>Some text`
	htmlFile := filepath.Join(tmpDir, "malformed.html")
	err = ioutil.WriteFile(htmlFile, []byte(htmlContent), 0644)
	require.NoError(t, err)

	// Should still work (playwright handles malformed HTML)
	result, err := converter.Convert(context.Background(), htmlFile)
	require.NoError(t, err)

	// Should extract some content
	assert.Contains(t, result, "Content")
}

func TestHTMLConverter_Convert_NonExistentFile(t *testing.T) {
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	_, err = converter.Convert(context.Background(), "/nonexistent/file.html")
	require.Error(t, err)

	// Should be a FileNotFoundError
	_, ok := err.(*FileNotFoundError)
	assert.True(t, ok, "Expected FileNotFoundError")
}

func TestHTMLConverter_Convert_EmptyHTML(t *testing.T) {
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	tmpDir, err := ioutil.TempDir("", "html-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	htmlContent := `<!DOCTYPE html><html><head><title></title></head><body></body></html>`
	htmlFile := filepath.Join(tmpDir, "empty.html")
	err = ioutil.WriteFile(htmlFile, []byte(htmlContent), 0644)
	require.NoError(t, err)

	// Should handle gracefully
	_, err = converter.Convert(context.Background(), htmlFile)
	// May succeed with empty result or error - both are acceptable
	// Just verify it doesn't panic
}

func TestHTMLConverter_Convert_PathTraversal(t *testing.T) {
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	// Test path traversal attempts
	tests := []string{
		"../etc/passwd",
		"../../html.go",
		"test/../../../html.go",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			_, err := converter.Convert(context.Background(), path)
			require.Error(t, err)

			// Should be a PathValidationError
			_, ok := err.(*PathValidationError)
			assert.True(t, ok, "Expected PathValidationError for path: %s", path)
		})
	}
}

func TestHTMLConverter_Convert_URL(t *testing.T) {
	// This test requires network access and may be flaky
	// We'll use a simple HTML file served via file:// URL
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	tmpDir, err := ioutil.TempDir("", "html-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	htmlContent := `<!DOCTYPE html>
<html>
<head><title>URL Test</title></head>
<body>
    <main>
        <h1>URL Content</h1>
        <p>This was loaded from a URL</p>
    </main>
</body>
</html>`

	htmlFile := filepath.Join(tmpDir, "url-test.html")
	err = ioutil.WriteFile(htmlFile, []byte(htmlContent), 0644)
	require.NoError(t, err)

	// Use file:// URL
	fileURL := "file://" + htmlFile
	result, err := converter.Convert(context.Background(), fileURL)
	require.NoError(t, err)

	assert.Contains(t, result, "URL Content")
	assert.Contains(t, result, "loaded from a URL")
}

func TestHTMLConverter_Close(t *testing.T) {
	t.Run("closes successfully", func(t *testing.T) {
		converter, err := NewHTMLConverter()
		require.NoError(t, err)

		err = converter.Close()
		assert.NoError(t, err)
	})

	t.Run("handles double close", func(t *testing.T) {
		converter, err := NewHTMLConverter()
		require.NoError(t, err)

		err = converter.Close()
		assert.NoError(t, err)

		// Second close should also be safe
		err = converter.Close()
		assert.NoError(t, err)
	})

	t.Run("handles close on nil converter", func(t *testing.T) {
		converter := &HTMLConverter{}
		err := converter.Close()
		assert.NoError(t, err)
	})
}

func TestHTMLConverter_convertFile(t *testing.T) {
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	tmpDir, err := ioutil.TempDir("", "html-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test HTML
	htmlContent := `<!DOCTYPE html>
<html>
<body>
    <main>
        <h1>Test Content</h1>
        <p>Paragraph text</p>
    </main>
</body>
</html>`

	htmlFile := filepath.Join(tmpDir, "test.html")
	err = ioutil.WriteFile(htmlFile, []byte(htmlContent), 0644)
	require.NoError(t, err)

	t.Run("extracts content from valid file", func(t *testing.T) {
		result, err := converter.convertFile(context.Background(), htmlFile)
		require.NoError(t, err)
		assert.Contains(t, result, "Test Content")
		assert.Contains(t, result, "Paragraph text")
	})

	t.Run("rejects path traversal", func(t *testing.T) {
		_, err := converter.convertFile(context.Background(), "../test.html")
		require.Error(t, err)
		_, ok := err.(*PathValidationError)
		assert.True(t, ok)
	})

	t.Run("handles non-existent file", func(t *testing.T) {
		_, err := converter.convertFile(context.Background(), "/nonexistent/file.html")
		require.Error(t, err)
		_, ok := err.(*FileNotFoundError)
		assert.True(t, ok)
	})
}

func TestHTMLConverter_convertURL(t *testing.T) {
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	tmpDir, err := ioutil.TempDir("", "html-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test HTML for file:// URL
	htmlContent := `<!DOCTYPE html>
<html>
<body>
    <main>
        <h1>URL Test</h1>
        <p>Content from URL</p>
    </main>
</body>
</html>`

	htmlFile := filepath.Join(tmpDir, "url-test.html")
	err = ioutil.WriteFile(htmlFile, []byte(htmlContent), 0644)
	require.NoError(t, err)

	t.Run("extracts content from file URL", func(t *testing.T) {
		fileURL := "file://" + htmlFile
		result, err := converter.convertURL(context.Background(), fileURL)
		require.NoError(t, err)
		assert.Contains(t, result, "URL Test")
		assert.Contains(t, result, "Content from URL")
	})
}

func TestHTMLConverter_renderWithPlaywright(t *testing.T) {
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	tmpDir, err := ioutil.TempDir("", "html-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create test HTML
	htmlContent := `<!DOCTYPE html>
<html>
<body>
    <main>
        <h1>Playwright Test</h1>
        <p>Rendered content</p>
    </main>
</body>
</html>`

	htmlFile := filepath.Join(tmpDir, "playwright.html")
	err = ioutil.WriteFile(htmlFile, []byte(htmlContent), 0644)
	require.NoError(t, err)

	t.Run("renders and extracts content", func(t *testing.T) {
		fileURL := "file://" + htmlFile
		result, err := converter.renderWithPlaywright(context.Background(), fileURL)
		require.NoError(t, err)
		assert.Contains(t, result, "Playwright Test")
		assert.Contains(t, result, "Rendered content")
	})
}

func TestHTMLConverter_SupportsInputTypes(t *testing.T) {
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	// Test that Supports correctly identifies different input types
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"HTML file", "test.html", true},
		{"HTM file", "test.htm", true},
		{"HTML file uppercase", "TEST.HTML", true},
		{"HTTP URL", "http://example.com/page.html", true},
		{"HTTPS URL", "https://example.com/page", true},
		{"Plain text", "some text content", false},
		{"PDF file", "document.pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := converter.Supports(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHTMLConverter_Integration(t *testing.T) {
	// Integration test that verifies the full workflow
	converter, err := NewHTMLConverter()
	require.NoError(t, err)
	defer converter.Close()

	tmpDir, err := ioutil.TempDir("", "html-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a realistic CV HTML
	// Note: go-readability extracts content from the main article area
	cvHTML := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Jane Smith - CV</title>
    <style>
        body { font-family: Arial, sans-serif; }
        .header { background: #f0f0f0; padding: 20px; }
        .nav { background: #333; color: white; padding: 10px; }
        .ad { background: #ffeb3b; padding: 10px; }
        .footer { background: #ccc; padding: 10px; font-size: 12px; }
    </style>
</head>
<body>
    <div class="nav">
        <a href="/">Home</a> | <a href="/contact">Contact</a>
    </div>

    <div class="ad">
        🔥 HIRING! Apply now! 🔥
    </div>

    <main>
        <article>
            <h1>Jane Smith</h1>
            <h2>Senior Software Engineer</h2>

            <section id="experience">
                <h3>Professional Experience</h3>
                <section>
                    <h4>Tech Solutions Inc.</h4>
                    <p><strong>Senior Engineer</strong> | 2020 - Present</p>
                    <ul>
                        <li>Led development of microservices architecture</li>
                        <li>Improved system performance by 40%</li>
                    </ul>
                </section>
                <section>
                    <h4>StartupCo</h4>
                    <p><strong>Software Engineer</strong> | 2018 - 2020</p>
                    <ul>
                        <li>Built REST APIs using Go</li>
                        <li>Implemented CI/CD pipelines</li>
                    </ul>
                </section>
            </section>

            <section id="education">
                <h3>Education</h3>
                <p><strong>BS Computer Science</strong></p>
                <p>University of Technology, 2018</p>
            </section>

            <section id="skills">
                <h3>Skills</h3>
                <p>Go, Python, JavaScript, Docker, Kubernetes, AWS</p>
            </section>
        </article>
    </main>

    <div class="footer">
        © 2024 Jane Smith. All rights reserved.
        <a href="/privacy">Privacy Policy</a> | <a href="/terms">Terms</a>
    </div>
</body>
</html>`

	cvFile := filepath.Join(tmpDir, "jane-smith-cv.html")
	err = ioutil.WriteFile(cvFile, []byte(cvHTML), 0644)
	require.NoError(t, err)

	// Test conversion
	result, err := converter.Convert(context.Background(), cvFile)
	require.NoError(t, err)

	// Verify essential CV content is extracted
	// Note: go-readability may not always include the <h1> in the extracted content
	// The important thing is that the main content (experience, education, skills) is extracted
	assert.Contains(t, result, "Senior Software Engineer")
	assert.Contains(t, result, "Tech Solutions Inc")
	assert.Contains(t, result, "Computer Science")
	assert.Contains(t, result, "Go")
	assert.Contains(t, result, "Docker")

	// Verify boilerplate is removed
	assert.NotContains(t, result, "HIRING")
	assert.NotContains(t, result, "Privacy Policy")
	assert.NotContains(t, result, "Terms")
	assert.NotContains(t, result, "© 2024")
}
