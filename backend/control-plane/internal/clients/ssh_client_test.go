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
	if err != nil || command != "docker logs --tail 200 'api-prod'" {
		t.Fatalf("unexpected command=%q err=%v", command, err)
	}
	if _, err := sshReadToolCommand(domain.ToolRequest{Tool: "docker.restart", Arguments: map[string]string{"container": "api-prod"}}); err == nil {
		t.Fatal("mutation should not be available through read-tool path")
	}
	if _, err := sshReadToolCommand(domain.ToolRequest{Tool: "system.journal", Arguments: map[string]string{"service": "api;whoami"}}); err == nil {
		t.Fatal("unsafe service name should be rejected")
	}
}

func TestSSHReadToolCommandSupportsMVPDiagnostics(t *testing.T) {
	cases := []domain.ToolRequest{
		{Tool: "git.status", Arguments: map[string]string{"path": "/srv/app"}},
		{Tool: "git.log", Arguments: map[string]string{"path": "/srv/app"}},
		{Tool: "git.diff", Arguments: map[string]string{"path": "/srv/app"}},
		{Tool: "git.show_commit", Arguments: map[string]string{"path": "/srv/app", "commit": "abcdef1234567"}},
		{Tool: "http.health_check", Arguments: map[string]string{"url": "https://internal-api.local/health"}},
		{Tool: "dns.lookup", Arguments: map[string]string{"host": "internal-api.local"}},
	}
	for _, request := range cases {
		if command, err := sshReadToolCommand(request); err != nil || command == "" {
			t.Fatalf("%s command=%q err=%v", request.Tool, command, err)
		}
	}
}

func TestSSHReadToolCommandRejectsUnsafeMVPDiagnostics(t *testing.T) {
	cases := []domain.ToolRequest{
		{Tool: "git.status", Arguments: map[string]string{"path": "/srv/app\nwhoami"}},
		{Tool: "git.show_commit", Arguments: map[string]string{"commit": "HEAD;whoami"}},
		{Tool: "http.health_check", Arguments: map[string]string{"url": "file:///etc/passwd"}},
		{Tool: "http.health_check", Arguments: map[string]string{"url": "https://user:secret@example.com/health"}},
		{Tool: "dns.lookup", Arguments: map[string]string{"host": "internal;whoami"}},
	}
	for _, request := range cases {
		if command, err := sshReadToolCommand(request); err == nil {
			t.Fatalf("%s accepted unsafe request as %q", request.Tool, command)
		}
	}
}

func TestShellQuoteEscapesSingleQuote(t *testing.T) {
	got := shellQuote("/srv/team's-app")
	want := "'/srv/team'\"'\"'s-app'"
	if got != want {
		t.Fatalf("shellQuote=%q want %q", got, want)
	}
}
