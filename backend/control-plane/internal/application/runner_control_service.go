package application

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
	"github.com/jahla2/Xentra/backend/control-plane/internal/observability"
)

type RunnerTokenStore interface {
	StoreRunnerToken(context.Context, string) (string, error)
	GetRunnerToken(context.Context, string) (string, error)
}

type RunnerControlRepository interface {
	SaveRunner(context.Context, domain.RunnerRegistration) error
	FindRunnerByID(context.Context, string) (domain.RunnerRegistration, error)
	FindRunnerByEnvironment(context.Context, string, string) (domain.RunnerRegistration, error)
	TouchRunner(context.Context, string, time.Time) error
	EnqueueTask(context.Context, domain.RunnerTask) error
	ClaimNextTask(context.Context, string, time.Time) (domain.RunnerTask, bool, error)
	CompleteTask(context.Context, string, string, domain.RunnerTaskResult, time.Time) error
	GetTask(context.Context, string) (domain.RunnerTask, error)
}

type RunnerControlService struct {
	repo         RunnerControlRepository
	environments EnvironmentRepository
	projects     ProjectRepository
	tokens       RunnerTokenStore
}

func NewRunnerControlService(
	repo RunnerControlRepository,
	environments EnvironmentRepository,
	projects ProjectRepository,
	tokens RunnerTokenStore,
) *RunnerControlService {
	return &RunnerControlService{
		repo: repo, environments: environments, projects: projects, tokens: tokens,
	}
}

func (s *RunnerControlService) CreateEnrollment(
	ctx context.Context,
	organizationID, projectID, name, environmentType string,
) (domain.RunnerEnrollment, error) {
	if organizationID == "" || projectID == "" || name == "" {
		return domain.RunnerEnrollment{}, errors.New("organization, projectId and name are required")
	}
	if _, err := s.projects.Get(ctx, organizationID, projectID); err != nil {
		return domain.RunnerEnrollment{}, err
	}
	environmentID, err := newResourceID("env")
	if err != nil {
		return domain.RunnerEnrollment{}, err
	}
	runnerID, err := newResourceID("run")
	if err != nil {
		return domain.RunnerEnrollment{}, err
	}
	token, err := newRunnerToken()
	if err != nil {
		return domain.RunnerEnrollment{}, err
	}
	credentialID, err := s.tokens.StoreRunnerToken(ctx, token)
	if err != nil {
		return domain.RunnerEnrollment{}, err
	}
	if environmentType == "" {
		environmentType = "development"
	}
	env := domain.Environment{
		ID: environmentID, OrganizationID: organizationID, ProjectID: projectID,
		Name: name, Type: environmentType, ConnectionType: "runner_outbound",
		Capabilities: []string{},
	}
	if err := s.environments.Save(ctx, env); err != nil {
		return domain.RunnerEnrollment{}, err
	}
	registration := domain.RunnerRegistration{
		ID: runnerID, OrganizationID: organizationID, EnvironmentID: environmentID,
		CredentialID: credentialID, CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.SaveRunner(ctx, registration); err != nil {
		return domain.RunnerEnrollment{}, err
	}
	return domain.RunnerEnrollment{
		Environment: env, RunnerID: runnerID, RunnerToken: token,
		ControlPath: "/api/runners/" + runnerID,
	}, nil
}

func (s *RunnerControlService) Poll(
	ctx context.Context, runnerID, token string, discovery domain.Discovery,
) (domain.RunnerTask, bool, error) {
	registration, err := s.authenticate(ctx, runnerID, token)
	if err != nil {
		return domain.RunnerTask{}, false, err
	}
	env, err := s.environments.Get(ctx, registration.OrganizationID, registration.EnvironmentID)
	if err != nil {
		return domain.RunnerTask{}, false, err
	}
	if discovery.OS != "" || discovery.Hostname != "" || discovery.CPU != "" || discovery.Memory != "" || discovery.Disk != "" || discovery.Containers != nil || discovery.Capabilities != nil {
		if discovery.OS != "" {
			env.OS = discovery.OS
		}
		if discovery.Hostname != "" {
			env.Hostname = discovery.Hostname
		}
		if discovery.CPU != "" {
			env.CPU = discovery.CPU
		}
		if discovery.Memory != "" {
			env.Memory = discovery.Memory
		}
		if discovery.Disk != "" {
			env.Disk = discovery.Disk
		}
		if discovery.Containers != nil {
			env.Containers = discovery.Containers
		}
		if discovery.Capabilities != nil {
			env.Capabilities = discovery.Capabilities
		}
		if err := s.environments.Save(ctx, env); err != nil {
			return domain.RunnerTask{}, false, err
		}
	}
	now := time.Now().UTC()
	if err := s.repo.TouchRunner(ctx, runnerID, now); err != nil {
		return domain.RunnerTask{}, false, err
	}
	return s.repo.ClaimNextTask(ctx, runnerID, now.Add(-time.Minute))
}

func (s *RunnerControlService) Complete(
	ctx context.Context, runnerID, taskID, token string, result domain.RunnerTaskResult,
) error {
	if _, err := s.authenticate(ctx, runnerID, token); err != nil {
		return err
	}
	return s.repo.CompleteTask(ctx, runnerID, taskID, result, time.Now().UTC())
}

func (s *RunnerControlService) Dispatch(
	ctx context.Context, env domain.Environment, request domain.ToolRequest,
) (domain.RunnerTaskResult, error) {
	registration, err := s.repo.FindRunnerByEnvironment(ctx, env.OrganizationID, env.ID)
	if err != nil {
		return domain.RunnerTaskResult{}, err
	}
	taskID, err := newResourceID("task")
	if err != nil {
		return domain.RunnerTaskResult{}, err
	}
	traceParent, traceState := observability.InjectContext(ctx)
	task := domain.RunnerTask{
		ID: taskID, RunnerID: registration.ID, Tool: request.Tool,
		Arguments: request.Arguments, Status: "queued",
		TraceParent: traceParent, TraceState: traceState,
		CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.EnqueueTask(ctx, task); err != nil {
		return domain.RunnerTaskResult{}, err
	}

	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return domain.RunnerTaskResult{}, ctx.Err()
		case <-ticker.C:
			current, err := s.repo.GetTask(ctx, task.ID)
			if err != nil {
				return domain.RunnerTaskResult{}, err
			}
			if current.Status == "completed" {
				return current.Result, nil
			}
		}
	}
}

