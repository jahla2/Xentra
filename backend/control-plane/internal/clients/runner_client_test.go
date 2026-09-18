package clients

import (
	"net/http"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

func TestSecureRunnerClientRejectsPlainHTTPURL(t *testing.T) {
	client := NewRunnerClientWithHTTPClient(&http.Client{}, true)
	if _, err := client.endpoint("http://runner.example:8090", "/v1/discovery"); err == nil {
		t.Fatal("expected secure runner client to reject plain HTTP URL")
	}
}

func TestSecureRunnerClientAcceptsHTTPSURL(t *testing.T) {
	client := NewRunnerClientWithHTTPClient(&http.Client{}, true)
	got, err := client.endpoint("https://runner.example:8090/", "/v1/discovery")
	if err != nil {
		t.Fatal(err)
	}
	if got != "https://runner.example:8090/v1/discovery" {
		t.Fatalf("unexpected endpoint %q", got)
	}
}

func TestActionArgumentsRemainTyped(t *testing.T) {
	args, err := actionArguments("docker.restart", "api-prod")
	if err != nil || args["container"] != "api-prod" {
		t.Fatalf("unexpected typed action args: %#v err=%v", args, err)
	}
	if _, err := actionArguments("shell.exec", "anything"); err == nil {
		t.Fatal("expected arbitrary action to remain blocked")
	}
}

var _ = domain.Environment{}


func TestMVPReadToolsRemainAllowlisted(t *testing.T) {
	for _, tool := range []string{"git.status", "git.log", "git.diff", "git.show_commit", "http.health_check", "dns.lookup"} {
		if !runnerReadTools[tool] {
			t.Fatalf("expected %s to be allowlisted", tool)
		}
	}
}
