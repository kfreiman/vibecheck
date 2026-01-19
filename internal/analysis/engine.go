package analysis

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/blevesearch/bleve/v2"
	"github.com/blevesearch/bleve/v2/mapping"
	"github.com/blevesearch/bleve/v2/search/query"
)

var logger = slog.Default()

// TermScore represents a term with its BM25 score
type TermScore struct {
	Term  string  `json:"term"`
	Score float64 `json:"score"`
}

// AnalysisResult contains structured analysis output
type AnalysisResult struct {
	MatchPercentage  int             `json:"match_percentage"`
	WeightedScore    int             `json:"weighted_score"`
	SkillCoverage    float64         `json:"skill_coverage"`
	ExperienceMatch  float64         `json:"experience_match"`
	TopSkills        []string        `json:"top_skills"`
	MissingSkills    []string        `json:"missing_skills"`
	PresentSkills    []string        `json:"present_skills"`
	CommonTerms      []TermScore     `json:"common_terms"`
	ScoringBreakdown *ScoreBreakdown `json:"scoring_breakdown"`
}

// AnalysisEngine uses bleve BM25 for CV/JD matching
type AnalysisEngine struct {
	indexMapping mapping.IndexMapping
}

// NewAnalysisEngine creates a new analysis engine with bleve BM25 configuration
func NewAnalysisEngine() *AnalysisEngine {
	indexMapping := bleve.NewIndexMapping()
	return &AnalysisEngine{
		indexMapping: indexMapping,
	}
}

// preprocessText normalizes text for analysis
func preprocessText(text string) string {
	// Normalize: lowercase, trim whitespace
	text = strings.ToLower(text)
	text = strings.TrimSpace(text)
	// Remove multiple spaces
	text = strings.Join(strings.Fields(text), " ")
	return text
}

// Analyze performs BM25-based analysis between CV and JD with skill extraction
func (e *AnalysisEngine) Analyze(ctx context.Context, cvContent, jdContent string) (*AnalysisResult, error) {
	logger.DebugContext(ctx, "starting BM25 analysis with skill extraction",
		"cv_length", len(cvContent),
		"jd_length", len(jdContent),
	)

	// Preprocess content
	cvClean := preprocessText(cvContent)
	jdClean := preprocessText(jdContent)

	if cvClean == "" || jdClean == "" {
		return nil, fmt.Errorf("both CV and JD content must not be empty")
	}

	// Create unified index with both documents
	bleveIndex, err := bleve.NewMemOnly(e.indexMapping)
	if err != nil {
		return nil, fmt.Errorf("failed to create analysis index: %w", err)
	}
	defer func(c context.Context) {
		if closeErr := bleveIndex.Close(); closeErr != nil {
			logger.DebugContext(c, "error closing bleve index", "error", closeErr)
		}
	}(ctx)

	// Index both documents
	if err := bleveIndex.Index("cv", cvClean); err != nil {
		return nil, fmt.Errorf("failed to index CV: %w", err)
	}
	if err := bleveIndex.Index("jd", jdClean); err != nil {
		return nil, fmt.Errorf("failed to index JD: %w", err)
	}

	// Extract term frequencies using search queries (BM25-based)
	// This approach uses bleve's built-in BM25 scoring
	cvTerms := extractTermFrequenciesFromIndex(bleveIndex, "cv")
	jdTerms := extractTermFrequenciesFromIndex(bleveIndex, "jd")

	// Extract skills using dictionary-based matching
	skillsDict := NewSkillsDictionary()
	cvSkills := ExtractSkills(ctx, cvClean, skillsDict)
	jdSkills := ExtractSkills(ctx, jdClean, skillsDict)

	// Calculate match metrics using BM25 scores
	result := e.calculateMatchMetrics(cvTerms, jdTerms)

	// Calculate skill-based metrics
	skillCoverage := CalculateSkillCoverage(cvSkills, jdSkills)
	experienceMatch := CalculateExperienceMatch(cvSkills, jdSkills)

	// Calculate term similarity from BM25 (normalized 0-1)
	termSimilarity := 0.0
	if len(jdTerms) > 0 {
		termSimilarity = float64(result.MatchPercentage) / 100.0
	}

	// Calculate overall match from BM25 (normalized 0-1)
	overallMatch := float64(result.MatchPercentage) / 100.0

	// Calculate weighted score using default weights
	weightedScore, breakdown := CalculateWeightedScore(
		skillCoverage,
		experienceMatch,
		termSimilarity,
		overallMatch,
		NewDefaultWeights(),
	)

	// Update result with skill-based metrics
	result.WeightedScore = weightedScore
	result.ExperienceMatch = experienceMatch
	result.SkillCoverage = skillCoverage
	result.ScoringBreakdown = breakdown

	// Add present skills (skills that CV has that JD needs)
	presentSkills := make([]string, 0, len(cvSkills))
	for _, skill := range cvSkills {
		presentSkills = append(presentSkills, skill.Name)
	}
	result.PresentSkills = presentSkills

	logger.DebugContext(ctx, "BM25 analysis complete",
		"match_percentage", result.MatchPercentage,
		"weighted_score", result.WeightedScore,
		"skill_coverage", result.SkillCoverage,
		"experience_match", result.ExperienceMatch,
		"top_skills", len(result.TopSkills),
		"missing_skills", len(result.MissingSkills),
		"present_skills", len(result.PresentSkills),
	)

	return result, nil
}

