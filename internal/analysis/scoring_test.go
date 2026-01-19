package analysis

import (
	"math"
	"strings"
	"testing"
)

// almostEqual checks if two float64 values are approximately equal
func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) <= tolerance
}

func TestNewDefaultWeights(t *testing.T) {
	weights := NewDefaultWeights()

	if weights.SkillCoverage != 0.40 {
		t.Errorf("Expected SkillCoverage 0.40, got %f", weights.SkillCoverage)
	}
	if weights.Experience != 0.30 {
		t.Errorf("Expected Experience 0.30, got %f", weights.Experience)
	}
	if weights.TermSimilarity != 0.20 {
		t.Errorf("Expected TermSimilarity 0.20, got %f", weights.TermSimilarity)
	}
	if weights.OverallMatch != 0.10 {
		t.Errorf("Expected OverallMatch 0.10, got %f", weights.OverallMatch)
	}
}

func TestScoringWeights_ValidateWeights(t *testing.T) {
	tests := []struct {
		weights   ScoringWeights
		shouldErr bool
	}{
		{NewDefaultWeights(), false}, // 0.40 + 0.30 + 0.20 + 0.10 = 1.0
		{ScoringWeights{0.5, 0.3, 0.1, 0.1}, false}, // Sum: 1.0
		{ScoringWeights{0.8, 0.1, 0.05, 0.05}, false}, // Sum: 1.0
		{ScoringWeights{0.6, 0.3, 0.3, 0.0}, true},   // Sum: 1.2 (too high)
		{ScoringWeights{0.2, 0.1, 0.05, 0.05}, true}, // Sum: 0.4 (too low)
	}

	for _, tt := range tests {
		err := tt.weights.ValidateWeights()
		if tt.shouldErr && err == nil {
			t.Errorf("Expected validation error for weights %+v", tt.weights)
		}
		if !tt.shouldErr && err != nil {
			t.Errorf("Unexpected validation error for weights %+v: %v", tt.weights, err)
		}
	}
}

func TestScoringWeights_Normalize(t *testing.T) {
	tests := []struct {
		input    ScoringWeights
		expected ScoringWeights
	}{
		{
			ScoringWeights{0.4, 0.3, 0.2, 0.1},
			ScoringWeights{0.4, 0.3, 0.2, 0.1},
		},
		{
			ScoringWeights{0.5, 0.5, 0.0, 0.0},
			ScoringWeights{0.5, 0.5, 0.0, 0.0},
		},
		{
			ScoringWeights{0.8, 0.4, 0.2, 0.2}, // Sum: 1.6
			ScoringWeights{0.5, 0.25, 0.125, 0.125}, // Normalized to 1.0
		},
	}

	for i, tt := range tests {
		result := tt.input.Normalize()
		// Use delta comparison for floating point precision
		if !almostEqual(result.SkillCoverage, tt.expected.SkillCoverage, 0.001) ||
			!almostEqual(result.Experience, tt.expected.Experience, 0.001) ||
			!almostEqual(result.TermSimilarity, tt.expected.TermSimilarity, 0.001) ||
			!almostEqual(result.OverallMatch, tt.expected.OverallMatch, 0.001) {
			t.Errorf("Test %d: expected %+v, got %+v", i, tt.expected, result)
		}
	}
}

func TestNewScoringWeights(t *testing.T) {
	// Valid weights
	weights, err := NewScoringWeights(0.4, 0.3, 0.2, 0.1)
	if err != nil {
		t.Errorf("Expected no error for valid weights: %v", err)
	}
	if weights.SkillCoverage != 0.4 {
		t.Errorf("Expected SkillCoverage 0.4, got %f", weights.SkillCoverage)
	}

	// Invalid weights (don't sum to 1.0)
	_, err = NewScoringWeights(0.6, 0.3, 0.3, 0.0)
	if err == nil {
		t.Error("Expected error for invalid weights that don't sum to 1.0")
	}
}

