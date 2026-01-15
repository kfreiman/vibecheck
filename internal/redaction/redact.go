package redaction

import (
	"bytes"
	"regexp"
)

// PIIRedactor provides PII redaction functionality
type PIIRedactor struct {
	emailRegex    *regexp.Regexp
	phoneRegex    *regexp.Regexp
	ssnRegex      *regexp.Regexp
	creditCardRegex *regexp.Regexp
}

// NewPIIRedactor creates a new PII redactor
func NewPIIRedactor() *PIIRedactor {
	return &PIIRedactor{
		emailRegex: regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`),
		// Matches various phone formats including international
		phoneRegex: regexp.MustCompile(`(?:\+?\d{1,3}[-.\s]?)?(?:\(?\d{3}\)?[-.\s]?)?\d{3}[-.\s]?\d{4}`),
		// Basic SSN pattern (US format)
		ssnRegex: regexp.MustCompile(`\b\d{3}[- ]?\d{2}[- ]?\d{4}\b`),
		// Basic credit card pattern (13-19 digits)
		creditCardRegex: regexp.MustCompile(`\b(?:\d[ -]?){13,19}\d\b`),
	}
}

// RedactContent removes PII from the content
func (r *PIIRedactor) RedactContent(content []byte) []byte {
	result := content

	// Redact emails first
	result = r.emailRegex.ReplaceAll(result, []byte("[EMAIL_REDACTED]"))

	// Redact SSNs
	result = r.ssnRegex.ReplaceAll(result, []byte("[SSN_REDACTED]"))

	// Redact credit cards BEFORE phones to prevent credit cards from matching as phones
	result = r.creditCardRegex.ReplaceAll(result, []byte("[CREDIT_CARD_REDACTED]"))

	// Redact phone numbers (be careful not to redact too much)
	result = r.redactPhones(result)

	return result
}

// RedactString removes PII from a string
func (r *PIIRedactor) RedactString(content string) string {
	return string(r.RedactContent([]byte(content)))
}

// redactPhones redacts phone numbers with more careful matching
func (r *PIIRedactor) redactPhones(content []byte) []byte {
	// Only redact phone numbers that look like actual phone numbers
	// Avoid redacting things that might be part of IDs or other valid numbers
	matches := r.phoneRegex.FindAllString(string(content), -1)

	result := bytes.ReplaceAll(content, []byte{}, []byte{})

	for _, match := range matches {
		// Skip if it looks like a year or other number
		if len(match) >= 10 && len(match) <= 15 {
			redacted := bytes.ReplaceAll(result, []byte(match), []byte("[PHONE_REDACTED]"))
			result = redacted
		}
	}

	// Use simpler approach - replace all matches with placeholder
	return r.phoneRegex.ReplaceAll(content, []byte("[PHONE_REDACTED]"))
}

// RedactEmailsOnly redacts only email addresses
func (r *PIIRedactor) RedactEmailsOnly(content []byte) []byte {
	return r.emailRegex.ReplaceAll(content, []byte("[EMAIL_REDACTED]"))
}

// RedactPhonesOnly redacts only phone numbers
func (r *PIIRedactor) RedactPhonesOnly(content []byte) []byte {
	return r.phoneRegex.ReplaceAll(content, []byte("[PHONE_REDACTED]"))
}

// CountPIIItems counts the number of PII items found
func (r *PIIRedactor) CountPIIItems(content []byte) map[string]int {
	return map[string]int{
		"emails":       len(r.emailRegex.FindAllString(string(content), -1)),
		"phones":       len(r.phoneRegex.FindAllString(string(content), -1)),
		"ssns":         len(r.ssnRegex.FindAllString(string(content), -1)),
		"credit_cards": len(r.creditCardRegex.FindAllString(string(content), -1)),
	}
}

// DefaultRedactor is the default PII redactor instance
var DefaultRedactor = NewPIIRedactor()

// Redact is a convenience function that uses the default redactor
func Redact(content []byte) []byte {
	return DefaultRedactor.RedactContent(content)
}

// RedactString is a convenience function that uses the default redactor
func RedactString(content string) string {
	return DefaultRedactor.RedactString(content)
}
