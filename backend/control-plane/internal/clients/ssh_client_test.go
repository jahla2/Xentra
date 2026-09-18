package clients

import "testing"

func TestSafeMutationTargetRejectsShellSyntax(t *testing.T) {
	cases := map[string]bool{
		"api-prod": true,
		"worker_1": true,
		"nginx.service": true,
		"api;rm -rf /": false,
		"$(touch /tmp/pwn)": false,
		"": false,
	}
	for value, want := range cases {
		if got := safeMutationTarget(value); got != want {
			t.Fatalf("safeMutationTarget(%q)=%v want %v", value, got, want)
		}
	}
}
