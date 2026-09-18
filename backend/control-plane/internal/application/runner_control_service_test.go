package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type memoryRunnerTokenStore struct {
	mu     sync.Mutex
	values map[string]string
	next   int
}

func newMemoryRunnerTokenStore() *memoryRunnerTokenStore {
	return &memoryRunnerTokenStore{values: map[string]string{}}
}

func (s *memoryRunnerTokenStore) StoreRunnerToken(_ context.Context, token string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	id := "cred-runner-test"
	s.values[id] = token
	return id, nil
}

func (s *memoryRunnerTokenStore) GetRunnerToken(_ context.Context, id string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[id]
	if !ok {
		return "", errors.New("runner token not found")
	}
	return value, nil
}

func TestOutboundRunnerEnrollmentAndTaskLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	projects := NewMemoryProjectRepository()
	if err := projects.Save(ctx, domain.Project{ID: "prj-1", OrganizationID: "org-a", Name: "Core"}); err != nil {
		t.Fatal(err)
	}
	environments := NewMemoryEnvironmentRepository()
	repo := NewMemoryRunnerControlRepository()
	tokens := newMemoryRunnerTokenStore()
	service := NewRunnerControlService(repo, environments, projects, tokens)

	enrollment, err := service.CreateEnrollment(ctx, "org-a", "prj-1", "Production", "production")
	if err != nil {
		t.Fatal(err)
	}
	if enrollment.Environment.ConnectionType != "runner_outbound" || enrollment.RunnerID == "" || enrollment.RunnerToken == "" {
		t.Fatalf("unexpected enrollment: %#v", enrollment)
	}

	if _, _, err := service.Poll(ctx, enrollment.RunnerID, "wrong-token", domain.Discovery{}); err == nil {
		t.Fatal("expected invalid Runner token to be rejected")
	}

	resultCh := make(chan domain.RunnerTaskResult, 1)
	errCh := make(chan error, 1)
	go func() {
		result, dispatchErr := service.Dispatch(ctx, enrollment.Environment, domain.ToolRequest{
			Tool: "system.info", Arguments: map[string]string{},
		})
		if dispatchErr != nil {
			errCh <- dispatchErr
			return
		}
		resultCh <- result
	}()

	var task domain.RunnerTask
	for {
		var found bool
		var pollErr error
		task, found, pollErr = service.Poll(ctx, enrollment.RunnerID, enrollment.RunnerToken, domain.Discovery{
			OS: "linux", Hostname: "prod-01", Capabilities: []string{"docker", "systemd"},
		})
		if pollErr != nil {
			t.Fatal(pollErr)
		}
		if found {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("timed out waiting for queued Runner task")
		case <-time.After(20 * time.Millisecond):
		}
	}
	if task.Tool != "system.info" {
		t.Fatalf("unexpected task: %#v", task)
	}
	resultPayload := domain.RunnerTaskResult{Success: true, Output: "Linux prod-01"}
	if err := service.Complete(ctx, enrollment.RunnerID, task.ID, enrollment.RunnerToken, resultPayload); err != nil {
		t.Fatal(err)
	}
	if err := service.Complete(ctx, enrollment.RunnerID, task.ID, enrollment.RunnerToken, resultPayload); err != nil {
		t.Fatalf("duplicate result acknowledgement should be idempotent: %v", err)
	}

	select {
	case err := <-errCh:
		t.Fatal(err)
	case result := <-resultCh:
		if !result.Success || result.Output != "Linux prod-01" {
			t.Fatalf("unexpected result: %#v", result)
		}
	case <-ctx.Done():
		t.Fatal("timed out waiting for dispatch result")
	}

	updated, err := environments.Get(ctx, "org-a", enrollment.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.Hostname != "prod-01" || updated.OS != "linux" || len(updated.Capabilities) != 2 {
		t.Fatalf("Runner discovery was not persisted: %#v", updated)
	}
}
