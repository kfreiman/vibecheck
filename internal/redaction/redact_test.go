package redaction

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedact_Emails(t *testing.T) {
	content := []byte("Contact me at john.doe@example.com or support@company.org")
	redacted := Redact(content)

	assert.NotContains(t, string(redacted), "john.doe@example.com")
	assert.NotContains(t, string(redacted), "support@company.org")
	assert.Contains(t, string(redacted), "[EMAIL_REDACTED]")
}

func TestRedact_Phones(t *testing.T) {
	content := []byte("Call me at 555-123-4567 or +1 (800) 555-0123")
	redacted := Redact(content)

	assert.NotContains(t, string(redacted), "555-123-4567")
	assert.NotContains(t, string(redacted), "800")
	assert.Contains(t, string(redacted), "[PHONE_REDACTED]")
}

func TestRedact_SSNs(t *testing.T) {
	content := []byte("SSN: 123-45-6789")
	redacted := Redact(content)

	assert.NotContains(t, string(redacted), "123-45-6789")
	assert.Contains(t, string(redacted), "[SSN_REDACTED]")
}

func TestRedact_CreditCards(t *testing.T) {
	// Use a format that's clearly not a phone number
	content := []byte("Card number: 4111111111111111")
	redacted := Redact(content)

	assert.NotContains(t, string(redacted), "4111111111111111")
	assert.Contains(t, string(redacted), "[CREDIT_CARD_REDACTED]")
}

func TestRedact_NoPII(t *testing.T) {
	content := []byte("This is a normal text without any personal information")
	redacted := Redact(content)

	assert.Equal(t, string(content), string(redacted))
}

func TestRedact_MixedContent(t *testing.T) {
	content := []byte(`
Name: John Doe
Email: john@example.com
Phone: 555-123-4567
Experience: 5 years as a developer
`)
	redacted := Redact(content)

	assert.NotContains(t, string(redacted), "john@example.com")
	assert.NotContains(t, string(redacted), "555-123-4567")
	assert.Contains(t, string(redacted), "John Doe") // Names aren't redacted
	assert.Contains(t, string(redacted), "5 years as a developer")
}

func TestRedactString(t *testing.T) {
	content := "Email: test@example.com"
	redacted := RedactString(content)

	assert.NotContains(t, redacted, "test@example.com")
	assert.Contains(t, redacted, "[EMAIL_REDACTED]")
}

func TestCountPIIItems(t *testing.T) {
	redactor := NewPIIRedactor()
	content := []byte("Emails: a@test.com, b@test.com. Phone: 555-123-4567")

	counts := redactor.CountPIIItems(content)

	assert.Equal(t, 2, counts["emails"])
	assert.Equal(t, 1, counts["phones"])
	assert.Equal(t, 0, counts["ssns"])
	assert.Equal(t, 0, counts["credit_cards"])
}

func TestDefaultRedactor(t *testing.T) {
	// Verify default redactor is initialized
	assert.NotNil(t, DefaultRedactor)
}

func TestRedactEmailsOnly(t *testing.T) {
	content := []byte("Email: test@example.com, Phone: 555-1234")
	redacted := DefaultRedactor.RedactEmailsOnly(content)

	assert.NotContains(t, string(redacted), "test@example.com")
	assert.Contains(t, string(redacted), "555-1234") // Phone not redacted
}

func TestRedactPhonesOnly(t *testing.T) {
	content := []byte("Email: test@example.com, Phone: 555-1234")
	redacted := DefaultRedactor.RedactPhonesOnly(content)

	assert.Contains(t, string(redacted), "test@example.com") // Email not redacted
	assert.NotContains(t, string(redacted), "555-1234")
}
