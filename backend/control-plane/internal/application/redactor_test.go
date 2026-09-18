package application

import (
	"strings"
	"testing"
)

func TestEvidenceRedactorRemovesCommonSecrets(t *testing.T) {
	redactor := NewEvidenceRedactor()
	inputs := []struct {
		input  string
		secret string
	}{
		{"password=super-secret-password", "super-secret-password"},
		{"Authorization: Bearer abc.def.ghi", "abc.def.ghi"},
		{"postgres://admin:db-password@postgres:5432/app", "db-password"},
		{"AWS_ACCESS_KEY=AKIA1234567890ABCDEF", "AKIA1234567890ABCDEF"},
		{"token=ghp_abcdefghijklmnopqrstuvwxyz123456", "ghp_abcdefghijklmnopqrstuvwxyz123456"},
		{"api_key=sk-proj-abcdefghijklmnopqrstuv", "sk-proj-abcdefghijklmnopqrstuv"},
		{"-----BEGIN OPENSSH PRIVATE KEY-----\nsecret-key-material\n-----END OPENSSH PRIVATE KEY-----", "secret-key-material"},
	}
	for _, test := range inputs {
		output := redactor.Redact(test.input)
		if strings.Contains(output, test.secret) {
			t.Fatalf("secret remained in redacted output: %q", output)
		}
	}
}

func TestEvidenceRedactorPreservesNonSecretDiagnostics(t *testing.T) {
	redactor := NewEvidenceRedactor()
	input := "api container exited with status 1: connection refused to postgres:5432"
	if got := redactor.Redact(input); got != input {
		t.Fatalf("diagnostic content changed: %q", got)
	}
}