// extractTermFrequenciesFromIndex extracts term frequencies for a specific document
// using bleve's BM25 scoring. This uses search queries to get BM25-weighted term scores.
func extractTermFrequenciesFromIndex(bleveIndex bleve.Index, docID string) map[string]float64 {
	terms := make(map[string]float64)

	// Get all unique terms from the index
	fields, fieldsErr := bleveIndex.Fields()
	if fieldsErr != nil {
		return terms
	}
	for _, field := range fields {
		dict, err := bleveIndex.FieldDict(field)
		if err != nil {
			continue
		}

		for {
			entry, err := dict.Next()
			if err != nil || entry == nil {
				break
			}

			term := entry.Term

			// Use match query to get BM25 score for this term in the specific document
			q := query.NewMatchQuery(term)
			searchReq := bleve.NewSearchRequest(q)
			searchReq.Size = 10
			results, searchErr := bleveIndex.Search(searchReq)
			if searchErr != nil {
				continue
			}

			// Find the specific document and extract its BM25 score for this term
			for _, hit := range results.Hits {
				if hit.ID == docID {
					terms[term] = hit.Score
					break
				}
			}
		}
		if closeErr := dict.Close(); closeErr != nil {
			logger.DebugContext(context.Background(), "error closing field dictionary", "error", closeErr)
		}
	}

	return terms
}

// calculateMatchMetrics computes all match metrics from term data
func (e *AnalysisEngine) calculateMatchMetrics(cvTerms, jdTerms map[string]float64) *AnalysisResult {
	result := &AnalysisResult{}

	// Find common terms and calculate coverage
	commonTerms := make(map[string]float64)
	missingTerms := make([]string, 0)
	jdTotalWeight := 0.0

	// Calculate JD total weight
	for term, weight := range jdTerms {
		jdTotalWeight += weight
		if cvWeight, exists := cvTerms[term]; exists {
			// Term exists in both - calculate BM25-like score
			// Use the min of frequencies as a simple match metric
			matchScore := minFloat64(cvWeight, weight)
			commonTerms[term] = matchScore
		} else {
			// Term in JD but not CV
			missingTerms = append(missingTerms, term)
		}
	}

	// Calculate skill coverage (terms matched / total JD terms)
	if len(jdTerms) > 0 {
		result.SkillCoverage = float64(len(commonTerms)) / float64(len(jdTerms))
	} else {
		result.SkillCoverage = 0.0
	}

	// Calculate match percentage
	// Weighted by term frequencies
	matchScore := 0.0
	for _, score := range commonTerms {
		matchScore += score
	}

	if jdTotalWeight > 0 {
		result.MatchPercentage = int((matchScore / jdTotalWeight) * 100)
	} else {
		result.MatchPercentage = 0
	}

	// Ensure percentage is in valid range
	if result.MatchPercentage > 100 {
		result.MatchPercentage = 100
	}
	if result.MatchPercentage < 0 {
		result.MatchPercentage = 0
	}

	// Get top skills (top common terms by score)
	result.TopSkills = e.getTopTerms(commonTerms, 10)

	// Get missing skills (sorted by JD weight)
	result.MissingSkills = e.getMissingSkills(jdTerms, missingTerms, 20)

	// Get common terms with scores for detailed analysis
	result.CommonTerms = e.getTermScores(commonTerms, 15)

	return result
}

// getTopTerms returns top N terms by score
func (e *AnalysisEngine) getTopTerms(terms map[string]float64, limit int) []string {
	type termScore struct {
		term  string
		score float64
	}

	scores := make([]termScore, 0, len(terms))
	for term, score := range terms {
		scores = append(scores, termScore{term: term, score: score})
	}

	// Sort by score descending
	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	// Get top N
	if limit > len(scores) {
		limit = len(scores)
	}
	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = scores[i].term
	}

	return result
}

// getMissingSkills returns missing skills sorted by JD weight
func (e *AnalysisEngine) getMissingSkills(jdTerms map[string]float64, missing []string, limit int) []string {
	type termScore struct {
		term  string
		score float64
	}

	scores := make([]termScore, 0, len(missing))
	for _, term := range missing {
		if score, exists := jdTerms[term]; exists {
			scores = append(scores, termScore{term: term, score: score})
		}
	}

	// Sort by score descending
	for i := 0; i < len(scores)-1; i++ {
		for j := i + 1; j < len(scores); j++ {
			if scores[j].score > scores[i].score {
				scores[i], scores[j] = scores[j], scores[i]
			}
		}
	}

	// Get top N
	if limit > len(scores) {
		limit = len(scores)
	}
	result := make([]string, limit)
	for i := 0; i < limit; i++ {
		result[i] = scores[i].term
	}

	return result
}

// getTermScores returns TermScore objects for detailed analysis
func (e *AnalysisEngine) getTermScores(terms map[string]float64, limit int) []TermScore {
	result := make([]TermScore, 0, len(terms))
	for term, score := range terms {
		result = append(result, TermScore{Term: term, Score: score})
	}

	// Sort by score descending
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Score > result[i].Score {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	// Limit results
	if limit < len(result) {
		result = result[:limit]
	}

	return result
}

// minFloat64 returns the minimum of two float64 values
func minFloat64(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
