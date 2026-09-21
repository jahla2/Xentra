package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type streamInvestigationTools struct{}

func (streamInvestigationTools) Collect(context.Context, domain.Environment) ([]domain.Evidence, error) {
	return []domain.Evidence{{
		Source: "system.info", Output: "Linux", Success: true, OccurredAt: time.Now().UTC(),
	}}, nil
}

func (streamInvestigationTools) ExecuteReadTool(_ context.Context, _ domain.Environment, request domain.ToolRequest) (domain.Evidence, error) {
	return domain.Evidence{Source: request.Tool, Output: "ok", Success: true, OccurredAt: time.Now().UTC()}, nil
}

type streamInvestigationAI struct{}

func (streamInvestigationAI) Investigate(context.Context, domain.InvestigationRequest) (domain.InvestigationResult, error) {
	return domain.InvestigationResult{
		Summary: "complete", Confidence: "medium", ProbableRootCause: "test",
		RecommendedAction: "none",
	}, nil
}

func (streamInvestigationAI) Next(context.Context, domain.AgentInvestigationRequest) (domain.AgentDecision, error) {
	finding := domain.InvestigationResult{
		Summary: "complete", Confidence: "medium", ProbableRootCause: "test",
		RecommendedAction: "none",
	}
	return domain.AgentDecision{Mode: "complete", Finding: &finding}, nil
}

func TestInvestigationStreamEmitsProgressAndFinalFinding(t *testing.T) {
	auth := application.NewAuthService(application.NewMemoryAuthRepository(), time.Hour)
	session, err := auth.Register(context.Background(), "owner@example.com", "very-secure-password", "Acme")
	if err != nil {
		t.Fatal(err)
	}

	envs := application.NewMemoryEnvironmentRepository()
	_ = envs.Save(context.Background(), domain.Environment{
		ID: "env-1", OrganizationID: session.Principal.OrganizationID,
		Name: "Production", Type: "production", Capabilities: []string{"docker"},
	})
	investigations := application.NewInvestigationService(envs, streamInvestigationTools{}, streamInvestigationAI{})
	router := NewRouter(nil, investigations, Services{Auth: auth})

	req := httptest.NewRequest(http.MethodPost, "/api/investigations/stream", bytes.NewBufferString(
		`{"environmentId":"env-1","question":"Why is the API down?"}`,
	))
	req.Header.Set("Authorization", "Bearer "+session.Token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("stream status=%d body=%s", rec.Code, rec.Body.String())
	}
	if contentType := rec.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/event-stream") {
		t.Fatalf("unexpected content type %q", contentType)
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`"stage":"collecting"`,
		`"stage":"evidence"`,
		`"stage":"reasoning"`,
		`"stage":"complete"`,
		`"summary":"complete"`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("stream missing %s: %s", expected, body)
		}
	}
}
