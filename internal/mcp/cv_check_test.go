package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCVCheckTool_Call(t *testing.T) {
	tool := &CVCheckTool{}

	// Test case 1: Valid file paths
	t.Run("ValidFilePaths", func(t *testing.T) {
		// Use the correct relative path from the project root
		// The test runs from internal/mcp, so we need ../../testdata/
		args := map[string]interface{}{
			"cv":  "../../testdata/cv.md",
			"job": "../../testdata/job.md",
		}
		argsBytes, _ := json.Marshal(args)

		request := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			},
		}

		result, err := tool.Call(context.Background(), request)
		require.NoError(t, err, "Expected no error")
		require.NotNil(t, result, "Expected result")
		assert.NotEmpty(t, result.Content, "Expected content in result")
	})

	// Test case 2: Valid raw text
	t.Run("ValidRawText", func(t *testing.T) {
		cvText := "Experienced software engineer with Go expertise"
		jobText := "Looking for Go developer with 5+ years experience"

		args := map[string]interface{}{
			"cv":  cvText,
			"job": jobText,
		}
		argsBytes, _ := json.Marshal(args)

		request := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			},
		}

		result, err := tool.Call(context.Background(), request)
		require.NoError(t, err, "Expected no error")
		require.NotNil(t, result, "Expected result")
	})

	// Test case 3: Auto-detection (swapped inputs)
	t.Run("AutoDetection", func(t *testing.T) {
		cvText := "Experienced software engineer with expertise in Go, Kubernetes, and AWS. Worked on microservices."
		jobText := "Requirements: 5+ years experience, Go, Kubernetes. Responsibilities: Design scalable systems."

		args := map[string]interface{}{
			"cv":  cvText,
			"job": jobText,
		}
		argsBytes, _ := json.Marshal(args)

		request := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			},
		}

		result, err := tool.Call(context.Background(), request)
		require.NoError(t, err, "Expected no error")
		require.NotNil(t, result, "Expected result")
		require.NotEmpty(t, result.Content, "Expected content in result")

		// Check that the result contains the expected data
		content := result.Content[0].(*mcp.TextContent).Text
		assert.NotEmpty(t, content, "Expected non-empty content")
	})

	// Test case 4: Missing parameters
	t.Run("MissingParameters", func(t *testing.T) {
		args := map[string]interface{}{
			"cv": "some content",
			// missing job
		}
		argsBytes, _ := json.Marshal(args)

		request := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			},
		}

		_, err := tool.Call(context.Background(), request)
		assert.Error(t, err, "Expected error for missing parameters")
	})

	// Test case 5: Empty parameters
	t.Run("EmptyParameters", func(t *testing.T) {
		args := map[string]interface{}{
			"cv":  "",
			"job": "",
		}
		argsBytes, _ := json.Marshal(args)

		request := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			},
		}

		_, err := tool.Call(context.Background(), request)
		assert.Error(t, err, "Expected error for empty parameters")
	})

	// Test case 6: Invalid file path (file path pattern but file doesn't exist)
	t.Run("InvalidFilePath", func(t *testing.T) {
		// Create a clearly invalid path that will be treated as file path pattern
		args := map[string]interface{}{
			"cv":  "./nonexistent/path/to/file.md",
			"job": "test_job.md",
		}
		argsBytes, _ := json.Marshal(args)

		request := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			},
		}

		_, err := tool.Call(context.Background(), request)
		assert.Error(t, err, "Expected error for invalid file path")
	})
}

func TestDebugPathDetection(t *testing.T) {
	tool := &CVCheckTool{}

	paths := []string{
		"./nonexistent/path/to/file.md",
		"CV.md",
		"test_job.md",
		"nonexistent_file.md",
		"/absolute/path.md",
	}

	// Test that isFilePath handles various path patterns correctly
	expectedResults := map[string]bool{
		"./nonexistent/path/to/file.md": true, // relative path with extension
		"CV.md":                         true, // simple file with extension
		"test_job.md":                   true, // simple file with extension
		"nonexistent_file.md":           true, // simple file with extension
		"/absolute/path.md":             true, // absolute path with extension
	}

	for _, path := range paths {
		isFile := tool.isFilePath(path)
		expected := expectedResults[path]
		assert.Equal(t, expected, isFile, "isFilePath(%s) should return %v", path, expected)
		t.Logf("Path: %s -> isFilePath: %v", path, isFile)
	}
}
