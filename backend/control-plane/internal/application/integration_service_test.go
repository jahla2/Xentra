package application

import (
	"context"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeGitHubCredentialWriter struct {
	tokenCalls   int
	webhookCalls int
}

func (f *fakeGitHubCredentialWriter) StoreGitHubToken(context.Context, string) (string, error) {
	f.tokenCalls++
	return "cred-token", nil
}

func (f *fakeGitHubCredentialWriter) StoreGitHubWebhookSecret(context.Context, string) (string, error) {
	f.webhookCalls++
	return "cred-webhook", nil
}

type fakeGitHubInstallationResolver struct{ id int64 }

func (f fakeGitHubInstallationResolver) ResolveInstallation(context.Context, string, string) (int64, error) {
	return f.id, nil
}

func integrationEnvironmentRepo() *MemoryEnvironmentRepository {
	repo := NewMemoryEnvironmentRepository()
	_ = repo.Save(context.Background(), domain.Environment{
		ID: "env-1", OrganizationID: "org-a", ProjectID: "prj-1", Name: "Production",
	})
	return repo
}

func TestConnectGitHubUsesAppInstallationWithoutStoringPAT(t *testing.T) {
	credentials := &fakeGitHubCredentialWriter{}
	repo := NewMemoryIntegrationRepository()
	service := NewIntegrationService(
		repo,
		credentials,
		integrationEnvironmentRepo(),
		fakeGitHubInstallationResolver{id: 42},
	)

	setup, err := service.ConnectGitHub(context.Background(), "org-a", "env-1", "acme", "api", "github_app", "")
	if err != nil {
		t.Fatal(err)
	}
	if setup.Integration.AuthMode != "github_app" || setup.Integration.InstallationID != 42 {
		t.Fatalf("unexpected integration: %#v", setup.Integration)
	}
	if setup.Integration.CredentialID != "" || credentials.tokenCalls != 0 {
		t.Fatal("GitHub App connection should not store a PAT")
	}
	if setup.WebhookSecret == "" || credentials.webhookCalls != 1 {
		t.Fatal("expected one-time webhook secret")
	}
}

func TestConnectGitHubTokenFallbackStoresCredential(t *testing.T) {
	credentials := &fakeGitHubCredentialWriter{}
	service := NewIntegrationService(
		NewMemoryIntegrationRepository(),
		credentials,
		integrationEnvironmentRepo(),
		nil,
	)
	setup, err := service.ConnectGitHub(context.Background(), "org-a", "env-1", "acme", "api", "token", "pat")
	if err != nil {
		t.Fatal(err)
	}
	if setup.Integration.AuthMode != "token" || setup.Integration.CredentialID != "cred-token" {
		t.Fatalf("unexpected token integration: %#v", setup.Integration)
	}
	if credentials.tokenCalls != 1 {
		t.Fatalf("token stored %d times", credentials.tokenCalls)
	}
}
