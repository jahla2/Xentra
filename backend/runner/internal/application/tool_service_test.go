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

func TestExecuteAllowsTypedDockerRestart(t *testing.T) {
	result := NewToolService(fakeExecutor{outputs: map[string]string{"docker":"api-prod"}}).Execute(context.Background(), "docker.restart", map[string]string{"container":"api-prod"})
	if !result.Success { t.Fatalf("unexpected result: %#v", result) }
}

func TestExecuteRejectsUnsafeMutationTarget(t *testing.T) {
	service := NewToolService(fakeExecutor{outputs: map[string]string{}})
	for _, tool := range []string{"docker.restart", "system.service_restart"} {
		key := "container"
		if tool == "system.service_restart" { key = "service" }
		result := service.Execute(context.Background(), tool, map[string]string{key:"api;rm -rf /"})
		if result.Success { t.Fatalf("%s accepted unsafe target", tool) }
	}
}
