package application

import (
	"context"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeDiscoveryClient struct{ result domain.Discovery }

func (f fakeDiscoveryClient) Discover(_ context.Context, _ domain.Environment) (domain.Discovery, error) {
	return f.result, nil
}

type fakeSSHCredentialWriter struct{ id string }

func (f fakeSSHCredentialWriter) StoreSSH(context.Context, domain.SSHCredential) (string, error) {
	return f.id, nil
}

func TestCreateRunnerEnvironmentDiscoversCapabilities(t *testing.T) {
	repo := NewMemoryEnvironmentRepository()
	discovery := fakeDiscoveryClient{domain.Discovery{OS: "linux", Hostname: "prod-01", Capabilities: []string{"docker", "systemd"}}}
	service := NewEnvironmentService(repo, discovery, nil)

	env, err := service.Create(context.Background(), "org-a", CreateEnvironmentInput{Name: "Production", Type: "production", ConnectionType: "runner", RunnerURL: "http://runner:8090"})
	if err != nil {
		t.Fatal(err)
	}
	if env.OrganizationID != "org-a" || env.ConnectionType != "runner" || env.Hostname != "prod-01" || len(env.Capabilities) != 2 {
		t.Fatalf("unexpected environment: %#v", env)
	}
}

func TestCreateSSHEnvironmentStoresCredentialReferenceOnly(t *testing.T) {
	repo := NewMemoryEnvironmentRepository()
	discovery := fakeDiscoveryClient{domain.Discovery{OS: "linux", Hostname: "ssh-prod", Capabilities: []string{"docker"}}}
	service := NewEnvironmentService(repo, discovery, fakeSSHCredentialWriter{id: "cred-safe"})

	env, err := service.Create(context.Background(), "org-a", CreateEnvironmentInput{Name: "SSH Production", ConnectionType: "ssh", SSHHost: "10.0.0.10", SSHUser: "ubuntu", SSHPrivateKey: "PRIVATE", SSHHostKeyFingerprint: "SHA256:test"})
	if err != nil {
		t.Fatal(err)
	}
	if env.CredentialID != "cred-safe" || env.SSHPort != 22 {
		t.Fatalf("unexpected environment: %#v", env)
	}
}

func TestEnvironmentRepositoryIsolatesOrganizations(t *testing.T) {
	repo := NewMemoryEnvironmentRepository()
	_ = repo.Save(context.Background(), domain.Environment{ID: "env-a", OrganizationID: "org-a"})
	_ = repo.Save(context.Background(), domain.Environment{ID: "env-b", OrganizationID: "org-b"})

	items, _ := repo.List(context.Background(), "org-a")
	if len(items) != 1 || items[0].ID != "env-a" {
		t.Fatalf("unexpected org-a environments: %#v", items)
	}
	if _, err := repo.Get(context.Background(), "org-a", "env-b"); err == nil {
		t.Fatal("expected cross-organization lookup to fail")
	}
}
