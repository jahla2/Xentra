package application

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

const (
	defaultMemoryRecallLimit = 3
	maxMemoryDistance        = 0.65
)

type EmbeddingClient interface {
	Embed(context.Context, []string) ([][]float64, error)
}

type IncidentMemoryRepository interface {
	Save(context.Context, domain.IncidentMemory) error
	Search(context.Context, string, string, []float64, int) ([]domain.IncidentMemoryMatch, error)
}

type IncidentMemory interface {
	Recall(context.Context, string, string, string, int) ([]domain.Evidence, error)
	Remember(context.Context, domain.Incident) error
}

type IncidentMemoryService struct {
	repo       IncidentMemoryRepository
	embeddings EmbeddingClient
	redactor   *EvidenceRedactor
}

func NewIncidentMemoryService(repo IncidentMemoryRepository, embeddings EmbeddingClient) *IncidentMemoryService {
	return &IncidentMemoryService{
		repo:       repo,
		embeddings: embeddings,
		redactor:   NewEvidenceRedactor(),
	}
}

func (s *IncidentMemoryService) Recall(
	ctx context.Context,
	organizationID, environmentID, query string,
	limit int,
) ([]domain.Evidence, error) {
	if s == nil || s.repo == nil || s.embeddings == nil || query == "" {
		return nil, nil
	}
	if organizationID == "" || environmentID == "" {
		return nil, errors.New("organization and environment are required for incident memory")
	}
	if limit <= 0 {
		limit = defaultMemoryRecallLimit
	}

	vectors, err := s.embeddings.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed memory query: %w", err)
	}
	if len(vectors) != 1 || len(vectors[0]) != domain.IncidentMemoryDimensions {
		return nil, errors.New("embedding service returned an invalid memory query response")
	}

	matches, err := s.repo.Search(ctx, organizationID, environmentID, vectors[0], limit)
	if err != nil {
		return nil, fmt.Errorf("search incident memory: %w", err)
	}

	evidence := make([]domain.Evidence, 0, len(matches))
	for _, match := range matches {
		if match.Distance > maxMemoryDistance {
			continue
		}
		similarity := math.Max(0, 1-match.Distance)
		output := fmt.Sprintf(
			"Past incident similarity=%.2f
%s",
			similarity,
			match.Memory.Content,
		)
		evidence = append(evidence, domain.Evidence{
			Source:     fmt.Sprintf("incident.memory:%s", match.Memory.IncidentID),
			Output:     s.redactor.Redact(output),
			Success:    true,
			OccurredAt: match.Memory.CreatedAt,
		})
	}
	return evidence, nil
}

func (s *IncidentMemoryService) Remember(ctx context.Context, incident domain.Incident) error {
	if s == nil || s.repo == nil || s.embeddings == nil {
		return nil
	}
	if incident.OrganizationID == "" || incident.EnvironmentID == "" || incident.ID == "" {
		return errors.New("incident memory requires incident, organization and environment IDs")
	}

	content := s.redactor.Redact(memoryContent(incident))
	vectors, err := s.embeddings.Embed(ctx, []string{content})
	if err != nil {
		return fmt.Errorf("embed incident memory: %w", err)
	}
	if len(vectors) != 1 || len(vectors[0]) != domain.IncidentMemoryDimensions {
		return errors.New("embedding service returned an invalid incident memory response")
	}

	id, err := newResourceID("mem")
	if err != nil {
		return err
	}
	return s.repo.Save(ctx, domain.IncidentMemory{
		ID:             id,
		OrganizationID: incident.OrganizationID,
		EnvironmentID:  incident.EnvironmentID,
		IncidentID:     incident.ID,
		Content:        content,
		Embedding:      vectors[0],
		CreatedAt:      time.Now().UTC(),
	})
}

func memoryContent(incident domain.Incident) string {
	return fmt.Sprintf(
		"Question: %s
Summary: %s
Root cause: %s
Recommended action: %s",
		incident.Question,
		incident.Summary,
		incident.RootCause,
		incident.RecommendedAction,
	)
}

type MemoryIncidentMemoryRepository struct {
	mu   sync.RWMutex
	data map[string]domain.IncidentMemory
}

func NewMemoryIncidentMemoryRepository() *MemoryIncidentMemoryRepository {
	return &MemoryIncidentMemoryRepository{data: map[string]domain.IncidentMemory{}}
}

func (r *MemoryIncidentMemoryRepository) Save(_ context.Context, item domain.IncidentMemory) error {
	if len(item.Embedding) != domain.IncidentMemoryDimensions {
		return fmt.Errorf(
			"incident memory embedding must contain %d dimensions",
			domain.IncidentMemoryDimensions,
		)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	item.Embedding = append([]float64(nil), item.Embedding...)
	r.data[item.ID] = item
	return nil
}

func (r *MemoryIncidentMemoryRepository) Search(
	_ context.Context,
	organizationID, environmentID string,
	embedding []float64,
	limit int,
) ([]domain.IncidentMemoryMatch, error) {
	if len(embedding) != domain.IncidentMemoryDimensions {
		return nil, fmt.Errorf(
			"incident memory query must contain %d dimensions",
			domain.IncidentMemoryDimensions,
		)
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	matches := []domain.IncidentMemoryMatch{}
	for _, item := range r.data {
		if item.OrganizationID != organizationID || item.EnvironmentID != environmentID {
			continue
		}
		matches = append(matches, domain.IncidentMemoryMatch{
			Memory:   item,
			Distance: cosineDistance(embedding, item.Embedding),
		})
	}

	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Distance < matches[j].Distance
	})
	if limit > 0 && len(matches) > limit {
		matches = matches[:limit]
	}
	return matches, nil
}

func cosineDistance(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 1
	}
	var dot, aa, bb float64
	for index := range a {
		dot += a[index] * b[index]
		aa += a[index] * a[index]
		bb += b[index] * b[index]
	}
	if aa == 0 || bb == 0 {
		return 1
	}
	similarity := dot / (math.Sqrt(aa) * math.Sqrt(bb))
	return 1 - similarity
}
