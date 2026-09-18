package application

import (
	"context"
	"testing"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

var incidentTestBaseTime = time.Date(2026, 9, 18, 4, 0, 0, 0, time.UTC)

type fakeInvestigator struct {
	contextEvidence *[]domain.Evidence
}

func (f fakeInvestigator) InvestigateWithEvidence(_ context.Context, _ string, _ string, _ string, contextual []domain.Evidence) (domain.InvestigationResult, error) {
	if f.contextEvidence != nil {
		*f.contextEvidence = append((*f.contextEvidence)[:0], contextual...)
	}
	return domain.InvestigationResult{
		Summary: "failure", Confidence: "high", ProbableRootCause: "bad deploy", RecommendedAction: "rollback",
		Evidence: []domain.Evidence{
			{
				Source: "docker.logs:api", Output: "error", Success: true,
				OccurredAt: incidentTestBaseTime.Add(2 * time.Minute), DurationMS: 42,
			},
			{
				Source: "github.commit_diff", Output: "file=config.yaml", Success: true,
				OccurredAt: incidentTestBaseTime.Add(90 * time.Second), DurationMS: 12,
			},
		},
	}, nil
}

type fakeRepositoryContext struct{}

func (fakeRepositoryContext) FetchContext(context.Context, domain.RepositoryIntegration) (domain.RepositoryContext, error) {
	return domain.RepositoryContext{
		Timeline: []domain.TimelineEvent{{
			Source: "github", Kind: "commit", Summary: "deploy change",
			OccurredAt: incidentTestBaseTime.Add(time.Minute),
		}},
		Evidence: []domain.Evidence{{
			Source: "github.commit_diff", Output: "file=config.yaml", Success: true,
			OccurredAt: incidentTestBaseTime.Add(90 * time.Second), DurationMS: 12,
		}},
	}, nil
}

func TestIncidentCorrelatesRepositoryContextBeforeDiagnosis(t *testing.T) {
	integrations := NewMemoryIntegrationRepository()
	_ = integrations.Save(context.Background(), domain.RepositoryIntegration{
		ID: "int-1", OrganizationID: "org-a", EnvironmentID: "env-1", Provider: "github",
	})
	contextEvidence := []domain.Evidence{}
	service := NewIncidentService(
		NewMemoryIncidentRepository(),
		integrations,
		fakeInvestigator{contextEvidence: &contextEvidence},
		fakeRepositoryContext{},
	)

	incident, err := service.Create(context.Background(), "org-a", "env-1", "why failed?")
	if err != nil {
		t.Fatal(err)
	}
	if len(contextEvidence) != 1 || contextEvidence[0].Source != "github.commit_diff" {
		t.Fatalf("repository evidence was not supplied to investigator: %#v", contextEvidence)
	}
	if incident.OrganizationID != "org-a" || incident.Status != "action_required" {
		t.Fatalf("unexpected incident: %#v", incident)
	}
	if len(incident.Timeline) != 3 {
		t.Fatalf("expected repository + diagnostic evidence timeline, got %#v", incident.Timeline)
	}
	if incident.Timeline[0].Source != "github" {
		t.Fatalf("expected GitHub event first chronologically: %#v", incident.Timeline)
	}
	if incident.Timeline[len(incident.Timeline)-1].Source != "docker.logs:api" {
		t.Fatalf("expected Docker evidence last chronologically: %#v", incident.Timeline)
	}
}
