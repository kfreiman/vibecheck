package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCVCheckTool_ReturnsPrompt(t *testing.T) {
	t.Run("ReturnsAnalysisPromptInsteadOfSampling", func(t *testing.T) {
		// Arrange
		cvContent := "Senior Software Engineer with 5+ years in Go, Kubernetes, and cloud infrastructure."
		jobContent := "Need Go developer with 5+ years experience in Kubernetes"

		tool := &CVCheckTool{}
		args := map[string]interface{}{
			"cv":  cvContent,
			"job": jobContent,
		}
		argsBytes, _ := json.Marshal(args)

		request := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			},
		}

		// Act
		result, err := tool.Call(context.Background(), request)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, result.Content, 1)

		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok, "Content should be TextContent")

		// Verify prompt structure
		prompt := textContent.Text
		assert.Contains(t, prompt, "You are an expert career advisor")
		assert.Contains(t, prompt, "**CV Content:**")
		assert.Contains(t, prompt, "**Job Description:**")
		assert.Contains(t, prompt, "**Analysis Requirements:**")
		assert.Contains(t, prompt, "1. **Key Skills Match**")
		assert.Contains(t, prompt, "2. **Experience Alignment**")
		assert.Contains(t, prompt, "3. **Missing Requirements**")
		assert.Contains(t, prompt, "4. **Recommendations**")
		assert.Contains(t, prompt, "5. **Overall Match Score**")
		assert.Contains(t, prompt, cvContent)
		assert.Contains(t, prompt, jobContent)
	})

	t.Run("UsesFileContentInPrompt", func(t *testing.T) {
		// Arrange
		tool := &CVCheckTool{}
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

		// Act
		result, err := tool.Call(context.Background(), request)

		// Assert
		require.NoError(t, err)
		require.NotNil(t, result)

		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)

		// Should contain actual CV content
		assert.Contains(t, textContent.Text, "Senior Software Engineer")
		assert.Contains(t, textContent.Text, "Go")
		assert.Contains(t, textContent.Text, "Kubernetes")
	})
}

func TestCVCheckTool_PromptStructure(t *testing.T) {
	t.Run("AllRequiredSectionsPresent", func(t *testing.T) {
		prompt := BuildAnalysisPrompt("test cv", "test job")

		// Verify all sections are present
		sections := []string{
			"You are an expert career advisor",
			"**CV Content:**",
			"**Job Description:**",
			"**Analysis Requirements:**",
			"1. **Key Skills Match** (0-100%)",
			"2. **Experience Alignment** (0-100%)",
			"3. **Missing Requirements**",
			"4. **Recommendations**",
			"5. **Overall Match Score** (0-100%)",
			"Format your response clearly",
		}

		for _, section := range sections {
			assert.Contains(t, prompt, section, "Prompt should contain section: %s", section)
		}
	})

	t.Run("NoSamplingOrSessionLogic", func(t *testing.T) {
		// This test verifies that the implementation doesn't use session sampling
		args := map[string]interface{}{
			"cv":  "cv",
			"job": "job",
		}
		argsBytes, _ := json.Marshal(args)

		request := &mcp.CallToolRequest{
			Params: &mcp.CallToolParamsRaw{
				Arguments: argsBytes,
			},
		}

		// Call should succeed without session
		cvTool := &CVCheckTool{}
		result, err := cvTool.Call(context.Background(), request)

		require.NoError(t, err)
		require.NotNil(t, result)

		// Should always return prompt, never depends on session
		textContent := result.Content[0].(*mcp.TextContent)
		assert.Contains(t, textContent.Text, "Analyze")
	})
}