package application

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"sync"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type RepositoryIntegrationRepository interface {
	Save(context.Context, domain.RepositoryIntegration) error
	FindByEnvironment(context.Context, string, string) (domain.RepositoryIntegration, error)
	FindByID(context.Context, string) (domain.RepositoryIntegration, error)
}

type GitHubCredentialWriter interface {
	StoreGitHubToken(context.Context, string) (string, error)
	StoreGitHubWebhookSecret(context.Context, string) (string, error)
}

type IntegrationService struct {
	repo         RepositoryIntegrationRepository
	credentials  GitHubCredentialWriter
	environments EnvironmentRepository
}

func NewIntegrationService(repo RepositoryIntegrationRepository, credentials GitHubCredentialWriter, environments EnvironmentRepository) *IntegrationService {
	return &IntegrationService{repo: repo, credentials: credentials, environments: environments}
}

func (s *IntegrationService) ConnectGitHub(ctx context.Context, organizationID, environmentID, owner, repoName, token string) (domain.GitHubIntegrationSetup, error) {
	if organizationID == "" || environmentID == "" || owner == "" || repoName == "" || token == "" {
		return domain.GitHubIntegrationSetup{}, errors.New("organization, environmentId, owner, repo and accessToken are required")
	}
	if _, err := s.environments.Get(ctx, organizationID, environmentID); err != nil {
		return domain.GitHubIntegrationSetup{}, err
	}

	credentialID, err := s.credentials.StoreGitHubToken(ctx, token)
	if err != nil {
		return domain.GitHubIntegrationSetup{}, err
	}
	webhookSecret, err := newWebhookSecret()
	if err != nil {
		return domain.GitHubIntegrationSetup{}, err
	}
	webhookCredentialID, err := s.credentials.StoreGitHubWebhookSecret(ctx, webhookSecret)
	if err != nil {
		return domain.GitHubIntegrationSetup{}, err
	}
	id, err := newResourceID("int")
	if err != nil {
		return domain.GitHubIntegrationSetup{}, err
	}
	integration := domain.RepositoryIntegration{
		ID: id, OrganizationID: organizationID, EnvironmentID: environmentID, Provider: "github",
		Owner: owner, Repo: repoName, CredentialID: credentialID,
		WebhookSecretCredentialID: webhookCredentialID,
	}
	if err := s.repo.Save(ctx, integration); err != nil {
		return domain.GitHubIntegrationSetup{}, err
	}
	return domain.GitHubIntegrationSetup{
		Integration: integration,
		WebhookPath: "/api/webhooks/github/" + integration.ID,
		WebhookSecret: webhookSecret,
	}, nil
}

func newWebhookSecret() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

type MemoryIntegrationRepository struct {
	mu            sync.RWMutex
	byEnvironment map[string]domain.RepositoryIntegration
	byID          map[string]domain.RepositoryIntegration
}

func NewMemoryIntegrationRepository() *MemoryIntegrationRepository {
	return &MemoryIntegrationRepository{
		byEnvironment: map[string]domain.RepositoryIntegration{},
		byID: map[string]domain.RepositoryIntegration{},
	}
}

func (r *MemoryIntegrationRepository) Save(_ context.Context, item domain.RepositoryIntegration) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byEnvironment[item.EnvironmentID] = item
	r.byID[item.ID] = item
	return nil
}

func (r *MemoryIntegrationRepository) FindByEnvironment(_ context.Context, organizationID, id string) (domain.RepositoryIntegration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.byEnvironment[id]
	if !ok || item.OrganizationID != organizationID {
		return domain.RepositoryIntegration{}, errors.New("repository integration not found")
	}
	return item, nil
}

func (r *MemoryIntegrationRepository) FindByID(_ context.Context, id string) (domain.RepositoryIntegration, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.byID[id]
	if !ok {
		return domain.RepositoryIntegration{}, errors.New("repository integration not found")
	}
	return item, nil
}
