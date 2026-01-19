package analysis

import (
	"context"
	"encoding/json"
	"fmt"
)

// ScoringWeights defines weights for different scoring dimensions
// Weights should sum to 1.0 for normalized scoring
type ScoringWeights struct {
	SkillCoverage  float64 `json:"skill_coverage"`  // Default: 0.40 (40%)
	Experience     float64 `json:"experience"`      // Default: 0.30 (30%)
	TermSimilarity float64 `json:"term_similarity"` // Default: 0.20 (20%)
	OverallMatch   float64 `json:"overall_match"`   // Default: 0.10 (10%)
}

// ScoreBreakdown provides detailed scoring information
type ScoreBreakdown struct {
	SkillCoverage  float64 `json:"skill_coverage"`
	Experience     float64 `json:"experience_match"`
	TermSimilarity float64 `json:"term_similarity"`
	OverallMatch   float64 `json:"overall_match"`
	WeightedTotal  int     `json:"weighted_total"`
}

// NewDefaultWeights creates scoring weights with standard defaults
// Skill coverage: 40%, Experience: 30%, Term similarity: 20%, Overall match: 10%
func NewDefaultWeights() ScoringWeights {
	return ScoringWeights{
		SkillCoverage:  0.40,
		Experience:     0.30,
		TermSimilarity: 0.20,
		OverallMatch:   0.10,
	}
}

// ValidateWeights checks if weights sum to 1.0 (within tolerance)
func (w ScoringWeights) ValidateWeights() error {
	sum := w.SkillCoverage + w.Experience + w.TermSimilarity + w.OverallMatch
	if sum < 0.99 || sum > 1.01 {
		return fmt.Errorf("weights must sum to 1.0, got %.3f", sum)
	}
	return nil
}

// Normalize ensures weights sum to 1.0
func (w ScoringWeights) Normalize() ScoringWeights {
	sum := w.SkillCoverage + w.Experience + w.TermSimilarity + w.OverallMatch
	if sum == 0 {
		return NewDefaultWeights()
	}
	return ScoringWeights{
		SkillCoverage:  w.SkillCoverage / sum,
		Experience:     w.Experience / sum,
		TermSimilarity: w.TermSimilarity / sum,
		OverallMatch:   w.OverallMatch / sum,
	}
}

// CalculateWeightedScore computes a weighted score from multiple dimensions
// All inputs should be in 0.0-1.0 range
// Returns score in 0-100 range
func CalculateWeightedScore(
	skillCoverage float64,
	experienceMatch float64,
	termSimilarity float64,
	overallMatch float64,
	weights ScoringWeights,
) (int, *ScoreBreakdown) {
	// Ensure weights are normalized
	normalized := weights.Normalize()

	// Clamp inputs to 0.0-1.0 range
	skillCoverage = clampFloat64(skillCoverage, 0.0, 1.0)
	experienceMatch = clampFloat64(experienceMatch, 0.0, 1.0)
	termSimilarity = clampFloat64(termSimilarity, 0.0, 1.0)
	overallMatch = clampFloat64(overallMatch, 0.0, 1.0)

	// Calculate weighted total
	total := (skillCoverage * normalized.SkillCoverage) +
		(experienceMatch * normalized.Experience) +
		(termSimilarity * normalized.TermSimilarity) +
		(overallMatch * normalized.OverallMatch)

	// Convert to 0-100 scale and round
	score := int(total * 100)

	// Clamp to valid range
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	breakdown := &ScoreBreakdown{
		SkillCoverage:  skillCoverage,
		Experience:     experienceMatch,
		TermSimilarity: termSimilarity,
		OverallMatch:   overallMatch,
		WeightedTotal:  score,
	}

	return score, breakdown
}

