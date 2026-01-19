package analysis

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Skill represents a detected skill from CV/JD content
type Skill struct {
	Name       string  `json:"name"`        // Skill name (e.g., "Go", "Python")
	Category   string  `json:"category"`    // Category (e.g., "language", "framework", "database")
	Experience int     `json:"experience"`  // Years mentioned (0 if not specified)
	Confidence float64 `json:"confidence"`  // 0.0-1.0 confidence score
}

// SkillsDictionary provides skill matching capabilities
type SkillsDictionary struct {
	skillsByCategory map[string][]string
	skillIndex       map[string]string // normalized skill name -> category
	once             sync.Once
}

// NewSkillsDictionary creates and loads the skills dictionary
func NewSkillsDictionary() *SkillsDictionary {
	sd := &SkillsDictionary{
		skillsByCategory: make(map[string][]string),
		skillIndex:       make(map[string]string),
	}
	sd.loadDictionary()
	return sd
}

// loadDictionary loads skills from the dictionary file
func (sd *SkillsDictionary) loadDictionary() {
	sd.once.Do(func() {
		// Navigate from internal/analysis to project root (testdata/)
		dictPath := filepath.Join("..", "..", "testdata", "skills_dictionary.txt")
		file, err := os.Open(dictPath)
		if err != nil {
			slog.Warn("failed to load skills dictionary, using empty dict",
				"error", err,
				"path", dictPath,
			)
			return
		}
		defer file.Close()

		scanner := bufio.NewScanner(file)
		var currentCategory string

		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}

			// Check if it's a category header (e.g., "# Programming Languages")
			if strings.HasPrefix(line, "# ") {
				currentCategory = strings.TrimSpace(line[2:])
				continue
			}

			// Skip regular comments
			if strings.HasPrefix(line, "#") {
				continue
			}

			// Add skill to category
			if currentCategory != "" {
				sd.skillsByCategory[currentCategory] = append(sd.skillsByCategory[currentCategory], line)
				// Index by normalized name
				normalized := strings.ToLower(line)
				sd.skillIndex[normalized] = currentCategory
			}
		}
	})
}

// FindSkill looks up a skill in the dictionary and returns category if found
func (sd *SkillsDictionary) FindSkill(skillName string) (category string, found bool) {
	normalized := strings.ToLower(strings.TrimSpace(skillName))
	category, found = sd.skillIndex[normalized]
	return
}

// ExtractSkills extracts skills from content using dictionary matching
// This simulates LLM-based langextractor by using dictionary-based matching
// with additional context analysis
func ExtractSkills(ctx context.Context, content string, dict *SkillsDictionary) []Skill {
	logger.DebugContext(ctx, "extracting skills from content", "content_length", len(content))

	if dict == nil {
		dict = NewSkillsDictionary()
	}

	content = strings.ToLower(content)
	words := tokenizeContent(content)

	// Track seen skills to avoid duplicates
	seen := make(map[string]bool)
	var skills []Skill

	// Extract skills from words
	for _, word := range words {
		// Check if word is in dictionary
		category, found := dict.FindSkill(word)
		if found && !seen[word] {
			// Calculate confidence based on context
			confidence := calculateConfidence(word, content)
			experience := extractExperience(word, content)

			skill := Skill{
				Name:       word,
				Category:   category,
				Experience: experience,
				Confidence: confidence,
			}

			skills = append(skills, skill)
			seen[word] = true
		}
	}

	// Sort by confidence descending
	sort.Slice(skills, func(i, j int) bool {
		if skills[i].Confidence != skills[j].Confidence {
			return skills[i].Confidence > skills[j].Confidence
		}
		return skills[i].Name < skills[j].Name
	})

	logger.DebugContext(ctx, "extracted skills",
		"count", len(skills),
		"skills", fmt.Sprintf("%v", skills),
	)

	return skills
}

// tokenizeContent splits content into words/tokens
func tokenizeContent(content string) []string {
	content = strings.ToLower(content)
	// Split by common delimiters
	delimiters := []string{",", ";", ".", ":", "(", ")", "[", "]", "{", "}", " ", "\n", "\t"}
	for _, delim := range delimiters {
		content = strings.ReplaceAll(content, delim, " ")
	}

	// Split and filter
	rawWords := strings.Fields(content)
	var words []string

	// Filter short words and common noise words
	noiseWords := map[string]bool{
		"the": true, "and": true, "with": true, "for": true, "to": true,
		"in": true, "on": true, "of": true, "a": true, "an": true,
		"is": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "have": true, "has": true, "had": true, "do": true,
		"does": true, "did": true, "will": true, "would": true, "could": true,
		"should": true, "may": true, "might": true, "must": true, "can": true,
		"about": true, "from": true, "by": true, "as": true, "at": true,
		"that": true, "this": true, "it": true, "its": true, "their": true,
		"them": true, "they": true, "we": true, "our": true, "you": true,
		"your": true, "he": true, "she": true, "his": true, "her": true,
		"i": true, "me": true, "my": true, "or": true,
	}

	for _, word := range rawWords {
		if len(word) >= 2 && !noiseWords[word] {
			words = append(words, word)
		}
	}

	return words
}

