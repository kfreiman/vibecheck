package mcp

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/kfreiman/vibecheck/internal/analysis"
	"github.com/kfreiman/vibecheck/internal/storage"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeTool_New(t *testing.T) {
	tempDir := t.TempDir()
	sm, err := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   tempDir,
		DefaultTTL: 24 * time.Hour,
	})
	require.NoError(t, err)

	tool := NewAnalyzeTool(sm)
	assert.NotNil(t, tool)
	assert.NotNil(t, tool.engine)
	assert.Equal(t, sm, tool.storageManager)
}

func TestAnalyzeTool_Call_BasicAnalysis(t *testing.T) {
	tempDir := t.TempDir()
	sm, err := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   tempDir,
		DefaultTTL: 24 * time.Hour,
	})
	require.NoError(t, err)

	// Create test documents
	cvContent := []byte("golang python rust kubernetes")
	jdContent := []byte("golang java typescript")

	// Store documents
	cvURI, err := sm.SaveDocument(storage.DocumentTypeCV, cvContent, "test_cv.md")
	require.NoError(t, err)
	jdURI, err := sm.SaveDocument(storage.DocumentTypeJD, jdContent, "test_jd.md")
	require.NoError(t, err)

	tool := NewAnalyzeTool(sm)

	// Create request
	args := map[string]interface{}{
		"cv_uri": cvURI,
		"jd_uri": jdURI,
	}
	argsJSON, _ := json.Marshal(args)

	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{
			Arguments: json.RawMessage(argsJSON),
		},
	}

	// Execute
	result, err := tool.Call(context.Background(), request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.Content, 1)

	// Parse JSON result
	var analyzeResult AnalyzeResult
	err = json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &analyzeResult)
	require.NoError(t, err)

	// Verify analysis results
	assert.Greater(t, analyzeResult.MatchPercentage, 0)
	assert.Less(t, analyzeResult.MatchPercentage, 100)
	assert.Greater(t, analyzeResult.WeightedScore, 0)
	assert.LessOrEqual(t, analyzeResult.WeightedScore, 100)
	assert.Greater(t, analyzeResult.SkillCoverage, 0.0)
	assert.Greater(t, analyzeResult.ExperienceMatch, 0.0)
	assert.Contains(t, analyzeResult.TopSkills, "golang")
	assert.Contains(t, analyzeResult.MissingSkills, "java")
	assert.NotEmpty(t, analyzeResult.AnalysisSummary)
	assert.NotNil(t, analyzeResult.ScoringBreakdown)
}

func TestAnalyzeTool_Call_IdenticalDocuments(t *testing.T) {
	tempDir := t.TempDir()
	sm, _ := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   tempDir,
		DefaultTTL: 24 * time.Hour,
	})

	content := []byte("golang python rust")
	cvURI, _ := sm.SaveDocument(storage.DocumentTypeCV, content, "cv.md")
	jdURI, _ := sm.SaveDocument(storage.DocumentTypeJD, content, "jd.md")

	tool := NewAnalyzeTool(sm)
	args := map[string]interface{}{"cv_uri": cvURI, "jd_uri": jdURI}
	argsJSON, _ := json.Marshal(args)
	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(argsJSON)},
	}

	result, err := tool.Call(context.Background(), request)
	require.NoError(t, err)

	var analyzeResult AnalyzeResult
	json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &analyzeResult)

	// Identical documents should have high match
	assert.Greater(t, analyzeResult.MatchPercentage, 90)
	assert.Equal(t, 1.0, analyzeResult.SkillCoverage)
	assert.Empty(t, analyzeResult.MissingSkills)
}

func TestAnalyzeTool_Call_EmptyContent(t *testing.T) {
	tempDir := t.TempDir()
	sm, _ := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   tempDir,
		DefaultTTL: 24 * time.Hour,
	})

	// Documents with different lengths (one empty, one with content)
	jdURI, _ := sm.SaveDocument(storage.DocumentTypeJD, []byte(""), "empty.md")
	cvURI, _ := sm.SaveDocument(storage.DocumentTypeCV, []byte("golang"), "cv.md")

	tool := NewAnalyzeTool(sm)
	args := map[string]interface{}{"cv_uri": cvURI, "jd_uri": jdURI}
	argsJSON, _ := json.Marshal(args)
	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(argsJSON)},
	}

	result, err := tool.Call(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, result.Content[0].(*mcp.TextContent).Text, "analysis failed")
}

func TestAnalyzeTool_Call_InvalidURIs(t *testing.T) {
	tempDir := t.TempDir()
	sm, _ := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   tempDir,
		DefaultTTL: 24 * time.Hour,
	})

	tool := NewAnalyzeTool(sm)

	tests := []struct {
		name        string
		args        map[string]interface{}
		expectError bool
		errorField  string
	}{
		{
			name:        "missing cv_uri",
			args:        map[string]interface{}{"jd_uri": "jd://123"},
			expectError: true,
			errorField:  "cv_uri",
		},
		{
			name:        "missing jd_uri",
			args:        map[string]interface{}{"cv_uri": "cv://123"},
			expectError: true,
			errorField:  "jd_uri",
		},
		{
			name:        "wrong cv format",
			args:        map[string]interface{}{"cv_uri": "file://123", "jd_uri": "jd://123"},
			expectError: true,
			errorField:  "cv_uri",
		},
		{
			name:        "wrong jd format",
			args:        map[string]interface{}{"cv_uri": "cv://123", "jd_uri": "file://123"},
			expectError: true,
			errorField:  "jd_uri",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			argsJSON, _ := json.Marshal(tt.args)
			request := &mcp.CallToolRequest{
				Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(argsJSON)},
			}

			result, err := tool.Call(context.Background(), request)
			if tt.expectError {
				assert.Error(t, err)
				// Check if result contains error message about the field
				if len(result.Content) > 0 {
					textContent := result.Content[0].(*mcp.TextContent).Text
					assert.Contains(t, textContent, tt.errorField)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAnalyzeTool_Call_MissingDocuments(t *testing.T) {
	tempDir := t.TempDir()
	sm, _ := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   tempDir,
		DefaultTTL: 24 * time.Hour,
	})

	tool := NewAnalyzeTool(sm)

	// Non-existent documents
	args := map[string]interface{}{
		"cv_uri": "cv://nonexistent",
		"jd_uri": "jd://nonexistent",
	}
	argsJSON, _ := json.Marshal(args)
	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(argsJSON)},
	}

	result, err := tool.Call(context.Background(), request)
	require.Error(t, err)
	assert.Contains(t, result.Content[0].(*mcp.TextContent).Text, "not found")
}

