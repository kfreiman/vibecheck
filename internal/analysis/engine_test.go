package analysis

import (
	"context"
	"testing"
)

func TestNewAnalysisEngine(t *testing.T) {
	engine := NewAnalysisEngine()
	if engine == nil {
		t.Fatal("NewAnalysisEngine returned nil")
	}
	if engine.indexMapping == nil {
		t.Fatal("NewAnalysisEngine created engine with nil indexMapping")
	}
}

func TestEngine_Analyze_SingleWord(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	// Test identical single words
	result, err := engine.Analyze(ctx, "golang", "golang")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.MatchPercentage != 100 {
		t.Errorf("Expected 100%% match for identical words, got %d%%", result.MatchPercentage)
	}

	if result.SkillCoverage != 1.0 {
		t.Errorf("Expected skill coverage 1.0, got %f", result.SkillCoverage)
	}

	if len(result.TopSkills) != 1 || result.TopSkills[0] != "golang" {
		t.Errorf("Expected top skill 'golang', got %v", result.TopSkills)
	}

	if len(result.MissingSkills) != 0 {
		t.Errorf("Expected no missing skills, got %v", result.MissingSkills)
	}
}

func TestEngine_Analyze_MultipleWords(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	cv := "golang python rust"
	jd := "golang java typescript"

	result, err := engine.Analyze(ctx, cv, jd)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should have some match (golang)
	if result.MatchPercentage < 10 || result.MatchPercentage > 90 {
		t.Errorf("Match percentage should be reasonable for partial overlap, got %d%%", result.MatchPercentage)
	}

	// Skill coverage: 1 out of 3 JD terms matched
	expectedCoverage := 1.0 / 3.0
	if result.SkillCoverage != expectedCoverage {
		t.Errorf("Expected skill coverage %f, got %f", expectedCoverage, result.SkillCoverage)
	}

	// Should have golang in top skills
	found := false
	for _, skill := range result.TopSkills {
		if skill == "golang" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'golang' in top skills, got %v", result.TopSkills)
	}

	// Should have java and typescript in missing skills
	if len(result.MissingSkills) != 2 {
		t.Errorf("Expected 2 missing skills, got %d: %v", len(result.MissingSkills), result.MissingSkills)
	}
}

func TestEngine_Analyze_EmptyContent(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	// Empty CV
	_, err := engine.Analyze(ctx, "", "golang")
	if err == nil {
		t.Fatal("Expected error for empty CV, got nil")
	}

	// Empty JD
	_, err = engine.Analyze(ctx, "golang", "")
	if err == nil {
		t.Fatal("Expected error for empty JD, got nil")
	}

	// Both empty
	_, err = engine.Analyze(ctx, "", "")
	if err == nil {
		t.Fatal("Expected error for empty both, got nil")
	}
}

func TestEngine_Analyze_WhitespaceHandling(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	cv := "  golang   python  "
	jd := "golang python"

	result, err := engine.Analyze(ctx, cv, jd)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should be 100% match despite extra whitespace
	if result.MatchPercentage != 100 {
		t.Errorf("Expected 100%% match with whitespace difference, got %d%%", result.MatchPercentage)
	}
}

func TestEngine_Analyze_IdenticalDocuments(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	cv := "golang python rust kubernetes docker postgresql"
	jd := "golang python rust kubernetes docker postgresql"

	result, err := engine.Analyze(ctx, cv, jd)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Identical documents should have very high match
	if result.MatchPercentage < 95 {
		t.Errorf("Expected high match for identical documents, got %d%%", result.MatchPercentage)
	}

	if result.SkillCoverage < 0.9 {
		t.Errorf("Expected high skill coverage for identical documents, got %f", result.SkillCoverage)
	}

	if len(result.MissingSkills) != 0 {
		t.Errorf("Expected no missing skills for identical documents, got %v", result.MissingSkills)
	}
}

func TestEngine_Analyze_CompletelyDifferent(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	cv := "golang python rust"
	jd := "java typescript react"

	result, err := engine.Analyze(ctx, cv, jd)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Completely different should have low match
	if result.MatchPercentage > 20 {
		t.Errorf("Expected low match for completely different documents, got %d%%", result.MatchPercentage)
	}

	if result.SkillCoverage > 0.2 {
		t.Errorf("Expected low skill coverage, got %f", result.SkillCoverage)
	}

	if len(result.MissingSkills) != 3 {
		t.Errorf("Expected 3 missing skills, got %d", len(result.MissingSkills))
	}
}

