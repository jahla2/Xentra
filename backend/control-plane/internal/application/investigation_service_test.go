package application

import (
	"context"
	"strings"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeToolClient struct {
	results   []domain.Evidence
	executed  *[]domain.ToolRequest
	toolOutput string
}

func (f *fakeToolClient) Collect(context.Context, domain.Environment) ([]domain.Evidence, error) {
	return f.results, nil
}

func (f *fakeToolClient) ExecuteReadTool(_ context.Context, _ domain.Environment, request domain.ToolRequest) (domain.Evidence, error) {
	if f.executed != nil {
		*f.executed = append(*f.executed, request)
	}
	output := f.toolOutput
	if output == "" {
		output = "tool evidence"
	}
	return domain.Evidence{Source: request.Tool, Output: output, Success: true}, nil
}

type fakeAIClient struct {
	response  domain.InvestigationResult
	request   *domain.InvestigationRequest
	steps     []domain.AgentDecision
	stepCalls *[]domain.AgentInvestigationRequest
	index     int
}

func (f *fakeAIClient) Investigate(_ context.Context, request domain.InvestigationRequest) (domain.InvestigationResult, error) {
	if f.request != nil {
		*f.request = request
	}
	return f.response, nil
}

func (f *fakeAIClient) Next(_ context.Context, request domain.AgentInvestigationRequest) (domain.AgentDecision, error) {
	if f.stepCalls != nil {
		*f.stepCalls = append(*f.stepCalls, request)
	}
	if f.index >= len(f.steps) {
		return domain.AgentDecision{Mode: "complete", Finding: &f.response}, nil
	}
	decision := f.steps[f.index]
	f.index++
	return decision, nil
}

func testInvestigationEnvironment() (*MemoryEnvironmentRepository, domain.Environment) {
	repo := NewMemoryEnvironmentRepository()
	env := domain.Environment{
		ID: "env-1", OrganizationID: "org-a", Name: "Production", Type: "production",
		RunnerURL: "http://runner:8090", Capabilities: []string{"docker", "systemd", "git", "http", "dns"},
	}
	_ = repo.Save(context.Background(), env)
	return repo, env
}

func TestInvestigateExecutesRequestedReadToolAndRedactsEvidence(t *testing.T) {
	repo, _ := testInvestigationEnvironment()
	executed := []domain.ToolRequest{}
	stepCalls := []domain.AgentInvestigationRequest{}
	tools := &fakeToolClient{
		results: []domain.Evidence{{Source: "docker.list", Output: "api Up 2 seconds", Success: true}},
		executed: &executed,
		toolOutput: "password=do-not-send connection refused to postgres",
	}
	final := domain.InvestigationResult{
		Summary: "Database connectivity failure", Confidence: "high",
		ProbableRootCause: "API cannot reach PostgreSQL", RecommendedAction: "Verify DB configuration",
	}
	ai := &fakeAIClient{
		response: final, stepCalls: &stepCalls,
		steps: []domain.AgentDecision{
			{Mode: "tools", ToolRequests: []domain.ToolRequest{{Tool: "docker.logs", Arguments: map[string]string{"container": "api"}}}},
			{Mode: "complete", Finding: &final},
		},
	}
	service := NewInvestigationService(repo, tools, ai)

	result, err := service.Investigate(context.Background(), "org-a", "env-1", "Why is the API down?")
	if err != nil {
		t.Fatal(err)
	}
	if len(executed) != 1 || executed[0].Tool != "docker.logs" {
		t.Fatalf("unexpected executed tools: %#v", executed)
	}
	if len(stepCalls) != 2 {
		t.Fatalf("expected two agent steps, got %d", len(stepCalls))
	}
	if strings.Contains(stepCalls[1].Evidence[len(stepCalls[1].Evidence)-1].Output, "do-not-send") {
		t.Fatal("tool secret was sent back to AI")
	}
	if strings.Contains(result.Evidence[len(result.Evidence)-1].Output, "do-not-send") {
		t.Fatal("tool secret was returned for persistence")
	}
	if result.Confidence != "high" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestInvestigateRejectsMutationToolRequestedByAI(t *testing.T) {
	repo, _ := testInvestigationEnvironment()
	executed := []domain.ToolRequest{}
	tools := &fakeToolClient{results: []domain.Evidence{{Source: "docker.list", Output: "api up", Success: true}}, executed: &executed}
	fallback := domain.InvestigationResult{Summary: "Fallback", Confidence: "low", ProbableRootCause: "Unknown"}
	ai := &fakeAIClient{
		response: fallback,
		steps: []domain.AgentDecision{{Mode: "tools", ToolRequests: []domain.ToolRequest{{Tool: "docker.restart", Arguments: map[string]string{"container": "api"}}}}},
	}
	service := NewInvestigationService(repo, tools, ai)

	result, err := service.Investigate(context.Background(), "org-a", "env-1", "Why?")
	if err != nil {
		t.Fatal(err)
	}
	if len(executed) != 0 {
		t.Fatalf("mutation tool was executed: %#v", executed)
	}
	if result.Summary != "Fallback" {
		t.Fatalf("expected deterministic fallback finding, got %#v", result)
	}
}

func TestInvestigateRejectsCrossOrganizationEnvironment(t *testing.T) {
	repo, _ := testInvestigationEnvironment()
	service := NewInvestigationService(repo, &fakeToolClient{}, &fakeAIClient{})
	if _, err := service.Investigate(context.Background(), "org-b", "env-1", "Why?"); err == nil {
		t.Fatal("expected cross-organization investigation to fail")
	}
}


func TestAvailableInvestigationToolsIncludeMVPReadOnlyDiagnostics(t *testing.T) {
	tools := availableInvestigationTools(domain.Environment{Capabilities: []string{"docker", "systemd", "git", "http", "dns"}})
	for _, expected := range []string{"git.status", "git.log", "git.diff", "git.show_commit", "http.health_check", "dns.lookup"} {
		found := false
		for _, tool := range tools {
			if tool == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected %s in available tools: %#v", expected, tools)
		}
	}
}

func TestValidateInvestigationToolRequestRequiresTypedNetworkAndCommitArguments(t *testing.T) {
	available := []string{"git.show_commit", "http.health_check", "dns.lookup"}
	cases := []domain.ToolRequest{
		{Tool: "git.show_commit", Arguments: map[string]string{}},
		{Tool: "http.health_check", Arguments: map[string]string{}},
		{Tool: "dns.lookup", Arguments: map[string]string{}},
	}
	for _, request := range cases {
		if err := validateInvestigationToolRequest(request, available); err == nil {
			t.Fatalf("expected %s without required argument to be rejected", request.Tool)
		}
	}
}
