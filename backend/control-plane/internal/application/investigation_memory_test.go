package application

import (
	"context"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type recallMemoryStub struct {
	evidence []domain.Evidence
}

func (r recallMemoryStub) Recall(context.Context, string, string, string, int) ([]domain.Evidence, error) {
	return r.evidence, nil
}

func (recallMemoryStub) Remember(context.Context, domain.Incident) error { return nil }

type memoryToolStub struct{}

func (memoryToolStub) Collect(context.Context, domain.Environment) ([]domain.Evidence, error) {
	return []domain.Evidence{{Source: "system.info", Output: "linux", Success: true}}, nil
}

func (memoryToolStub) ExecuteReadTool(context.Context, domain.Environment, domain.ToolRequest) (domain.Evidence, error) {
	return domain.Evidence{}, nil
}

type memoryAIStub struct {
	captured *domain.AgentInvestigationRequest
}

func (m memoryAIStub) Investigate(_ context.Context, req domain.InvestigationRequest) (domain.InvestigationResult, error) {
	return domain.InvestigationResult{Summary: "fallback", Confidence: "low", Evidence: req.Evidence}, nil
}

func (m memoryAIStub) Next(_ context.Context, req domain.AgentInvestigationRequest) (domain.AgentDecision, error) {
	if m.captured != nil {
		*m.captured = req
	}
	finding := domain.InvestigationResult{
		Summary: "Used prior incident",
		Confidence: "medium",
		ProbableRootCause: "Known configuration failure",
		RecommendedAction: "Verify configuration",
	}
	return domain.AgentDecision{Mode: "complete", Finding: &finding}, nil
}

func TestAskAIIncludesRecalledIncidentMemory(t *testing.T) {
	repo := NewMemoryEnvironmentRepository()
	_ = repo.Save(context.Background(), domain.Environment{
		ID: "env-1", OrganizationID: "org-a", Name: "Production",
		ConnectionType: "runner",
	})
	var captured domain.AgentInvestigationRequest
	service := NewInvestigationService(
		repo,
		memoryToolStub{},
		memoryAIStub{captured: &captured},
		recallMemoryStub{evidence: []domain.Evidence{{
			Source: "incident.memory:inc-old",
			Output: "Past incident: DB_HOST was wrong",
			Success: true,
		}}},
	)

	_, err := service.Investigate(context.Background(), "org-a", "env-1", "Why is the API down?")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range captured.Evidence {
		if item.Source == "incident.memory:inc-old" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("recalled incident memory was not supplied to the agent: %#v", captured.Evidence)
	}
}
