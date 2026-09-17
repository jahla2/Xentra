package application

import (
	"context"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeDiscoveryClient struct{ result domain.Discovery }

func (f fakeDiscoveryClient) Discover(context.Context, string) (domain.Discovery, error) { return f.result, nil }

func TestCreateEnvironmentDiscoversCapabilities(t *testing.T) {
	repo := NewMemoryEnvironmentRepository()
	discovery := fakeDiscoveryClient{result: domain.Discovery{OS: "linux", Hostname: "prod-01", Capabilities: []string{"docker", "systemd"}}}
	service := NewEnvironmentService(repo, discovery)
	env, err := service.Create(context.Background(), CreateEnvironmentInput{Name: "Production", Type: "production", RunnerURL: "http://runner:8090"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if env.Name != "Production" || env.Hostname != "prod-01" || len(env.Capabilities) != 2 {
		t.Fatalf("unexpected environment: %#v", env)
	}
}