func TestCalculateWeightedScore(t *testing.T) {
	weights := NewDefaultWeights()

	tests := []struct {
		name             string
		skillCoverage    float64
		experienceMatch  float64
		termSimilarity   float64
		overallMatch     float64
		expectedMin      int
		expectedMax      int
	}{
		{
			name:            "Perfect score",
			skillCoverage:   1.0,
			experienceMatch: 1.0,
			termSimilarity:  1.0,
			overallMatch:    1.0,
			expectedMin:     95,
			expectedMax:     100,
		},
		{
			name:            "Zero score",
			skillCoverage:   0.0,
			experienceMatch: 0.0,
			termSimilarity:  0.0,
			overallMatch:    0.0,
			expectedMin:     0,
			expectedMax:     0,
		},
		{
			name:            "Half score",
			skillCoverage:   0.5,
			experienceMatch: 0.5,
			termSimilarity:  0.5,
			overallMatch:    0.5,
			expectedMin:     48,
			expectedMax:     52,
		},
		{
			name:            "High skill coverage only",
			skillCoverage:   1.0,
			experienceMatch: 0.0,
			termSimilarity:  0.0,
			overallMatch:    0.0,
			expectedMin:     38,
			expectedMax:     42, // 40% * 1.0 = 40
		},
		{
			name:            "High experience only",
			skillCoverage:   0.0,
			experienceMatch: 1.0,
			termSimilarity:  0.0,
			overallMatch:    0.0,
			expectedMin:     28,
			expectedMax:     32, // 30% * 1.0 = 30
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, breakdown := CalculateWeightedScore(
				tt.skillCoverage,
				tt.experienceMatch,
				tt.termSimilarity,
				tt.overallMatch,
				weights,
			)

			if score < tt.expectedMin || score > tt.expectedMax {
				t.Errorf("Expected score between %d and %d, got %d", tt.expectedMin, tt.expectedMax, score)
			}

			if breakdown == nil {
				t.Error("Expected non-nil breakdown")
			} else if breakdown.WeightedTotal != score {
				t.Errorf("Breakdown total %d doesn't match score %d", breakdown.WeightedTotal, score)
			}
		})
	}
}

func TestCalculateWeightedScore_Clamping(t *testing.T) {
	weights := NewDefaultWeights()

	// Test with values outside 0-1 range (should be clamped)
	score, _ := CalculateWeightedScore(
		1.5, // Should clamp to 1.0
		-0.5, // Should clamp to 0.0
		0.5,
		0.5,
		weights,
	)

	// Expected: (1.0 * 0.4) + (0.0 * 0.3) + (0.5 * 0.2) + (0.5 * 0.1) = 0.4 + 0 + 0.1 + 0.05 = 0.55 = 55
	if score < 54 || score > 56 {
		t.Errorf("Expected score ~55, got %d", score)
	}
}

func TestCalculateWeightedScore_EdgeCases(t *testing.T) {
	// Test with custom weights
	customWeights := ScoringWeights{
		SkillCoverage:  0.5,
		Experience:     0.5,
		TermSimilarity: 0.0,
		OverallMatch:   0.0,
	}

	score, breakdown := CalculateWeightedScore(
		0.5,
		0.5,
		0.0,
		0.0,
		customWeights,
	)

	// Expected: (0.5 * 0.5) + (0.5 * 0.5) = 0.25 + 0.25 = 0.5 = 50
	if score != 50 {
		t.Errorf("Expected score 50 with custom weights, got %d", score)
	}

	if breakdown.WeightedTotal != 50 {
		t.Errorf("Breakdown total %d doesn't match expected 50", breakdown.WeightedTotal)
	}
}

func TestCalculateExperienceMatch(t *testing.T) {
	tests := []struct {
		name      string
		cvSkills  []Skill
		jdSkills  []Skill
		expected  float64
	}{
		{
			name: "Exact experience match",
			cvSkills: []Skill{
				{Name: "go", Experience: 3},
			},
			jdSkills: []Skill{
				{Name: "go", Experience: 3},
			},
			expected: 1.0,
		},
		{
			name: "CV has more experience",
			cvSkills: []Skill{
				{Name: "go", Experience: 5},
			},
			jdSkills: []Skill{
				{Name: "go", Experience: 3},
			},
			expected: 1.0,
		},
		{
			name: "CV has less experience (partial match)",
			cvSkills: []Skill{
				{Name: "go", Experience: 2},
			},
			jdSkills: []Skill{
				{Name: "go", Experience: 5},
			},
			expected: 0.4, // 2/5 = 0.4
		},
		{
			name: "JD doesn't specify experience, CV has experience",
			cvSkills: []Skill{
				{Name: "go", Experience: 3},
			},
			jdSkills: []Skill{
				{Name: "go", Experience: 0},
			},
			expected: 0.8, // Good match when CV has experience
		},
		{
			name: "JD doesn't specify experience, CV also has no experience",
			cvSkills: []Skill{
				{Name: "go", Experience: 0},
			},
			jdSkills: []Skill{
				{Name: "go", Experience: 0},
			},
			expected: 0.5, // Some experience assumed
		},
		{
			name: "Empty JD",
			cvSkills: []Skill{
				{Name: "go", Experience: 3},
			},
			jdSkills: []Skill{},
			expected: 0.0,
		},
		{
			name: "No matching skills",
			cvSkills: []Skill{
				{Name: "go", Experience: 3},
			},
			jdSkills: []Skill{
				{Name: "python", Experience: 3},
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateExperienceMatch(tt.cvSkills, tt.jdSkills)
			if result < tt.expected-0.01 || result > tt.expected+0.01 {
				t.Errorf("Expected %f, got %f", tt.expected, result)
			}
		})
	}
}

