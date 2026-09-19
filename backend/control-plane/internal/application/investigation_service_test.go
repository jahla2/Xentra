package application

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeToolClient struct {
	mu           sync.Mutex
	results      []domain.Evidence
	executed     *[]domain.ToolRequest
	toolOutput   string
	delay        time.Duration
	blockUntilCancel bool
	active       int
	maxActive    int
}

func (f *fakeToolClient) Collect(context.Context, domain.Environment) ([]domain.Evidence, error) {
	return f.results, nil
}

func (f *fakeToolClient) ExecuteReadTool(ctx context.Context, _ domain.Environment, request domain.ToolRequest) (domain.Evidence, error) {
	f.mu.Lock()
	if f.executed != nil {
		*f.executed = append(*f.executed, request)
	}
	f.active++
	if f.active > f.maxActive {
		f.maxActive = f.active
	}
	f.mu.Unlock()

	defer func() {
		f.mu.Lock()
		f.active--
		f.mu.Unlock()
	}()

	if f.blockUntilCancel {
		<-ctx.Done()
		return domain.Evidence{}, ctx.Err()
	}
	if f.delay > 0 {
		select {
		case <-time.After(f.delay):
		case <-ctx.Done():
			return domain.Evidence{}, ctx.Err()
		}
	}

	output := f.toolOutput
	if output == "" {
		output = "tool evidence"
	}
	return domain.Evidence{Source: request.Tool, Output: output, Success: true}, nil
}

func (f *fakeToolClient) concurrency() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.maxActive
}

func (f *fakeToolClient) executionCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.executed == nil {
		return 0
	}
	return len(*f.executed)
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


func TestInvestigateRunsIndependentToolRequestsConcurrently(t *testing.T) {
	repo, _ := testInvestigationEnvironment()
	executed := []domain.ToolRequest{}
	tools := &fakeToolClient{
		results: []domain.Evidence{{Source: "docker.list", Output: "api up", Success: true}},
		executed: &executed,
		delay: 80 * time.Millisecond,
	}
	final := domain.InvestigationResult{
		Summary: "done", Confidence: "medium",
		ProbableRootCause: "bounded test", RecommendedAction: "none",
	}
	ai := &fakeAIClient{
		response: final,
		steps: []domain.AgentDecision{
			{Mode: "tools", ToolRequests: []domain.ToolRequest{
				{Tool: "docker.logs", Arguments: map[string]string{"container": "api"}},
				{Tool: "docker.inspect", Arguments: map[string]string{"container": "api"}},
				{Tool: "docker.stats", Arguments: map[string]string{"container": "api"}},
			}},
			{Mode: "complete", Finding: &final},
		},
	}
	service := NewInvestigationService(repo, tools, ai)

	if _, err := service.Investigate(context.Background(), "org-a", "env-1", "Why?"); err != nil {
		t.Fatal(err)
	}
	if tools.executionCount() != 3 {
		t.Fatalf("expected 3 dynamic tool calls, got %d", tools.executionCount())
	}
	if tools.concurrency() < 2 {
		t.Fatalf("expected concurrent tool execution, max active=%d", tools.concurrency())
	}
}

func TestInvestigateEnforcesTotalToolCallBudgetIncludingBaseline(t *testing.T) {
	repo, _ := testInvestigationEnvironment()
	executed := []domain.ToolRequest{}
	tools := &fakeToolClient{
		results: []domain.Evidence{{Source: "system.info", Output: "linux", Success: true}},
		executed: &executed,
	}
	final := domain.InvestigationResult{
		Summary: "budgeted", Confidence: "low",
		ProbableRootCause: "unknown", RecommendedAction: "collect later",
	}
	ai := &fakeAIClient{
		response: final,
		steps: []domain.AgentDecision{
			{Mode: "tools", ToolRequests: []domain.ToolRequest{
				{Tool: "docker.logs", Arguments: map[string]string{"container": "api"}},
				{Tool: "docker.inspect", Arguments: map[string]string{"container": "api"}},
				{Tool: "docker.stats", Arguments: map[string]string{"container": "api"}},
			}},
			{Mode: "complete", Finding: &final},
		},
	}
	service := NewInvestigationService(repo, tools, ai)
	service.limits.MaxToolCalls = 7 // 5 baseline + only 2 targeted calls.

	result, err := service.Investigate(context.Background(), "org-a", "env-1", "Why?")
	if err != nil {
		t.Fatal(err)
	}
	if tools.executionCount() != 2 {
		t.Fatalf("expected only 2 targeted calls under total budget, got %d", tools.executionCount())
	}
	if len(result.Evidence) == 0 {
		t.Fatal("expected bounded evidence")
	}
}