func TestAnalyzeTool_Call_MultiWordSentences(t *testing.T) {
	tempDir := t.TempDir()
	sm, _ := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   tempDir,
		DefaultTTL: 24 * time.Hour,
	})

	cvContent := []byte("backend developer with golang experience and python skills")
	jdContent := []byte("senior golang developer needed with python and rust")

	cvURI, _ := sm.SaveDocument(storage.DocumentTypeCV, cvContent, "cv.md")
	jdURI, _ := sm.SaveDocument(storage.DocumentTypeJD, jdContent, "jd.md")

	tool := NewAnalyzeTool(sm)
	args := map[string]interface{}{"cv_uri": cvURI, "jd_uri": jdURI}
	argsJSON, _ := json.Marshal(args)
	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(argsJSON)},
	}

	result, err := tool.Call(context.Background(), request)
	require.NoError(t, err)

	var analyzeResult AnalyzeResult
	json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &analyzeResult)

	// Should have reasonable match (golang, python overlap)
	assert.Greater(t, analyzeResult.MatchPercentage, 10)
	assert.Less(t, analyzeResult.MatchPercentage, 90)
	assert.Greater(t, analyzeResult.SkillCoverage, 0.0)
	assert.Contains(t, analyzeResult.TopSkills, "golang")
	assert.Contains(t, analyzeResult.TopSkills, "python")
}

func TestAnalyzeTool_Call_WhitespaceHandling(t *testing.T) {
	tempDir := t.TempDir()
	sm, _ := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   tempDir,
		DefaultTTL: 24 * time.Hour,
	})

	cvContent := []byte("  golang   python  ")
	jdContent := []byte("golang python")

	cvURI, _ := sm.SaveDocument(storage.DocumentTypeCV, cvContent, "cv.md")
	jdURI, _ := sm.SaveDocument(storage.DocumentTypeJD, jdContent, "jd.md")

	tool := NewAnalyzeTool(sm)
	args := map[string]interface{}{"cv_uri": cvURI, "jd_uri": jdURI}
	argsJSON, _ := json.Marshal(args)
	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(argsJSON)},
	}

	result, err := tool.Call(context.Background(), request)
	require.NoError(t, err)

	var analyzeResult AnalyzeResult
	json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &analyzeResult)

	// Should be 100% match despite whitespace
	assert.Equal(t, 100, analyzeResult.MatchPercentage)
}

func TestAnalyzeTool_Call_CaseInsensitive(t *testing.T) {
	tempDir := t.TempDir()
	sm, _ := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   tempDir,
		DefaultTTL: 24 * time.Hour,
	})

	cvContent := []byte("GOLANG Python")
	jdContent := []byte("golang PYTHON")

	cvURI, _ := sm.SaveDocument(storage.DocumentTypeCV, cvContent, "cv.md")
	jdURI, _ := sm.SaveDocument(storage.DocumentTypeJD, jdContent, "jd.md")

	tool := NewAnalyzeTool(sm)
	args := map[string]interface{}{"cv_uri": cvURI, "jd_uri": jdURI}
	argsJSON, _ := json.Marshal(args)
	request := &mcp.CallToolRequest{
		Params: &mcp.CallToolParamsRaw{Arguments: json.RawMessage(argsJSON)},
	}

	result, err := tool.Call(context.Background(), request)
	require.NoError(t, err)

	var analyzeResult AnalyzeResult
	json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &analyzeResult)

	// Should be 100% match despite case differences
	assert.Equal(t, 100, analyzeResult.MatchPercentage)
}

func TestAnalyzeTool_buildSummary(t *testing.T) {
	tempDir := t.TempDir()
	sm, _ := storage.NewStorageManager(storage.StorageConfig{
		BasePath:   tempDir,
		DefaultTTL: 24 * time.Hour,
	})
	tool := NewAnalyzeTool(sm)

	result := &analysis.AnalysisResult{
		MatchPercentage: 75,
		SkillCoverage:   0.6,
		TopSkills:       []string{"golang", "python"},
		MissingSkills:   []string{"java", "rust"},
		CommonTerms: []analysis.TermScore{
			{Term: "golang", Score: 2.5},
			{Term: "python", Score: 1.8},
		},
	}

	summary := tool.buildSummary(result)

	assert.Contains(t, summary, "75%")
	assert.Contains(t, summary, "60.0%")
	assert.Contains(t, summary, "golang")
	assert.Contains(t, summary, "python")
	assert.Contains(t, summary, "java")
	assert.Contains(t, summary, "rust")
}