func (s *RunnerControlService) authenticate(
	ctx context.Context, runnerID, token string,
) (domain.RunnerRegistration, error) {
	if runnerID == "" || token == "" {
		return domain.RunnerRegistration{}, errors.New("runner authentication required")
	}
	registration, err := s.repo.FindRunnerByID(ctx, runnerID)
	if err != nil {
		return domain.RunnerRegistration{}, err
	}
	expected, err := s.tokens.GetRunnerToken(ctx, registration.CredentialID)
	if err != nil {
		return domain.RunnerRegistration{}, err
	}
	if subtle.ConstantTimeCompare([]byte(expected), []byte(token)) != 1 {
		return domain.RunnerRegistration{}, errors.New("invalid runner credentials")
	}
	return registration, nil
}

func newRunnerToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buffer), nil
}

type MemoryRunnerControlRepository struct {
	mu      sync.Mutex
	runners map[string]domain.RunnerRegistration
	tasks   map[string]domain.RunnerTask
}

func NewMemoryRunnerControlRepository() *MemoryRunnerControlRepository {
	return &MemoryRunnerControlRepository{
		runners: map[string]domain.RunnerRegistration{},
		tasks: map[string]domain.RunnerTask{},
	}
}

func (r *MemoryRunnerControlRepository) SaveRunner(_ context.Context, item domain.RunnerRegistration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, runner := range r.runners {
		if runner.EnvironmentID == item.EnvironmentID {
			return errors.New("environment already has an outbound runner")
		}
	}
	r.runners[item.ID] = item
	return nil
}

func (r *MemoryRunnerControlRepository) FindRunnerByID(_ context.Context, id string) (domain.RunnerRegistration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.runners[id]
	if !ok {
		return domain.RunnerRegistration{}, errors.New("outbound runner not found")
	}
	return item, nil
}

func (r *MemoryRunnerControlRepository) FindRunnerByEnvironment(_ context.Context, organizationID, environmentID string) (domain.RunnerRegistration, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, item := range r.runners {
		if item.OrganizationID == organizationID && item.EnvironmentID == environmentID {
			return item, nil
		}
	}
	return domain.RunnerRegistration{}, errors.New("outbound runner not found")
}

func (r *MemoryRunnerControlRepository) TouchRunner(_ context.Context, id string, seenAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.runners[id]
	if !ok {
		return errors.New("outbound runner not found")
	}
	item.LastSeenAt = &seenAt
	r.runners[id] = item
	return nil
}

func (r *MemoryRunnerControlRepository) EnqueueTask(_ context.Context, item domain.RunnerTask) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks[item.ID] = item
	return nil
}

func (r *MemoryRunnerControlRepository) ClaimNextTask(_ context.Context, runnerID string, staleBefore time.Time) (domain.RunnerTask, bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var selected *domain.RunnerTask
	for id, task := range r.tasks {
		if task.RunnerID != runnerID {
			continue
		}
		if task.Status == "claimed" && task.ClaimedAt != nil && task.ClaimedAt.Before(staleBefore) {
			task.Status = "queued"
			task.ClaimedAt = nil
			r.tasks[id] = task
		}
		if task.Status != "queued" {
			continue
		}
		copyTask := task
		if selected == nil || copyTask.CreatedAt.Before(selected.CreatedAt) {
			selected = &copyTask
		}
	}
	if selected == nil {
		return domain.RunnerTask{}, false, nil
	}
	now := time.Now().UTC()
	selected.Status = "claimed"
	selected.ClaimedAt = &now
	r.tasks[selected.ID] = *selected
	return *selected, true, nil
}

func (r *MemoryRunnerControlRepository) CompleteTask(_ context.Context, runnerID, taskID string, result domain.RunnerTaskResult, completedAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[taskID]
	if !ok || task.RunnerID != runnerID {
		return errors.New("runner task is not claimable for completion")
	}
	if task.Status == "completed" {
		return nil
	}
	if task.Status != "claimed" {
		return errors.New("runner task is not claimable for completion")
	}
	task.Status = "completed"
	task.Result = result
	task.CompletedAt = &completedAt
	r.tasks[taskID] = task
	return nil
}

func (r *MemoryRunnerControlRepository) GetTask(_ context.Context, id string) (domain.RunnerTask, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	task, ok := r.tasks[id]
	if !ok {
		return domain.RunnerTask{}, fmt.Errorf("runner task %q not found", id)
	}
	return task, nil
}
