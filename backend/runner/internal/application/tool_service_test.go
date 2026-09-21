package application

import (
	"context"
	"testing"
)

func TestExecuteRejectsUnknownTool(t *testing.T) {
	result := NewToolService(fakeExecutor{outputs: map[string]string{}}).Execute(context.Background(), "shell.anything", map[string]string{})
	if result.Success { t.Fatal("expected unknown tool to be rejected") }
}

func TestExecuteDockerListUsesAllowlistedCommand(t *testing.T) {
	result := NewToolService(fakeExecutor{outputs: map[string]string{"docker":"api\tUp 2 minutes"}}).Execute(context.Background(), "docker.list", map[string]string{})
	if !result.Success || result.Output == "" { t.Fatalf("unexpected result: %#v", result) }
}

func TestExecuteAllowsTypedDockerDiagnostics(t *testing.T) {
	service := NewToolService(fakeExecutor{outputs: map[string]string{"docker":"diagnostic"}})
	for _, tool := range []string{"docker.inspect", "docker.stats", "docker.logs"} {
		result := service.Execute(context.Background(), tool, map[string]string{"container":"api-prod"})
		if !result.Success { t.Fatalf("%s failed: %#v", tool, result) }
	}
}

func TestExecuteAllowsTypedSystemDiagnostics(t *testing.T) {
	service := NewToolService(fakeExecutor{outputs: map[string]string{"lscpu":"cpu","free":"memory","journalctl":"logs","systemctl":"active"}})
	cases := []struct{tool string; args map[string]string}{
		{"system.cpu", map[string]string{}},
		{"system.memory", map[string]string{}},
		{"system.journal", map[string]string{"service":"api.service"}},
		{"system.service_status", map[string]string{"service":"api.service"}},
	}
	for _, item := range cases {
		result := service.Execute(context.Background(), item.tool, item.args)
		if !result.Success { t.Fatalf("%s failed: %#v", item.tool, result) }
	}
}

func TestExecuteAllowsTypedGitDiagnostics(t *testing.T) {
	service := NewToolService(fakeExecutor{outputs: map[string]string{"git":"git output"}})
	cases := []struct{tool string; args map[string]string}{
		{"git.status", map[string]string{"path":"/srv/app"}},
		{"git.log", map[string]string{"path":"/srv/app"}},
		{"git.diff", map[string]string{"path":"/srv/app"}},
		{"git.show_commit", map[string]string{"path":"/srv/app","commit":"abcdef1234567"}},
	}
	for _, item := range cases {
		result := service.Execute(context.Background(), item.tool, item.args)
		if !result.Success { t.Fatalf("%s failed: %#v", item.tool, result) }
	}
}

func TestExecuteAllowsTypedNetworkDiagnostics(t *testing.T) {
	service := NewToolService(fakeExecutor{outputs: map[string]string{"curl":"200","getent":"10.0.0.10 internal-api.local"}})
	health := service.Execute(context.Background(), "http.health_check", map[string]string{"url":"https://internal-api.local/health"})
	if !health.Success || health.Output != "200" { t.Fatalf("unexpected health result: %#v", health) }
	dns := service.Execute(context.Background(), "dns.lookup", map[string]string{"host":"internal-api.local"})
	if !dns.Success { t.Fatalf("unexpected dns result: %#v", dns) }
}

func TestExecuteRejectsUnsafeReadToolArguments(t *testing.T) {
	service := NewToolService(fakeExecutor{outputs: map[string]string{}})
	cases := []struct{tool string; args map[string]string}{
		{"git.show_commit", map[string]string{"commit":"HEAD;rm-rf"}},
		{"git.status", map[string]string{"path":"/srv/app\nrm -rf /"}},
		{"http.health_check", map[string]string{"url":"file:///etc/passwd"}},
		{"http.health_check", map[string]string{"url":"https://user:secret@example.com/health"}},
		{"dns.lookup", map[string]string{"host":"example.com;id"}},
	}
	for _, item := range cases {
		result := service.Execute(context.Background(), item.tool, item.args)
		if result.Success { t.Fatalf("%s accepted unsafe arguments %#v", item.tool, item.args) }
	}
}

func TestExecuteAllowsTypedDockerRestart(t *testing.T) {
	result := NewToolService(fakeExecutor{outputs: map[string]string{"docker":"api-prod"}}).Execute(context.Background(), "docker.restart", map[string]string{"container":"api-prod"})
	if !result.Success { t.Fatalf("unexpected result: %#v", result) }
}

func TestExecuteRejectsUnsafeMutationTarget(t *testing.T) {
	service := NewToolService(fakeExecutor{outputs: map[string]string{}})
	for _, tool := range []string{"docker.restart", "system.service_restart", "docker.inspect", "system.journal"} {
		key := "container"
		if tool == "system.service_restart" || tool == "system.journal" { key = "service" }
		result := service.Execute(context.Background(), tool, map[string]string{key:"api;rm -rf /"})
		if result.Success { t.Fatalf("%s accepted unsafe target", tool) }
	}
}
