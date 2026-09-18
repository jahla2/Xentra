package application

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type EnvironmentRepository interface {
	Save(context.Context, domain.Environment) error
	Get(context.Context, string, string) (domain.Environment, error)
	List(context.Context, string) ([]domain.Environment, error)
}

type DiscoveryClient interface {
	Discover(context.Context, domain.Environment) (domain.Discovery, error)
}

type SSHCredentialWriter interface {
	StoreSSH(context.Context, domain.SSHCredential) (string, error)
}

type CreateEnvironmentInput struct {
	ProjectID             string `json:"projectId"`
	Name                  string `json:"name"`
	Type                  string `json:"type"`
	ConnectionType        string `json:"connectionType"`
	RunnerURL             string `json:"runnerUrl"`
	SSHHost               string `json:"sshHost"`
	SSHPort               int    `json:"sshPort"`
	SSHUser               string `json:"sshUser"`
	SSHHostKeyFingerprint string `json:"sshHostKeyFingerprint"`
	SSHPrivateKey         string `json:"sshPrivateKey"`
	SSHPassphrase         string `json:"sshPassphrase"`
}

type EnvironmentService struct {
	repo        EnvironmentRepository
	projects    ProjectRepository
	discovery   DiscoveryClient
	credentials SSHCredentialWriter
}

func NewEnvironmentService(repo EnvironmentRepository, projects ProjectRepository, discovery DiscoveryClient, credentials SSHCredentialWriter) *EnvironmentService {
	return &EnvironmentService{repo: repo, projects: projects, discovery: discovery, credentials: credentials}
}

func (s *EnvironmentService) Create(ctx context.Context, organizationID string, input CreateEnvironmentInput) (domain.Environment, error) {
	if organizationID == "" {
		return domain.Environment{}, errors.New("organization is required")
	}
	if input.ProjectID == "" {
		return domain.Environment{}, errors.New("projectId is required")
	}
	if input.Name == "" {
		return domain.Environment{}, errors.New("name is required")
	}
	if s.projects == nil {
		return domain.Environment{}, errors.New("project repository is unavailable")
	}
	if _, err := s.projects.Get(ctx, organizationID, input.ProjectID); err != nil {
		return domain.Environment{}, err
	}

	id, err := newResourceID("env")
	if err != nil {
		return domain.Environment{}, fmt.Errorf("create environment id: %w", err)
	}
	connectionType := input.ConnectionType
	if connectionType == "" {
		connectionType = "runner"
	}
	env := domain.Environment{
		ID: id, OrganizationID: organizationID, ProjectID: input.ProjectID, Name: input.Name, Type: input.Type,
		ConnectionType: connectionType, RunnerURL: input.RunnerURL, SSHHost: input.SSHHost,
		SSHPort: input.SSHPort, SSHUser: input.SSHUser, SSHHostKeyFingerprint: input.SSHHostKeyFingerprint,
	}
	if env.Type == "" {
		env.Type = "development"
	}

	switch connectionType {
	case "runner":
		if input.RunnerURL == "" {
			return domain.Environment{}, errors.New("runnerUrl is required for runner connections")
		}
	case "runner_outbound":
		return domain.Environment{}, errors.New("use /api/runner-enrollments for outbound Runner environments")
	case "ssh":
		if input.SSHHost == "" || input.SSHUser == "" || input.SSHPrivateKey == "" {
			return domain.Environment{}, errors.New("sshHost, sshUser and sshPrivateKey are required for ssh connections")
		}
		if input.SSHHostKeyFingerprint == "" {
			return domain.Environment{}, errors.New("sshHostKeyFingerprint is required to prevent host impersonation")
		}
		if env.SSHPort == 0 {
			env.SSHPort = 22
		}
		if s.credentials == nil {
			return domain.Environment{}, errors.New("credential store is unavailable")
		}
		credentialID, storeErr := s.credentials.StoreSSH(ctx, domain.SSHCredential{PrivateKey: input.SSHPrivateKey, Passphrase: input.SSHPassphrase})
		if storeErr != nil {
			return domain.Environment{}, fmt.Errorf("store ssh credential: %w", storeErr)
		}
		env.CredentialID = credentialID
	default:
		return domain.Environment{}, errors.New("connectionType must be runner, runner_outbound, or ssh")
	}

	discovered, err := s.discovery.Discover(ctx, env)
	if err != nil {
		return domain.Environment{}, fmt.Errorf("discover environment: %w", err)
	}
	env.OS = discovered.OS
	env.Hostname = discovered.Hostname
	env.Capabilities = discovered.Capabilities

	if err := s.repo.Save(ctx, env); err != nil {
		return domain.Environment{}, err
	}
	return env, nil
}

func (s *EnvironmentService) List(ctx context.Context, organizationID string) ([]domain.Environment, error) {
	return s.repo.List(ctx, organizationID)
}

type MemoryEnvironmentRepository struct {
	mu   sync.RWMutex
	data map[string]domain.Environment
}

func NewMemoryEnvironmentRepository() *MemoryEnvironmentRepository {
	return &MemoryEnvironmentRepository{data: map[string]domain.Environment{}}
}

func (r *MemoryEnvironmentRepository) Save(_ context.Context, env domain.Environment) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[env.ID] = env
	return nil
}

func (r *MemoryEnvironmentRepository) Get(_ context.Context, organizationID, id string) (domain.Environment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	env, ok := r.data[id]
	if !ok || env.OrganizationID != organizationID {
		return domain.Environment{}, errors.New("environment not found")
	}
	return env, nil
}

func (r *MemoryEnvironmentRepository) List(_ context.Context, organizationID string) ([]domain.Environment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := []domain.Environment{}
	for _, env := range r.data {
		if env.OrganizationID == organizationID {
			result = append(result, env)
		}
	}
	return result, nil
}
