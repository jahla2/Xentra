package application

import (
	"context"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeToolClient struct{ results []domain.Evidence }

func (f fakeToolClient) Collect(context.Context, domain.Environment) ([]domain.Evidence, error) {
	return f.results, nil
}

type fakeAIClient struct{ response domain.InvestigationResult }

func (f fakeAIClient) Investigate(context.Context, domain.InvestigationRequest) (domain.InvestigationResult, error) {
	return f.response, nil
}

func TestInvestigateCollectsEvidenceAndCallsAI(t *testing.T) {
	repo := NewMemoryEnvironmentRepository()
	_ = repo.Save(context.Background(), domain.Environment{ID: "env-1", OrganizationID: "org-a", Name: "Production", Type: "production", RunnerURL: "http://runner:8090"})
	service := NewInvestigationService(repo, fakeToolClient{results: []domain.Evidence{{Source: "docker.list", Output: "api unhealthy"}}}, fakeAIClient{response: domain.InvestigationResult{Summary: "API container unhealthy", Confidence: "high"}})

	result, err := service.Investigate(context.Background(), "org-a", "env-1", "Why is the API down?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "API container unhealthy" {
		t.Fatalf("unexpected result: %#v", result)
	}

	if _, err := service.Investigate(context.Background(), "org-b", "env-1", "Why?"); err == nil {
		t.Fatal("expected cross-organization investigation to fail")
	}
}
