package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type Investigator interface {
	InvestigateWithEvidence(context.Context, string, string, string, []domain.Evidence) (domain.InvestigationResult, error)
}

type RepositoryContextClient interface {
	FetchContext(context.Context, domain.RepositoryIntegration) (domain.RepositoryContext, error)
}

type IncidentRepository interface {
	Save(context.Context, domain.Incident) error
	Get(context.Context, string, string) (domain.Incident, error)
	List(context.Context, string) ([]domain.Incident, error)
}

type IncidentService struct {
	repo              IncidentRepository
	integrations      RepositoryIntegrationRepository
	investigator      Investigator
	repositoryContext RepositoryContextClient
	memory            IncidentMemory
}

func NewIncidentService(
	repo IncidentRepository,
	integrations RepositoryIntegrationRepository,
	investigator Investigator,
	repositoryContext RepositoryContextClient,
	memory ...IncidentMemory,
) *IncidentService {
	var incidentMemory IncidentMemory
	if len(memory) > 0 {
		incidentMemory = memory[0]
	}
	return &IncidentService{
		repo: repo, integrations: integrations, investigator: investigator,
		repositoryContext: repositoryContext, memory: incidentMemory,
	}
}

func (s *IncidentService) Create(ctx context.Context, organizationID, environmentID, question string) (domain.Incident, error) {
	if organizationID == "" || environmentID == "" || question == "" {
		return domain.Incident{}, errors.New("organization, environmentId and question are required")
	}
	var repositoryEvidence []domain.Evidence
	var repositoryTimeline []domain.TimelineEvent
	var repositoryError error
	if s.integrations != nil && s.repositoryContext != nil {
		if integration, findErr := s.integrations.FindByEnvironment(ctx, organizationID, environmentID); findErr == nil {
			repositoryContext, contextErr := s.repositoryContext.FetchContext(ctx, integration)
			if contextErr != nil {
				repositoryError = contextErr
			} else {
				repositoryEvidence = repositoryContext.Evidence
				repositoryTimeline = repositoryContext.Timeline
			}
		}
	}

	result, err := s.investigator.InvestigateWithEvidence(ctx, organizationID, environmentID, question, repositoryEvidence)
	if err != nil {
		return domain.Incident{}, err
	}
	events := evidenceTimelineEvents(result.Evidence)
	events = append(events, repositoryTimeline...)
	if repositoryError != nil {
		events = append(events, domain.TimelineEvent{
			Source: "github", Kind: "integration_error",
			Summary: repositoryError.Error(), OccurredAt: time.Now().UTC(),
		})
	}
	sort.SliceStable(events, func(i, j int) bool { return events[i].OccurredAt.Before(events[j].OccurredAt) })
	status := "investigating"
	if result.Confidence == "high" {
		status = "action_required"
	}
	id, err := newResourceID("inc")
	if err != nil {
		return domain.Incident{}, err
	}
	incident := domain.Incident{
		ID: id, OrganizationID: organizationID, EnvironmentID: environmentID, Question: question,
		Status: status, Summary: result.Summary, RootCause: result.ProbableRootCause,
		Confidence: result.Confidence, RecommendedAction: result.RecommendedAction,
		Evidence: result.Evidence, Timeline: events, CreatedAt: time.Now().UTC(),
	}
	if err := s.repo.Save(ctx, incident); err != nil {
		return domain.Incident{}, err
	}
	if s.memory != nil {
		if rememberErr := s.memory.Remember(ctx, incident); rememberErr != nil {
			incident.Timeline = append(incident.Timeline, domain.TimelineEvent{
				Source: "memory", Kind: "memory_error",
				Summary: rememberErr.Error(), OccurredAt: time.Now().UTC(),
			})
			_ = s.repo.Save(ctx, incident)
		}
	}
	return incident, nil
}

func (s *IncidentService) List(ctx context.Context, organizationID string) ([]domain.Incident, error) {
	return s.repo.List(ctx, organizationID)
}

type MemoryIncidentRepository struct {
	mu   sync.RWMutex
	data map[string]domain.Incident
}

func NewMemoryIncidentRepository() *MemoryIncidentRepository {
	return &MemoryIncidentRepository{data: map[string]domain.Incident{}}
}

func (r *MemoryIncidentRepository) Save(_ context.Context, item domain.Incident) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[item.ID] = item
	return nil
}

func (r *MemoryIncidentRepository) Get(_ context.Context, organizationID, id string) (domain.Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.data[id]
	if !ok || item.OrganizationID != organizationID {
		return domain.Incident{}, errors.New("incident not found")
	}
	return item, nil
}

func (r *MemoryIncidentRepository) List(_ context.Context, organizationID string) ([]domain.Incident, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := []domain.Incident{}
	for _, item := range r.data {
		if item.OrganizationID == organizationID {
			items = append(items, item)
		}
	}
	return items, nil
}

func evidenceTimelineEvents(evidence []domain.Evidence) []domain.TimelineEvent {
	events := make([]domain.TimelineEvent, 0, len(evidence))
	for _, item := range evidence {
		summary := fmt.Sprintf("Collected %s (%d ms)", item.Source, item.DurationMS)
		kind := "evidence"
		if !item.Success {
			summary = fmt.Sprintf("Failed %s (%d ms)", item.Source, item.DurationMS)
			kind = "evidence_error"
		}
		events = append(events, domain.TimelineEvent{
			Source: item.Source,
			Kind: kind,
			Summary: summary,
			OccurredAt: item.OccurredAt,
		})
	}
	return events
}
