package clients

import (
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

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

func TestSSHReadToolCommandIsTypedAndRejectsMutation(t *testing.T) {
	command, err := sshReadToolCommand(domain.ToolRequest{Tool: "docker.logs", Arguments: map[string]string{"container": "api-prod"}})
	if err != nil || command != "docker logs --tail 200 api-prod" {
		t.Fatalf("unexpected command=%q err=%v", command, err)
	}
	if _, err := sshReadToolCommand(domain.ToolRequest{Tool: "docker.restart", Arguments: map[string]string{"container": "api-prod"}}); err == nil {
		t.Fatal("mutation should not be available through read-tool path")
	}
	if _, err := sshReadToolCommand(domain.ToolRequest{Tool: "system.journal", Arguments: map[string]string{"service": "api;whoami"}}); err == nil {
		t.Fatal("unsafe service name should be rejected")
	}
}
