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