// CalculateExperienceMatch computes experience match score
// Higher score = better match between CV and JD experience requirements
func CalculateExperienceMatch(cvSkills, jdSkills []Skill) float64 {
	if len(jdSkills) == 0 {
		return 0.0
	}

	matches, _, _ := MatchSkills(cvSkills, jdSkills)
	if len(matches) == 0 {
		return 0.0
	}

	totalScore := 0.0
	totalPossible := 0.0

	for _, jdSkill := range jdSkills {
		for _, cvSkill := range matches {
			if cvSkill.Name == jdSkill.Name {
				// Calculate experience match
				// If JD specifies experience, compare with CV
				// If JD doesn't specify, any CV experience counts as match
				if jdSkill.Experience > 0 {
					if cvSkill.Experience >= jdSkill.Experience {
						totalScore += 1.0
					} else if cvSkill.Experience > 0 {
						// Partial match: ratio of CV experience to JD requirement
						totalScore += float64(cvSkill.Experience) / float64(jdSkill.Experience)
					}
				} else {
					// JD doesn't specify experience requirement
					if cvSkill.Experience > 0 {
						totalScore += 0.8 // Good match
					} else {
						totalScore += 0.5 // Some experience (assumed)
					}
				}
				totalPossible += 1.0
				break
			}
		}
	}

	if totalPossible == 0 {
		return 0.0
	}

	return totalScore / totalPossible
}

// CalculateTermSimilarity computes term-based similarity score
// Uses the provided similarity score directly (from LLM or BM25 analysis)
func CalculateTermSimilarity(termSimilarityScore float64) float64 {
	// Term similarity score should already be in 0.0-1.0 range
	// This function is primarily for future extensibility
	return clampFloat64(termSimilarityScore, 0.0, 1.0)
}

// CalculateOverallMatch computes overall document match score
// Uses the provided overall match score from LLM or BM25 analysis
func CalculateOverallMatch(overallMatchScore float64) float64 {
	// Overall match score should already be in 0.0-1.0 range
	// This function is primarily for future extensibility
	return clampFloat64(overallMatchScore, 0.0, 1.0)
}

// CalculateSkillCoverage computes skill coverage percentage
// Returns value in 0.0-1.0 range
func CalculateSkillCoverage(cvSkills, jdSkills []Skill) float64 {
	if len(jdSkills) == 0 {
		return 0.0
	}

	matches, _, _ := MatchSkills(cvSkills, jdSkills)
	return float64(len(matches)) / float64(len(jdSkills))
}

// clampFloat64 clamps a float64 value to the given range
func clampFloat64(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// NormalizeScoreToPercentage converts a raw score to percentage
func NormalizeScoreToPercentage(rawScore, maxPossible float64) int {
	if maxPossible == 0 {
		return 0
	}
	percentage := int((rawScore / maxPossible) * 100)
	if percentage < 0 {
		return 0
	}
	if percentage > 100 {
		return 100
	}
	return percentage
}

// ToJSON converts ScoreBreakdown to JSON
func (s *ScoreBreakdown) ToJSON() (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal score breakdown: %w", err)
	}
	return string(data), nil
}

// ToJSON converts ScoringWeights to JSON
func (w ScoringWeights) ToJSON() (string, error) {
	data, err := json.MarshalIndent(w, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal scoring weights: %w", err)
	}
	return string(data), nil
}

// NewScoringWeights creates custom scoring weights
func NewScoringWeights(skillCoverage, experience, termSimilarity, overallMatch float64) (ScoringWeights, error) {
	weights := ScoringWeights{
		SkillCoverage:  skillCoverage,
		Experience:     experience,
		TermSimilarity: termSimilarity,
		OverallMatch:   overallMatch,
	}

	if err := weights.ValidateWeights(); err != nil {
		return weights, err
	}

	return weights, nil
}

// LogDebug logs scoring details for debugging
func (s *ScoreBreakdown) LogDebug(ctx context.Context, msg string) {
	logger.DebugContext(ctx, msg,
		"skill_coverage", s.SkillCoverage,
		"experience_match", s.Experience,
		"term_similarity", s.TermSimilarity,
		"overall_match", s.OverallMatch,
		"weighted_total", s.WeightedTotal,
	)
}
