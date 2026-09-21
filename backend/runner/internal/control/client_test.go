package control

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jahla2/Xentra/backend/runner/internal/domain"
)

func TestProductionControlClientRejectsPlainHTTP(t *testing.T) {
	_, err := NewClient(Config{
		BaseURL: "http://control.example", RunnerID: "run-1", RunnerToken: "token",
	})
	if err == nil {
		t.Fatal("expected production client to reject plain HTTP")
	}
}

func TestDevelopmentControlClientPollsAndCompletesWithRunnerToken(t *testing.T) {
	var completed bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Runner runner-secret" {
			t.Fatalf("unexpected authorization header %q", r.Header.Get("Authorization"))
		}
		switch r.URL.Path {
		case "/api/runners/run-1/poll":
			var discovery domain.Discovery
			if err := json.NewDecoder(r.Body).Decode(&discovery); err != nil {
				t.Fatal(err)
			}
			if discovery.Hostname != "host-1" {
				t.Fatalf("unexpected discovery: %#v", discovery)
			}
			_ = json.NewEncoder(w).Encode(Task{ID: "task-1", RunnerID: "run-1", Tool: "system.info"})
		case "/api/runners/run-1/tasks/task-1/result":
			completed = true
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewClient(Config{
		BaseURL: server.URL, RunnerID: "run-1", RunnerToken: "runner-secret", InsecureDev: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	task, err := client.Poll(context.Background(), domain.Discovery{Hostname: "host-1"})
	if err != nil {
		t.Fatal(err)
	}
	if task == nil || task.ID != "task-1" {
		t.Fatalf("unexpected task: %#v", task)
	}
	if err := client.Complete(context.Background(), task.ID, domain.ToolResult{Tool: task.Tool, Success: true, Output: "ok"}); err != nil {
		t.Fatal(err)
	}
	if !completed {
		t.Fatal("task result was not submitted")
	}
}