// calculateConfidence calculates confidence score for a skill
// This uses heuristics to simulate LLM-based confidence scoring
func calculateConfidence(skill, content string) float64 {
	baseConfidence := 0.5

	// Boost confidence if skill appears multiple times
	count := strings.Count(content, skill)
	if count > 0 {
		baseConfidence += float64(count) * 0.1
	}

	// Boost confidence if in skill-specific context
	contexts := []string{
		"experience with", "proficient in", "skilled in", "knowledge of",
		"worked with", "built using", "developed with", "implemented using",
	}
	for _, ctx := range contexts {
		if strings.Contains(content, ctx+" "+skill) {
			baseConfidence += 0.2
		}
	}

	// Cap at 1.0
	if baseConfidence > 1.0 {
		baseConfidence = 1.0
	}

	return baseConfidence
}

// extractExperience extracts years of experience for a skill
// Looks for patterns like "5 years of Go experience" or "Go (3 years)"
func extractExperience(skill, content string) int {
	// Extract numeric patterns near the skill
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, skill) {
			// Look for years pattern
			if strings.Contains(line, "year") {
				words := strings.Fields(line)
				for i, word := range words {
					if word == skill {
						// Check previous few words for number
						// Pattern: "5 years of go", "go with 3 years"
						start := i - 5
						if start < 0 {
							start = 0
						}
						for k := start; k < i; k++ {
							if years := parseInt(words[k]); years > 0 {
								return years
							}
						}
						// Check next few words
						end := i + 5
						if end > len(words) {
							end = len(words)
						}
						for j := i + 1; j < end; j++ {
							if years := parseInt(words[j]); years > 0 {
								return years
							}
						}
					}
				}
			}
		}
	}

	return 0 // No experience specified
}

// parseInt extracts first integer from a string
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	var numStr strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			numStr.WriteRune(r)
		} else if numStr.Len() > 0 {
			break
		}
	}

	if numStr.Len() == 0 {
		return 0
	}

	var result int
	for _, r := range numStr.String() {
		result = result*10 + int(r-'0')
	}
	return result
}

// MatchSkills compares CV skills against JD skills
func MatchSkills(cvSkills, jdSkills []Skill) (matches []Skill, missing []Skill, partialMatches []Skill) {
	cvIndex := make(map[string]Skill)
	for _, skill := range cvSkills {
		cvIndex[skill.Name] = skill
	}

	jdIndex := make(map[string]Skill)
	for _, skill := range jdSkills {
		jdIndex[skill.Name] = skill
	}

	// Find exact matches
	for _, jdSkill := range jdSkills {
		if cvSkill, exists := cvIndex[jdSkill.Name]; exists {
			// Calculate match confidence
			matchConfidence := (cvSkill.Confidence + jdSkill.Confidence) / 2
			matchedSkill := Skill{
				Name:       jdSkill.Name,
				Category:   jdSkill.Category,
				Experience: cvSkill.Experience,
				Confidence: matchConfidence,
			}
			matches = append(matches, matchedSkill)
		} else {
			missing = append(missing, jdSkill)
		}
	}

	// For partial matching, we could use fuzzy matching or synonym detection
	// For now, we rely on exact matches via dictionary
	_ = partialMatches // Placeholder for future enhancement

	return matches, missing, partialMatches
}

// CalculateSkillCoverage calculates skill coverage percentage
func CalculateSkillCoverage(cvSkills, jdSkills []Skill) float64 {
	if len(jdSkills) == 0 {
		return 0.0
	}

	matches, _, _ := MatchSkills(cvSkills, jdSkills)
	return float64(len(matches)) / float64(len(jdSkills))
}

// LoadSkillsDictionary loads skills dictionary from file
func LoadSkillsDictionary() ([]string, error) {
	dictPath := filepath.Join("..", "..", "testdata", "skills_dictionary.txt")
	file, err := os.Open(dictPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open skills dictionary: %w", err)
	}
	defer file.Close()

	var skills []string
	scanner := bufio.NewScanner(file)
	var currentCategory string

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "# ") {
				currentCategory = strings.TrimSpace(line[2:])
			}
			continue
		}

		if currentCategory != "" {
			skills = append(skills, line)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read skills dictionary: %w", err)
	}

	return skills, nil
}

// SkillExtractionResult represents the result of skill extraction
type SkillExtractionResult struct {
	Skills       []Skill `json:"skills"`
	TotalSkills  int     `json:"total_skills"`
	UniqueSkills int     `json:"unique_skills"`
}

// ToJSON converts skill extraction result to JSON
func (s *SkillExtractionResult) ToJSON() (string, error) {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal skill result: %w", err)
	}
	return string(data), nil
}
