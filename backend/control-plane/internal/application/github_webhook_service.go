package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

var ErrInvalidWebhookSignature = errors.New("invalid GitHub webhook signature")

type GitHubWebhookSecretReader interface {
	GetGitHubWebhookSecret(context.Context, string) (string, error)
}

type WebhookDeliveryRepository interface {
	Claim(context.Context, string, string, string) (bool, error)
}

type AutomatedIncidentCreator interface {
	Create(context.Context, string, string, string) (domain.Incident, error)
}

type GitHubWebhookService struct {
	integrations RepositoryIntegrationRepository
	credentials  GitHubWebhookSecretReader
	deliveries   WebhookDeliveryRepository
	incidents    AutomatedIncidentCreator
}

type GitHubWebhookResult struct {
	Ignored   bool             `json:"ignored"`
	Duplicate bool             `json:"duplicate"`
	Incident  *domain.Incident `json:"incident,omitempty"`
}

func NewGitHubWebhookService(
	integrations RepositoryIntegrationRepository,
	credentials GitHubWebhookSecretReader,
	deliveries WebhookDeliveryRepository,
	incidents AutomatedIncidentCreator,
) *GitHubWebhookService {
	return &GitHubWebhookService{
		integrations: integrations,
		credentials: credentials,
		deliveries: deliveries,
		incidents: incidents,
	}
}

func (s *GitHubWebhookService) Process(
	ctx context.Context,
	integrationID, eventType, deliveryID, signature string,
	body []byte,
) (GitHubWebhookResult, error) {
	if integrationID == "" || eventType == "" || deliveryID == "" {
		return GitHubWebhookResult{}, errors.New("integration ID, event type and delivery ID are required")
	}
	integration, err := s.integrations.FindByID(ctx, integrationID)
	if err != nil {
		return GitHubWebhookResult{}, err
	}
	if integration.WebhookSecretCredentialID == "" {
		return GitHubWebhookResult{}, errors.New("webhook secret is not configured")
	}
	secret, err := s.credentials.GetGitHubWebhookSecret(ctx, integration.WebhookSecretCredentialID)
	if err != nil {
		return GitHubWebhookResult{}, err
	}
	if !verifyGitHubSignature(secret, signature, body) {
		return GitHubWebhookResult{}, ErrInvalidWebhookSignature
	}

	claimed, err := s.deliveries.Claim(ctx, integration.ID, deliveryID, eventType)
	if err != nil {
		return GitHubWebhookResult{}, err
	}
	if !claimed {
		return GitHubWebhookResult{Duplicate: true}, nil
	}
	if eventType != "workflow_run" {
		return GitHubWebhookResult{Ignored: true}, nil
	}

	var event struct {
		Action string `json:"action"`
		WorkflowRun struct {
			Name       string `json:"name"`
			Conclusion string `json:"conclusion"`
			HTMLURL    string `json:"html_url"`
		} `json:"workflow_run"`
		Repository struct {
			Name  string `json:"name"`
			Owner struct {
				Login string `json:"login"`
			} `json:"owner"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(body, &event); err != nil {
		return GitHubWebhookResult{}, fmt.Errorf("decode GitHub webhook: %w", err)
	}
	if !strings.EqualFold(event.Repository.Name, integration.Repo) ||
		!strings.EqualFold(event.Repository.Owner.Login, integration.Owner) {
		return GitHubWebhookResult{}, errors.New("webhook repository does not match integration")
	}
	if event.Action != "completed" || event.WorkflowRun.Conclusion != "failure" {
		return GitHubWebhookResult{Ignored: true}, nil
	}

	question := fmt.Sprintf(
		"GitHub Actions workflow %q failed for %s/%s. Investigate the connected environment and determine the probable root cause.",
		event.WorkflowRun.Name, integration.Owner, integration.Repo,
	)
	incident, err := s.incidents.Create(ctx, integration.OrganizationID, integration.EnvironmentID, question)
	if err != nil {
		return GitHubWebhookResult{}, err
	}
	return GitHubWebhookResult{Incident: &incident}, nil
}

func verifyGitHubSignature(secret, signature string, body []byte) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(signature, prefix) {
		return false
	}
	provided, err := hex.DecodeString(strings.TrimPrefix(signature, prefix))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return hmac.Equal(mac.Sum(nil), provided)
}

type MemoryWebhookDeliveryRepository struct {
	mu   sync.Mutex
	seen map[string]bool
}

func NewMemoryWebhookDeliveryRepository() *MemoryWebhookDeliveryRepository {
	return &MemoryWebhookDeliveryRepository{seen: map[string]bool{}}
}

func (r *MemoryWebhookDeliveryRepository) Claim(_ context.Context, integrationID, deliveryID, eventType string) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.seen[deliveryID] {
		return false, nil
	}
	r.seen[deliveryID] = true
	return true, nil
}
