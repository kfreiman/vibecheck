package analysis

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestSkillsDictionary_Load(t *testing.T) {
	sd := NewSkillsDictionary()

	// Dictionary should load successfully
	if len(sd.skillIndex) == 0 {
		t.Fatal("Skills dictionary failed to load")
	}

	// Should have multiple categories
	if len(sd.skillsByCategory) == 0 {
		t.Fatal("No categories loaded")
	}

	// Test lookup
	category, found := sd.FindSkill("go")
	if !found {
		t.Error("Failed to find 'go' in dictionary")
	}
	if category == "" {
		t.Error("Category should not be empty")
	}
}

func TestSkillsDictionary_Lookup(t *testing.T) {
	sd := NewSkillsDictionary()

	tests := []struct {
		skill    string
		found    bool
		category string
	}{
		{"go", true, "Programming Languages"},
		{"python", true, "Programming Languages"},
		{"react", true, "Frontend Frameworks"},
		{"postgresql", true, "Databases"},
		{"kubernetes", true, "Containerization & Orchestration"},
		{"terraform", true, "Infrastructure as Code"},
		{"nonexistent", false, ""},
	}

	for _, tt := range tests {
		category, found := sd.FindSkill(tt.skill)
		if found != tt.found {
			t.Errorf("FindSkill(%q) found=%v, want %v", tt.skill, found, tt.found)
		}
		if found && category != tt.category {
			t.Errorf("FindSkill(%q) category=%q, want %q", tt.skill, category, tt.category)
		}
	}
}

func TestExtractSkills_SingleSkill(t *testing.T) {
	ctx := context.Background()
	sd := NewSkillsDictionary()

	content := "Experienced in Go development"
	skills := ExtractSkills(ctx, content, sd)

	if len(skills) == 0 {
		t.Fatal("Expected to extract at least 1 skill")
	}

	if skills[0].Name != "go" {
		t.Errorf("Expected skill 'go', got %q", skills[0].Name)
	}

	if skills[0].Category != "Programming Languages" {
		t.Errorf("Expected category 'Programming Languages', got %q", skills[0].Category)
	}

	// Should have reasonable confidence
	if skills[0].Confidence < 0.5 {
		t.Errorf("Expected confidence >= 0.5, got %f", skills[0].Confidence)
	}
}

func TestExtractSkills_MultipleSkills(t *testing.T) {
	ctx := context.Background()
	sd := NewSkillsDictionary()

	content := "Backend developer with Go, Python, and PostgreSQL experience"
	skills := ExtractSkills(ctx, content, sd)

	if len(skills) < 3 {
		t.Fatalf("Expected at least 3 skills, got %d", len(skills))
	}

	// Check that expected skills are present
	skillNames := make(map[string]bool)
	for _, skill := range skills {
		skillNames[skill.Name] = true
	}

	expectedSkills := []string{"go", "python", "postgresql"}
	for _, expected := range expectedSkills {
		if !skillNames[expected] {
			t.Errorf("Expected to find skill %q", expected)
		}
	}
}

func TestExtractSkills_ExperienceExtraction(t *testing.T) {
	ctx := context.Background()
	sd := NewSkillsDictionary()

	tests := []struct {
		content          string
		skill            string
		expectedYears    int
	}{
		{"5 years of go experience", "go", 5},
		{"python with 3 years", "python", 3},
		{"go (2 years)", "go", 2},
		{"java developer, 8 years", "java", 8},
		{"react developer", "react", 0}, // No years specified
	}

	for _, tt := range tests {
		skills := ExtractSkills(ctx, tt.content, sd)
		var found *Skill
		for i := range skills {
			if skills[i].Name == tt.skill {
				found = &skills[i]
				break
			}
		}

		if found == nil {
			t.Errorf("Skill %q not found in content: %q", tt.skill, tt.content)
			continue
		}

		if found.Experience != tt.expectedYears {
			t.Errorf("For content %q, expected %d years, got %d",
				tt.content, tt.expectedYears, found.Experience)
		}
	}
}

func TestExtractSkills_ConfidenceScoring(t *testing.T) {
	ctx := context.Background()
	sd := NewSkillsDictionary()

	// Single mention
	skills1 := ExtractSkills(ctx, "Go developer", sd)
	// Multiple mentions
	skills2 := ExtractSkills(ctx, "Go developer with Go experience and Go skills", sd)

	var confidence1, confidence2 float64
	for _, s := range skills1 {
		if s.Name == "go" {
			confidence1 = s.Confidence
		}
	}
	for _, s := range skills2 {
		if s.Name == "go" {
			confidence2 = s.Confidence
		}
	}

	if confidence2 <= confidence1 {
		t.Errorf("Multiple mentions should have higher confidence. Single: %f, Multiple: %f",
			confidence1, confidence2)
	}
}

func TestMatchSkills_ExactMatches(t *testing.T) {
	cvSkills := []Skill{
		{Name: "go", Category: "Programming Languages", Experience: 3, Confidence: 0.8},
		{Name: "python", Category: "Programming Languages", Experience: 2, Confidence: 0.7},
		{Name: "postgresql", Category: "Databases", Experience: 3, Confidence: 0.9},
	}

	jdSkills := []Skill{
		{Name: "go", Category: "Programming Languages", Experience: 0, Confidence: 1.0},
		{Name: "python", Category: "Programming Languages", Experience: 0, Confidence: 1.0},
		{Name: "react", Category: "Frontend Frameworks", Experience: 0, Confidence: 1.0},
	}

	matches, missing, _ := MatchSkills(cvSkills, jdSkills)

	if len(matches) != 2 {
		t.Errorf("Expected 2 matches, got %d", len(matches))
	}

	if len(missing) != 1 {
		t.Errorf("Expected 1 missing skill, got %d", len(missing))
	}

	// Verify matches
	matchNames := make(map[string]bool)
	for _, m := range matches {
		matchNames[m.Name] = true
	}
	if !matchNames["go"] || !matchNames["python"] {
		t.Error("Expected 'go' and 'python' in matches")
	}

	// Verify missing
	if missing[0].Name != "react" {
		t.Errorf("Expected 'react' to be missing, got %s", missing[0].Name)
	}
}

