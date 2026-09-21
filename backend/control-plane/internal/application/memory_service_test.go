package application

import (
	"context"
	"strings"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeEmbeddingClient struct {
	vector []float64
}

func (f fakeEmbeddingClient) Embed(_ context.Context, texts []string) ([][]float64, error) {
	result := make([][]float64, len(texts))
	for index := range texts {
		result[index] = append([]float64(nil), f.vector...)
	}
	return result, nil
}

func unitMemoryVector() []float64 {
	vector := make([]float64, domain.IncidentMemoryDimensions)
	vector[0] = 1
	return vector
}

func TestIncidentMemoryRememberAndRecallIsScoped(t *testing.T) {
	repo := NewMemoryIncidentMemoryRepository()
	service := NewIncidentMemoryService(repo, fakeEmbeddingClient{vector: unitMemoryVector()})
	incident := domain.Incident{
		ID: "inc-1", OrganizationID: "org-a", EnvironmentID: "env-1",
		Question: "Why did postgres fail?",
		Summary: "API lost database connectivity",
		RootCause: "DB_HOST was incorrect",
		RecommendedAction: "Restore DB_HOST",
	}
	if err := service.Remember(context.Background(), incident); err != nil {
		t.Fatal(err)
	}

	evidence, err := service.Recall(context.Background(), "org-a", "env-1", "postgres connection", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 || evidence[0].Source != "incident.memory:inc-1" {
		t.Fatalf("unexpected recalled evidence: %#v", evidence)
	}

	otherOrg, err := service.Recall(context.Background(), "org-b", "env-1", "postgres connection", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherOrg) != 0 {
		t.Fatalf("cross-organization memory leaked: %#v", otherOrg)
	}

	otherEnvironment, err := service.Recall(context.Background(), "org-a", "env-2", "postgres connection", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(otherEnvironment) != 0 {
		t.Fatalf("cross-environment memory leaked: %#v", otherEnvironment)
	}
}

func TestMemoryContentExcludesRawEvidenceAndRedactsSummarySecrets(t *testing.T) {
	repo := NewMemoryIncidentMemoryRepository()
	service := NewIncidentMemoryService(repo, fakeEmbeddingClient{vector: unitMemoryVector()})
	incident := domain.Incident{
		ID: "inc-secret", OrganizationID: "org-a", EnvironmentID: "env-1",
		Question: "Why failed?",
		Summary: "Database error password=summary-secret",
		RootCause: "Bad configuration",
		RecommendedAction: "Restore config",
		Evidence: []domain.Evidence{{
			Source: "docker.logs",
			Output: "password=raw-evidence-secret",
			Success: true,
		}},
	}
	if err := service.Remember(context.Background(), incident); err != nil {
		t.Fatal(err)
	}

	evidence, err := service.Recall(context.Background(), "org-a", "env-1", "database error", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(evidence) != 1 {
		t.Fatalf("expected recalled memory, got %#v", evidence)
	}
	output := evidence[0].Output
	if strings.Contains(output, "raw-evidence-secret") || strings.Contains(output, "summary-secret") {
		t.Fatalf("secret leaked from long-term memory: %q", output)
	}
	if !strings.Contains(output, "Bad configuration") {
		t.Fatalf("root cause missing from memory: %q", output)
	}
}

func TestMemoryRepositoryRejectsWrongEmbeddingDimensions(t *testing.T) {
	repo := NewMemoryIncidentMemoryRepository()
	err := repo.Save(context.Background(), domain.IncidentMemory{
		ID: "bad", OrganizationID: "org-a", EnvironmentID: "env-1",
		IncidentID: "inc-1", Embedding: []float64{1},
	})
	if err == nil {
		t.Fatal("expected invalid embedding dimension to fail")
	}
}
