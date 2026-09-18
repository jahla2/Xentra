package application

import (
	"context"
	"testing"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

var incidentTestBaseTime = time.Date(2026, 9, 18, 4, 0, 0, 0, time.UTC)

type fakeInvestigator struct{}

func (fakeInvestigator) Investigate(context.Context, string, string, string) (domain.InvestigationResult, error) {
	return domain.InvestigationResult{
		Summary: "failure", Confidence: "high", ProbableRootCause: "bad deploy", RecommendedAction: "rollback",
		Evidence: []domain.Evidence{{
			Source: "docker.logs:api", Output: "error", Success: true,
			OccurredAt: incidentTestBaseTime.Add(2 * time.Minute), DurationMS: 42,
		}},
	}, nil
}

type fakeTimeline struct{}

func (fakeTimeline) FetchTimeline(context.Context, domain.RepositoryIntegration) ([]domain.TimelineEvent, error) {
	return []domain.TimelineEvent{{
		Source: "github", Kind: "commit", Summary: "deploy change",
		OccurredAt: incidentTestBaseTime.Add(time.Minute),
	}}, nil
}

func TestIncidentCreatesChronologicalTimelineAndActionRequiredStatus(t *testing.T) {
	integrations := NewMemoryIntegrationRepository()
	_ = integrations.Save(context.Background(), domain.RepositoryIntegration{
		ID: "int-1", OrganizationID: "org-a", EnvironmentID: "env-1", Provider: "github",
	})
	service := NewIncidentService(NewMemoryIncidentRepository(), integrations, fakeInvestigator{}, fakeTimeline{})

	incident, err := service.Create(context.Background(), "org-a", "env-1", "why failed?")
	if err != nil {
		t.Fatal(err)
	}
	if incident.OrganizationID != "org-a" || incident.Status != "action_required" || len(incident.Evidence) != 1 {
		t.Fatalf("unexpected incident: %#v", incident)
	}
	if len(incident.Timeline) != 2 {
		t.Fatalf("expected GitHub + evidence timeline events, got %#v", incident.Timeline)
	}
	if incident.Timeline[0].Source != "github" || incident.Timeline[1].Source != "docker.logs:api" {
		t.Fatalf("timeline is not chronological: %#v", incident.Timeline)
	}
	if incident.Timeline[1].Kind != "evidence" || incident.Timeline[1].Summary != "Collected docker.logs:api (42 ms)" {
		t.Fatalf("unexpected evidence event: %#v", incident.Timeline[1])
	}
}