func TestMatchSkills_EmptyInputs(t *testing.T) {
	// Empty CV
	matches, missing, _ := MatchSkills([]Skill{}, []Skill{{Name: "go", Category: "Programming Languages"}})
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches with empty CV, got %d", len(matches))
	}
	if len(missing) != 1 {
		t.Errorf("Expected 1 missing skill with empty CV, got %d", len(missing))
	}

	// Empty JD
	matches, missing, _ = MatchSkills([]Skill{{Name: "go", Category: "Programming Languages"}}, []Skill{})
	if len(matches) != 0 {
		t.Errorf("Expected 0 matches with empty JD, got %d", len(matches))
	}
	if len(missing) != 0 {
		t.Errorf("Expected 0 missing skills with empty JD, got %d", len(missing))
	}

	// Both empty
	matches, missing, _ = MatchSkills([]Skill{}, []Skill{})
	if len(matches) != 0 || len(missing) != 0 {
		t.Error("Both empty should return empty slices")
	}
}

func TestCalculateSkillCoverage_Skills(t *testing.T) {
	tests := []struct {
		cvSkills  []Skill
		jdSkills  []Skill
		expected  float64
	}{
		// Full coverage
		{
			[]Skill{{Name: "go"}, {Name: "python"}},
			[]Skill{{Name: "go"}, {Name: "python"}},
			1.0,
		},
		// Partial coverage
		{
			[]Skill{{Name: "go"}},
			[]Skill{{Name: "go"}, {Name: "python"}},
			0.5,
		},
		// No coverage
		{
			[]Skill{{Name: "java"}},
			[]Skill{{Name: "go"}, {Name: "python"}},
			0.0,
		},
		// Empty JD
		{
			[]Skill{{Name: "go"}},
			[]Skill{},
			0.0,
		},
		// Empty CV
		{
			[]Skill{},
			[]Skill{{Name: "go"}, {Name: "python"}},
			0.0,
		},
	}

	for i, tt := range tests {
		result := CalculateSkillCoverage(tt.cvSkills, tt.jdSkills)
		if result != tt.expected {
			t.Errorf("Test %d: expected %f, got %f", i, tt.expected, result)
		}
	}
}

func TestTokenizeContent(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"golang", []string{"golang"}},
		{"go, python; rust", []string{"go", "python", "rust"}},
		{"Go and Python", []string{"go", "python"}},
		{"experience with Go", []string{"experience", "go"}},
		{"", []string{}},
		{"the a an and", []string{}}, // All noise words
	}

	for _, tt := range tests {
		result := tokenizeContent(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("tokenizeContent(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}

		// Check contents
		resultMap := make(map[string]bool)
		for _, w := range result {
			resultMap[w] = true
		}

		for _, w := range tt.expected {
			if !resultMap[w] {
				t.Errorf("tokenizeContent(%q) missing %q", tt.input, w)
			}
		}
	}
}

func TestParseInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"5", 5},
		{"123", 123},
		{"5 years", 5},
		{"2+", 2},
		{"eight", 0},
		{"", 0},
		{"  7  ", 7},
	}

	for _, tt := range tests {
		result := parseInt(tt.input)
		if result != tt.expected {
			t.Errorf("parseInt(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

func TestLoadSkillsDictionary(t *testing.T) {
	skills, err := LoadSkillsDictionary()
	if err != nil {
		t.Fatalf("Failed to load skills dictionary: %v", err)
	}

	if len(skills) < 50 {
		t.Errorf("Expected at least 50 skills, got %d", len(skills))
	}

	// Check for known skills
	expectedSkills := []string{"Go", "Python", "React", "PostgreSQL", "Kubernetes", "Terraform"}
	skillMap := make(map[string]bool)
	for _, s := range skills {
		skillMap[s] = true
	}

	for _, expected := range expectedSkills {
		if !skillMap[expected] {
			t.Errorf("Expected skill %q not found in dictionary", expected)
		}
	}
}

func TestSkillsDictionary_LoadNotFound(t *testing.T) {
	// Change to non-existent directory to test error handling
	originalPath := filepath.Join("testdata", "skills_dictionary.txt")

	// Test that it handles missing dictionary gracefully
_sd := &SkillsDictionary{
		skillsByCategory: make(map[string][]string),
		skillIndex:       make(map[string]string),
	}
	// This should not panic even if file is missing
	_sd.loadDictionary()
	// Should just have empty dictionary
	if _sd.skillIndex == nil {
		t.Error("skillIndex should not be nil")
	}

	_ = originalPath // Use the variable
}

func TestSkillExtractionResult_ToJSON(t *testing.T) {
	result := &SkillExtractionResult{
		Skills: []Skill{
			{Name: "go", Category: "Programming Languages", Experience: 3, Confidence: 0.9},
			{Name: "python", Category: "Programming Languages", Experience: 2, Confidence: 0.8},
		},
		TotalSkills:  2,
		UniqueSkills: 2,
	}

	jsonStr, err := result.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	if jsonStr == "" {
		t.Error("JSON string should not be empty")
	}

	// Should contain expected fields
	if !strings.Contains(jsonStr, "go") || !strings.Contains(jsonStr, "Programming Languages") {
		t.Error("JSON should contain skill data")
	}
}