func TestCalculateSkillCoverage(t *testing.T) {
	tests := []struct {
		cvSkills []Skill
		jdSkills []Skill
		expected float64
	}{
		{
			cvSkills: []Skill{{Name: "go"}, {Name: "python"}},
			jdSkills: []Skill{{Name: "go"}, {Name: "python"}},
			expected: 1.0,
		},
		{
			cvSkills: []Skill{{Name: "go"}},
			jdSkills: []Skill{{Name: "go"}, {Name: "python"}},
			expected: 0.5,
		},
		{
			cvSkills: []Skill{{Name: "java"}},
			jdSkills: []Skill{{Name: "go"}, {Name: "python"}},
			expected: 0.0,
		},
		{
			cvSkills: []Skill{},
			jdSkills: []Skill{{Name: "go"}},
			expected: 0.0,
		},
		{
			cvSkills: []Skill{{Name: "go"}},
			jdSkills: []Skill{},
			expected: 0.0,
		},
	}

	for i, tt := range tests {
		result := CalculateSkillCoverage(tt.cvSkills, tt.jdSkills)
		if result != tt.expected {
			t.Errorf("Test %d: expected %f, got %f", i, tt.expected, result)
		}
	}
}

func TestCalculateTermSimilarity(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{0.5, 0.5},
		{1.0, 1.0},
		{0.0, 0.0},
		{1.5, 1.0}, // Clamped
		{-0.5, 0.0}, // Clamped
	}

	for _, tt := range tests {
		result := CalculateTermSimilarity(tt.input)
		if result != tt.expected {
			t.Errorf("CalculateTermSimilarity(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

func TestCalculateOverallMatch(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{0.5, 0.5},
		{1.0, 1.0},
		{0.0, 0.0},
		{1.5, 1.0}, // Clamped
		{-0.5, 0.0}, // Clamped
	}

	for _, tt := range tests {
		result := CalculateOverallMatch(tt.input)
		if result != tt.expected {
			t.Errorf("CalculateOverallMatch(%f) = %f, want %f", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeScoreToPercentage(t *testing.T) {
	tests := []struct {
		rawScore     float64
		maxPossible  float64
		expected     int
	}{
		{50, 100, 50},
		{75, 100, 75},
		{0, 100, 0},
		{100, 100, 100},
		{150, 200, 75},
		{0, 0, 0}, // Division by zero
	}

	for _, tt := range tests {
		result := NormalizeScoreToPercentage(tt.rawScore, tt.maxPossible)
		if result != tt.expected {
			t.Errorf("NormalizeScoreToPercentage(%f, %f) = %d, want %d",
				tt.rawScore, tt.maxPossible, result, tt.expected)
		}
	}
}

func TestScoreBreakdown_ToJSON(t *testing.T) {
	breakdown := &ScoreBreakdown{
		SkillCoverage:  0.4,
		Experience:     0.3,
		TermSimilarity: 0.2,
		OverallMatch:   0.1,
		WeightedTotal:  30,
	}

	jsonStr, err := breakdown.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	if jsonStr == "" {
		t.Error("JSON string should not be empty")
	}

	// Check that expected fields are present
	if !strings.Contains(jsonStr, "skill_coverage") {
		t.Error("JSON should contain skill_coverage")
	}
	if !strings.Contains(jsonStr, "experience") {
		t.Error("JSON should contain experience")
	}
	if !strings.Contains(jsonStr, "weighted_total") {
		t.Error("JSON should contain weighted_total")
	}
}

func TestScoringWeights_ToJSON(t *testing.T) {
	weights := NewDefaultWeights()

	jsonStr, err := weights.ToJSON()
	if err != nil {
		t.Fatalf("Failed to convert to JSON: %v", err)
	}

	if jsonStr == "" {
		t.Error("JSON string should not be empty")
	}

	// Check that expected fields are present
	if !strings.Contains(jsonStr, "skill_coverage") {
		t.Error("JSON should contain skill_coverage")
	}
	if !strings.Contains(jsonStr, "experience") {
		t.Error("JSON should contain experience")
	}
}

func TestClampFloat64(t *testing.T) {
	tests := []struct {
		value    float64
		min      float64
		max      float64
		expected float64
	}{
		{0.5, 0.0, 1.0, 0.5},
		{1.5, 0.0, 1.0, 1.0},
		{-0.5, 0.0, 1.0, 0.0},
		{0.0, 0.0, 1.0, 0.0},
		{1.0, 0.0, 1.0, 1.0},
	}

	for _, tt := range tests {
		result := clampFloat64(tt.value, tt.min, tt.max)
		if result != tt.expected {
			t.Errorf("clampFloat64(%f, %f, %f) = %f, want %f",
				tt.value, tt.min, tt.max, result, tt.expected)
		}
	}
}
