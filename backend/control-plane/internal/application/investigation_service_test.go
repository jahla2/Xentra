package application

import (
	"context"
	"strings"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeToolClient struct{ results []domain.Evidence }

func (f fakeToolClient) Collect(context.Context, domain.Environment) ([]domain.Evidence, error) {
	return f.results, nil
}

type fakeAIClient struct {
	response domain.InvestigationResult
	request  *domain.InvestigationRequest
}

func (f *fakeAIClient) Investigate(_ context.Context, request domain.InvestigationRequest) (domain.InvestigationResult, error) {
	if f.request != nil {
		*f.request = request
	}
	return f.response, nil
}

func TestInvestigateCollectsRedactedEvidenceAndCallsAI(t *testing.T) {
	repo := NewMemoryEnvironmentRepository()
	_ = repo.Save(context.Background(), domain.Environment{ID: "env-1", OrganizationID: "org-a", Name: "Production", Type: "production", RunnerURL: "http://runner:8090"})
	var captured domain.InvestigationRequest
	ai := &fakeAIClient{
		response: domain.InvestigationResult{Summary: "API container unhealthy", Confidence: "high"},
		request:  &captured,
	}
	service := NewInvestigationService(repo, fakeToolClient{results: []domain.Evidence{
		{Source: "docker.logs", Output: "api unhealthy password=do-not-send-this"},
	}}, ai)

	result, err := service.Investigate(context.Background(), "org-a", "env-1", "Why is the API down?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Summary != "API container unhealthy" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if strings.Contains(captured.Evidence[0].Output, "do-not-send-this") {
		t.Fatal("raw secret was sent to AI")
	}
	if strings.Contains(result.Evidence[0].Output, "do-not-send-this") {
		t.Fatal("raw secret was returned for persistence")
	}

	if _, err := service.Investigate(context.Background(), "org-b", "env-1", "Why?"); err == nil {
		t.Fatal("expected cross-organization investigation to fail")
	}
}
