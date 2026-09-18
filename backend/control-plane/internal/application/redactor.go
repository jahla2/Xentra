package application

import (
	"regexp"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type EvidenceRedactor struct {
	rules []redactionRule
}

type redactionRule struct {
	pattern     *regexp.Regexp
	replacement string
}

func NewEvidenceRedactor() *EvidenceRedactor {
	return &EvidenceRedactor{rules: []redactionRule{
		{regexp.MustCompile(`(?s)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----.*?-----END [A-Z0-9 ]*PRIVATE KEY-----`), "[REDACTED_PRIVATE_KEY]"},
		{regexp.MustCompile(`(?i)(authorization\s*:\s*bearer\s+)[A-Za-z0-9._~+/=-]+`), `1[REDACTED]`},
		{regexp.MustCompile(`(?i)\b(password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key|secret[_-]?key|client[_-]?secret)\b(\s*[:=]\s*)("[^"]*"|'[^']*'|[^\s,;]+)`), `12[REDACTED]`},
		{regexp.MustCompile(`(?i)\b(postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis|amqp)://([^:@/\s]+):([^@/\s]+)@`), `1://2:[REDACTED]@`},
		{regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`), "[REDACTED_AWS_ACCESS_KEY]"},
		{regexp.MustCompile(`\bgh[pousr]_[A-Za-z0-9]{20,}\b`), "[REDACTED_GITHUB_TOKEN]"},
		{regexp.MustCompile(`\bsk-(?:proj-)?[A-Za-z0-9_-]{16,}\b`), "[REDACTED_API_KEY]"},
		{regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), "[REDACTED_JWT]"},
	}}
}

func (r *EvidenceRedactor) Redact(value string) string {
	if r == nil {
		return value
	}
	for _, rule := range r.rules {
		value = rule.pattern.ReplaceAllString(value, rule.replacement)
	}
	return value
}

func (r *EvidenceRedactor) RedactEvidence(items []domain.Evidence) []domain.Evidence {
	result := make([]domain.Evidence, len(items))
	for index, item := range items {
		item.Output = r.Redact(item.Output)
		result[index] = item
	}
	return result
}
