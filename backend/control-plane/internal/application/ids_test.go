package application

import (
	"strings"
	"testing"
)

func TestNewResourceIDIsPrefixedAndUnique(t *testing.T) {
	first, err := newResourceID("env")
	if err != nil { t.Fatal(err) }
	second, err := newResourceID("env")
	if err != nil { t.Fatal(err) }
	if !strings.HasPrefix(first, "env-") || !strings.HasPrefix(second, "env-") {
		t.Fatalf("unexpected ids: %q %q", first, second)
	}
	if first == second { t.Fatal("resource ids must be unique") }
}
