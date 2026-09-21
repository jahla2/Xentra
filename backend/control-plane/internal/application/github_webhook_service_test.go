package application

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeWebhookSecretReader struct{ secret string }

func (f fakeWebhookSecretReader) GetGitHubWebhookSecret(context.Context, string) (string, error) {
	return f.secret, nil
}

type fakeAutomatedIncidentCreator struct {
	count int
	lastOrganizationID string
	lastEnvironmentID string
	lastQuestion string
}

func (f *fakeAutomatedIncidentCreator) Create(_ context.Context, organizationID, environmentID, question string) (domain.Incident, error) {
	f.count++
	f.lastOrganizationID = organizationID
	f.lastEnvironmentID = environmentID
	f.lastQuestion = question
	return domain.Incident{ID: "inc-auto", OrganizationID: organizationID, EnvironmentID: environmentID}, nil
}

func githubTestSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write(body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

func githubWebhookTestService() (*GitHubWebhookService, *fakeAutomatedIncidentCreator) {
	integrations := NewMemoryIntegrationRepository()
	_ = integrations.Save(context.Background(), domain.RepositoryIntegration{
		ID: "int-1", OrganizationID: "org-a", EnvironmentID: "env-1",
		Provider: "github", Owner: "acme", Repo: "api",
		WebhookSecretCredentialID: "cred-webhook",
	})
	creator := &fakeAutomatedIncidentCreator{}
	service := NewGitHubWebhookService(
		integrations,
		fakeWebhookSecretReader{secret: "test-secret"},
		NewMemoryWebhookDeliveryRepository(),
		creator,
	)
	return service, creator
}

func TestFailedWorkflowWebhookCreatesIncidentOnce(t *testing.T) {
	service, creator := githubWebhookTestService()
	body := []byte(`{"action":"completed","workflow_run":{"name":"deploy","conclusion":"failure","html_url":"https://github/run/1"},"repository":{"name":"api","owner":{"login":"acme"}}}`)
	signature := githubTestSignature("test-secret", body)

	result, err := service.Process(context.Background(), "int-1", "workflow_run", "delivery-1", signature, body)
	if err != nil {
		t.Fatal(err)
	}
	if result.Incident == nil || result.Incident.ID != "inc-auto" || creator.count != 1 {
		t.Fatalf("unexpected webhook result: %#v count=%d", result, creator.count)
	}
	if creator.lastOrganizationID != "org-a" || creator.lastEnvironmentID != "env-1" {
		t.Fatalf("unexpected incident target: %#v", creator)
	}

	duplicate, err := service.Process(context.Background(), "int-1", "workflow_run", "delivery-1", signature, body)
	if err != nil {
		t.Fatal(err)
	}
	if !duplicate.Duplicate || creator.count != 1 {
		t.Fatalf("duplicate webhook created another incident: %#v count=%d", duplicate, creator.count)
	}
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	service, creator := githubWebhookTestService()
	body := []byte(`{"action":"completed","workflow_run":{"name":"deploy","conclusion":"failure"},"repository":{"name":"api","owner":{"login":"acme"}}}`)

	_, err := service.Process(context.Background(), "int-1", "workflow_run", "delivery-2", "sha256=00", body)
	if !errors.Is(err, ErrInvalidWebhookSignature) {
		t.Fatalf("expected invalid signature, got %v", err)
	}
	if creator.count != 0 {
		t.Fatal("invalid signature created an incident")
	}
}

func TestSuccessfulWorkflowIsIgnored(t *testing.T) {
	service, creator := githubWebhookTestService()
	body := []byte(`{"action":"completed","workflow_run":{"name":"deploy","conclusion":"success"},"repository":{"name":"api","owner":{"login":"acme"}}}`)
	result, err := service.Process(context.Background(), "int-1", "workflow_run", "delivery-3", githubTestSignature("test-secret", body), body)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Ignored || creator.count != 0 {
		t.Fatalf("successful workflow should be ignored: %#v", result)
	}
}