func TestEngine_Analyze_PartialOverlap(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	// CV has golang, python, rust
	// JD has golang, java, python
	// Overlap: golang, python (2 out of 3)
	cv := "golang python rust"
	jd := "golang java python"

	result, err := engine.Analyze(ctx, cv, jd)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should have medium match
	if result.MatchPercentage < 40 || result.MatchPercentage > 80 {
		t.Errorf("Expected medium match for partial overlap, got %d%%", result.MatchPercentage)
	}

	// Skill coverage: 2 out of 3
	expectedCoverage := 2.0 / 3.0
	if result.SkillCoverage != expectedCoverage {
		t.Errorf("Expected skill coverage %f, got %f", expectedCoverage, result.SkillCoverage)
	}

	// Should have java in missing skills
	found := false
	for _, skill := range result.MissingSkills {
		if skill == "java" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected 'java' in missing skills, got %v", result.MissingSkills)
	}
}

func TestEngine_Analyze_CommonTerms(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	cv := "golang python"
	jd := "golang python rust"

	result, err := engine.Analyze(ctx, cv, jd)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should have common terms
	if len(result.CommonTerms) != 2 {
		t.Errorf("Expected 2 common terms, got %d", len(result.CommonTerms))
	}

	// Check that terms are in the result
	termMap := make(map[string]bool)
	for _, ts := range result.CommonTerms {
		termMap[ts.Term] = true
		if ts.Score <= 0 {
			t.Errorf("Common term should have positive score, got %f for %s", ts.Score, ts.Term)
		}
	}

	if !termMap["golang"] || !termMap["python"] {
		t.Errorf("Expected 'golang' and 'python' in common terms, got %v", result.CommonTerms)
	}
}

func TestEngine_Analyze_MultiWordSentences(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	cv := "backend developer with golang experience and python skills"
	jd := "senior golang developer needed with python and rust"

	result, err := engine.Analyze(ctx, cv, jd)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should have reasonable match (golang, python overlap)
	if result.MatchPercentage < 20 || result.MatchPercentage > 80 {
		t.Errorf("Expected reasonable match for sentence overlap, got %d%%", result.MatchPercentage)
	}

	// Should find common terms
	if len(result.CommonTerms) < 2 {
		t.Errorf("Expected at least 2 common terms, got %d", len(result.CommonTerms))
	}
}

func TestEngine_Analyze_TopSkillsLimit(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	// Many overlapping terms
	cv := "a b c d e f g h i j k l m n o"
	jd := "a b c d e f g h i j k l m n o"

	result, err := engine.Analyze(ctx, cv, jd)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Top skills should be limited to 10
	if len(result.TopSkills) > 10 {
		t.Errorf("Expected top skills limited to 10, got %d", len(result.TopSkills))
	}
}

func TestEngine_Analyze_MissingSkillsLimit(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	cv := "a"
	jd := "a b c d e f g h i j k l m n o p q r s t"

	result, err := engine.Analyze(ctx, cv, jd)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Missing skills should be limited to 20
	if len(result.MissingSkills) > 20 {
		t.Errorf("Expected missing skills limited to 20, got %d", len(result.MissingSkills))
	}
}

func TestEngine_Analyze_CommonTermsLimit(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	// Many overlapping terms
	cv := "a b c d e f g h i j k l m n o p q r s t"
	jd := "a b c d e f g h i j k l m n o p q r s t"

	result, err := engine.Analyze(ctx, cv, jd)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Common terms should be limited to 15
	if len(result.CommonTerms) > 15 {
		t.Errorf("Expected common terms limited to 15, got %d", len(result.CommonTerms))
	}
}

func TestEngine_Analyze_CaseInsensitive(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	cv := "GOLANG Python"
	jd := "golang PYTHON"

	result, err := engine.Analyze(ctx, cv, jd)
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Should be 100% match despite case differences
	if result.MatchPercentage != 100 {
		t.Errorf("Expected 100%% match with case difference, got %d%%", result.MatchPercentage)
	}
}

func TestEngine_Analyze_MatchPercentageBoundaries(t *testing.T) {
	engine := NewAnalysisEngine()
	ctx := context.Background()

	// Test minimum boundary
	_, err := engine.Analyze(ctx, "a", "z")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	// Test maximum boundary
	result, err := engine.Analyze(ctx, "golang", "golang")
	if err != nil {
		t.Fatalf("Analyze failed: %v", err)
	}

	if result.MatchPercentage < 0 || result.MatchPercentage > 100 {
		t.Errorf("MatchPercentage should be in 0-100 range, got %d", result.MatchPercentage)
	}
}

func TestPreprocessText(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"golang", "golang"},
		{"  golang  ", "golang"},
		{"GOLANG", "golang"},
		{"golang  python", "golang python"},
		{"  golang   python   rust  ", "golang python rust"},
		{"", ""},
		{"   ", ""},
	}

	for _, tt := range tests {
		result := preprocessText(tt.input)
		if result != tt.expected {
			t.Errorf("preprocessText(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}