func TestInvestigateBoundsEvidenceItemsAndBytes(t *testing.T) {
	repo, _ := testInvestigationEnvironment()
	tools := &fakeToolClient{results: []domain.Evidence{
		{Source: "one", Output: strings.Repeat("a", 70), Success: true},
		{Source: "two", Output: strings.Repeat("b", 70), Success: true},
		{Source: "three", Output: strings.Repeat("c", 70), Success: true},
	}}
	final := domain.InvestigationResult{
		Summary: "bounded", Confidence: "low",
		ProbableRootCause: "unknown", RecommendedAction: "none",
	}
	service := NewInvestigationService(repo, tools, &fakeAIClient{response: final})
	service.limits.MaxEvidenceItems = 2
	service.limits.MaxEvidenceBytes = 90

	result, err := service.Investigate(context.Background(), "org-a", "env-1", "Why?")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Evidence) > 2 {
		t.Fatalf("evidence item cap exceeded: %#v", result.Evidence)
	}
	total := 0
	for _, item := range result.Evidence {
		total += len(item.Source) + len(item.Output)
	}
	if total > 90 {
		t.Fatalf("evidence byte cap exceeded: %d", total)
	}
}

func TestInvestigateHonorsTotalDeadline(t *testing.T) {
	repo, _ := testInvestigationEnvironment()
	executed := []domain.ToolRequest{}
	tools := &fakeToolClient{
		results: []domain.Evidence{{Source: "docker.list", Output: "api up", Success: true}},
		executed: &executed,
		blockUntilCancel: true,
	}
	ai := &fakeAIClient{
		response: domain.InvestigationResult{
			Summary: "fallback", Confidence: "low",
			ProbableRootCause: "unknown", RecommendedAction: "none",
		},
		steps: []domain.AgentDecision{
			{Mode: "tools", ToolRequests: []domain.ToolRequest{
				{Tool: "docker.logs", Arguments: map[string]string{"container": "api"}},
			}},
		},
	}
	service := NewInvestigationService(repo, tools, ai)
	service.limits.Timeout = 25 * time.Millisecond

	_, err := service.Investigate(context.Background(), "org-a", "env-1", "Why?")
	if err == nil || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestInvestigateEmitsLiveProgressThroughCompletion(t *testing.T) {
	repo, _ := testInvestigationEnvironment()
	tools := &fakeToolClient{results: []domain.Evidence{{Source: "system.info", Output: "linux", Success: true}}}
	final := domain.InvestigationResult{
		Summary: "complete", Confidence: "medium",
		ProbableRootCause: "test", RecommendedAction: "none",
	}
	service := NewInvestigationService(repo, tools, &fakeAIClient{response: final})
	stages := []string{}

	result, err := service.InvestigateWithProgress(
		context.Background(), "org-a", "env-1", "Why?",
		func(event domain.InvestigationProgress) { stages = append(stages, event.Stage) },
	)
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary != "complete" {
		t.Fatalf("unexpected result: %#v", result)
	}
	for _, expected := range []string{"collecting", "evidence", "reasoning", "complete"} {
		found := false
		for _, stage := range stages {
			if stage == expected {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing progress stage %q in %#v", expected, stages)
		}
	}
}
