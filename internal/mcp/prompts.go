package mcp

import (
	"fmt"
)

// InterviewQuestionStyle represents the style of interview questions to generate
type InterviewQuestionStyle string

const (
	// InterviewStyleTechnical focuses on technical skills and knowledge
	InterviewStyleTechnical InterviewQuestionStyle = "technical"
	// InterviewStyleBehavioral focuses on soft skills and past experiences
	InterviewStyleBehavioral InterviewQuestionStyle = "behavioral"
	// InterviewStyleComprehensive balances technical and behavioral questions
	InterviewStyleComprehensive InterviewQuestionStyle = "comprehensive"
)

// BuildInterviewQuestionsPrompt creates a prompt for generating interview questions based on CV/JD gaps
func BuildInterviewQuestionsPrompt(cvURI, jdURI string, style InterviewQuestionStyle, count int) string {
	styleInstructions := ""
	switch style {
	case InterviewStyleTechnical:
		styleInstructions = "Focus on technical skills, tools, methodologies, and domain-specific knowledge."
	case InterviewStyleBehavioral:
		styleInstructions = "Focus on soft skills, teamwork, problem-solving approach, and past experiences."
	case InterviewStyleComprehensive:
		styleInstructions = "Balance technical and behavioral questions, covering both hard and soft skills."
	}

	return fmt.Sprintf(`You are an expert interviewer and career advisor. Your task is to generate targeted interview questions based on the gap analysis between a candidate's CV and a job description.

## Resources Available

Please use the following MCP resources to access the documents:
- CV: %s
- Job Description: %s

## Task

Generate %d interview questions that will help assess the candidate's fit for the role. The questions should be based on:
1. Gaps between the CV and job requirements
2. Areas where the CV lacks detail or evidence
3. Skills mentioned in the job description but not prominently featured in the CV
4. Potential concerns or areas needing clarification

## Question Style: %s

%s

## Guidelines

- Questions should be specific and targeted, not generic
- Reference specific skills, technologies, or experiences from the job description
- Focus on areas where the CV shows gaps or limited experience
- For technical questions: ask about specific tools, frameworks, or methodologies
- For behavioral questions: use the STAR method (Situation, Task, Action, Result) as a framework
- For comprehensive questions: balance technical depth with behavioral insights
- Make questions open-ended to encourage detailed responses
- Avoid yes/no questions

## Response Format

Provide your questions in a numbered list with brief context for why each question is relevant. For example:

1. **Question text** - [Context: Why this question matters based on gap analysis]

## Important Notes

- Use the cv:// and jd:// resources to read the actual content
- Be specific and cite evidence from the documents
- Generate exactly %d questions
- Detect the natural language of the job description and respond in that language
- Focus on objective analysis rather than subjective opinions

## Example Structure

1. **Technical Question** - [Context: JD requires React but CV shows limited frontend experience]
2. **Behavioral Question** - [Context: JD emphasizes team leadership but CV lacks leadership examples]
3. **Domain Question** - [Context: JD mentions specific industry knowledge not evident in CV]
... (continue for %d questions)`, cvURI, jdURI, count, style, styleInstructions, count, count)
}

// BuildQuickInterviewQuestionsPrompt creates a concise prompt for quick interview question generation
func BuildQuickInterviewQuestionsPrompt(cvURI, jdURI string, count int) string {
	return fmt.Sprintf(`Generate %d targeted interview questions based on CV (%s) and job description (%s).

Focus on:
1. Skills/experience gaps between CV and JD
2. Areas needing clarification
3. Technical and behavioral questions

Format as numbered list with brief context for each question.`, count, cvURI, jdURI)
}
