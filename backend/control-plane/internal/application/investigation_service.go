package application

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

const (
	maxAgentSteps          = 3
	maxToolRequestsPerStep = 3
)

type InvestigationLimits struct {
	Timeout          time.Duration
	MaxToolCalls     int
	MaxEvidenceItems int
	MaxEvidenceBytes int
	MaxQuestionChars int
}

func defaultInvestigationLimits() InvestigationLimits {
	return InvestigationLimits{
		Timeout:          time.Duration(investigationEnvInt("XENTRA_INVESTIGATION_TIMEOUT_SECONDS", 45, 5, 120)) * time.Second,
		MaxToolCalls:     investigationEnvInt("XENTRA_INVESTIGATION_MAX_TOOL_CALLS", 8, 5, 24),
		MaxEvidenceItems: investigationEnvInt("XENTRA_INVESTIGATION_MAX_EVIDENCE_ITEMS", 48, 8, 128),
		MaxEvidenceBytes: investigationEnvInt("XENTRA_INVESTIGATION_MAX_EVIDENCE_BYTES", 96*1024, 16*1024, 512*1024),
		MaxQuestionChars: investigationEnvInt("XENTRA_INVESTIGATION_MAX_QUESTION_CHARS", 4000, 256, 16000),
	}
}

type ToolClient interface {
	Collect(context.Context, domain.Environment) ([]domain.Evidence, error)
	ExecuteReadTool(context.Context, domain.Environment, domain.ToolRequest) (domain.Evidence, error)
}

type AIClient interface {
	Investigate(context.Context, domain.InvestigationRequest) (domain.InvestigationResult, error)
	Next(context.Context, domain.AgentInvestigationRequest) (domain.AgentDecision, error)
}

type InvestigationService struct {
	repo     EnvironmentRepository
	tools    ToolClient
	ai       AIClient
	memory   IncidentMemory
	redactor *EvidenceRedactor
	limits   InvestigationLimits
}

func NewInvestigationService(
	repo EnvironmentRepository,
	tools ToolClient,
	ai AIClient,
	memory ...IncidentMemory,
) *InvestigationService {
	var incidentMemory IncidentMemory
	if len(memory) > 0 {
		incidentMemory = memory[0]
	}
	return &InvestigationService{
		repo: repo, tools: tools, ai: ai, memory: incidentMemory,
		redactor: NewEvidenceRedactor(), limits: defaultInvestigationLimits(),
	}
}

func (s *InvestigationService) Investigate(ctx context.Context, organizationID, environmentID, question string) (domain.InvestigationResult, error) {
	return s.InvestigateWithEvidence(ctx, organizationID, environmentID, question, nil)
}

func (s *InvestigationService) InvestigateWithEvidence(ctx context.Context, organizationID, environmentID, question string, contextualEvidence []domain.Evidence) (domain.InvestigationResult, error) {
	return s.investigate(ctx, organizationID, environmentID, question, contextualEvidence, nil)
}

func (s *InvestigationService) InvestigateWithProgress(
	ctx context.Context,
	organizationID, environmentID, question string,
	progress func(domain.InvestigationProgress),
) (domain.InvestigationResult, error) {
	return s.investigate(ctx, organizationID, environmentID, question, nil, progress)
}

func (s *InvestigationService) investigate(
	ctx context.Context,
	organizationID, environmentID, question string,
	contextualEvidence []domain.Evidence,
	progress func(domain.InvestigationProgress),
) (domain.InvestigationResult, error) {
	question = strings.TrimSpace(question)
	if question == "" {
		return domain.InvestigationResult{}, errors.New("question is required")
	}
	if len(question) > s.limits.MaxQuestionChars {
		return domain.InvestigationResult{}, fmt.Errorf("question exceeds %d character investigation limit", s.limits.MaxQuestionChars)
	}

	runCtx, cancel := context.WithTimeout(ctx, s.limits.Timeout)
	defer cancel()
	startedAt := time.Now().UTC()
	toolCalls := 0
	safeEvidence := []domain.Evidence{}

	emit := func(stage, message string, evidence *domain.Evidence, result *domain.InvestigationResult) {
		if progress == nil {
			return
		}
		progress(domain.InvestigationProgress{
			Stage: stage, Message: message, Evidence: evidence, Result: result,
			ToolCalls: toolCalls, EvidenceCount: len(safeEvidence),
			ElapsedMS: time.Since(startedAt).Milliseconds(),
		})
	}

	env, err := s.repo.Get(runCtx, organizationID, environmentID)
	if err != nil {
		return domain.InvestigationResult{}, err
	}

	emit("collecting", "Collecting independent system and runtime evidence", nil, nil)
	evidence, err := s.tools.Collect(runCtx, env)
	if err != nil {
		return domain.InvestigationResult{}, fmt.Errorf("collect evidence: %w", err)
	}
	// Baseline collection always attempts five typed diagnostics:
	// system.info, system.disk, system.cpu, system.memory, and docker.list.
	toolCalls = 5
	evidence = append(evidence, contextualEvidence...)

	if s.memory != nil {
		memoryEvidence, memoryErr := s.memory.Recall(
			runCtx, organizationID, environmentID, question, defaultMemoryRecallLimit,
		)
		if memoryErr != nil {
			memoryEvidence = []domain.Evidence{{
				Source: "incident.memory", Output: memoryErr.Error(), Success: false,
				OccurredAt: time.Now().UTC(),
			}}
		}
		evidence = append(evidence, memoryEvidence...)
	}

	safeEvidence = s.boundEvidence(s.redactor.RedactEvidence(normalizeEvidenceTiming(evidence)))
	for index := range safeEvidence {
		item := safeEvidence[index]
		emit("evidence", "Collected "+item.Source, &item, nil)
	}

	availableTools := availableInvestigationTools(env)
	seen := map[string]bool{}

	for step := 0; step < maxAgentSteps; step++ {
		if err := runCtx.Err(); err != nil {
			return domain.InvestigationResult{}, fmt.Errorf("investigation deadline exceeded: %w", err)
		}

		emit("reasoning", fmt.Sprintf("Reasoning step %d of %d", step+1, maxAgentSteps), nil, nil)
		decision, decisionErr := s.ai.Next(runCtx, domain.AgentInvestigationRequest{
			Environment: env, Question: question, Evidence: safeEvidence,
			AvailableTools: availableTools, RemainingSteps: maxAgentSteps - step,
		})
		if decisionErr != nil {
			if runCtx.Err() != nil {
				return domain.InvestigationResult{}, fmt.Errorf("investigation deadline exceeded: %w", runCtx.Err())
			}
			emit("reasoning_error", "Agent decision failed; using bounded fallback analysis", nil, nil)
			break
		}
		if decision.Mode == "complete" && decision.Finding != nil {
			result := *decision.Finding
			result.Evidence = safeEvidence
			emit("complete", "Investigation complete", nil, &result)
			return result, nil
		}
		if decision.Mode != "tools" || len(decision.ToolRequests) == 0 {
			break
		}

		requests := decision.ToolRequests
		if len(requests) > maxToolRequestsPerStep {
			requests = requests[:maxToolRequestsPerStep]
		}
		remainingCalls := s.limits.MaxToolCalls - toolCalls
		if remainingCalls <= 0 {
			limitItem := domain.Evidence{
				Source: "agent.limit", Success: false,
				Output: fmt.Sprintf("dynamic tool-call budget reached (%d)", s.limits.MaxToolCalls),
				OccurredAt: time.Now().UTC(),
			}
			safeEvidence = s.boundEvidence(append(safeEvidence, limitItem))
			emit("limit", limitItem.Output, &limitItem, nil)
			break
		}
		if len(requests) > remainingCalls {
			requests = requests[:remainingCalls]
		}

		validRequests := make([]domain.ToolRequest, 0, len(requests))
		for _, request := range requests {
			key := toolRequestKey(request)
			if seen[key] {
				continue
			}
			seen[key] = true
			if err := validateInvestigationToolRequest(request, availableTools); err != nil {
				item := domain.Evidence{
					Source: "tool.request_rejected", Output: err.Error(), Success: false,
					OccurredAt: time.Now().UTC(),
				}
				safeEvidence = s.boundEvidence(append(safeEvidence, item))
				emit("evidence", "Rejected unsafe or unavailable tool request", &item, nil)
				continue
			}
			validRequests = append(validRequests, request)
		}
		if len(validRequests) == 0 {
			break
		}

		toolCalls += len(validRequests)
		emit("tools", fmt.Sprintf("Running %d independent typed tools concurrently", len(validRequests)), nil, nil)
		resultCh := make(chan domain.Evidence, len(validRequests))
		for _, request := range validRequests {
			request := request
			go func() {
				item, toolErr := s.tools.ExecuteReadTool(runCtx, env, request)
				if toolErr != nil {
					item = domain.Evidence{
						Source: request.Tool, Output: toolErr.Error(), Success: false,
						OccurredAt: time.Now().UTC(),
					}
				}
				normalized := s.redactor.RedactEvidence(normalizeEvidenceTiming([]domain.Evidence{item}))[0]
				resultCh <- normalized
			}()
		}

		for range validRequests {
			select {
			case <-runCtx.Done():
				return domain.InvestigationResult{}, fmt.Errorf("investigation deadline exceeded: %w", runCtx.Err())
			case item := <-resultCh:
				safeEvidence = s.boundEvidence(append(safeEvidence, item))
				itemCopy := item
				emit("evidence", "Received "+item.Source, &itemCopy, nil)
			}
		}
	}

	if err := runCtx.Err(); err != nil {
		return domain.InvestigationResult{}, fmt.Errorf("investigation deadline exceeded: %w", err)
	}
	emit("reasoning", "Producing final finding from bounded evidence", nil, nil)
	result, err := s.ai.Investigate(runCtx, domain.InvestigationRequest{
		Environment: env, Question: question, Evidence: safeEvidence,
	})
	if err != nil {
		return domain.InvestigationResult{}, fmt.Errorf("investigate: %w", err)
	}
	result.Evidence = safeEvidence
	emit("complete", "Investigation complete", nil, &result)
	return result, nil
}

func (s *InvestigationService) boundEvidence(items []domain.Evidence) []domain.Evidence {
	if len(items) == 0 {
		return []domain.Evidence{}
	}
	maxItems := s.limits.MaxEvidenceItems
	maxBytes := s.limits.MaxEvidenceBytes
	perItemMax := 8*1024 - len("\n[truncated]")
	if perItemMax > maxBytes {
		perItemMax = maxBytes
	}

	selectedReverse := make([]domain.Evidence, 0, maxItems)
	remaining := maxBytes
	for index := len(items) - 1; index >= 0 && len(selectedReverse) < maxItems && remaining > 0; index-- {
		item := items[index]
		if len(item.Output) > perItemMax {
			item.Output = item.Output[:perItemMax] + "\n[truncated]"
		}
		cost := len(item.Source) + len(item.Output)
		if cost > remaining {
			available := remaining - len(item.Source)
			if available <= 0 {
				continue
			}
			if len(item.Output) > available {
				item.Output = item.Output[:available]
			}
			cost = len(item.Source) + len(item.Output)
		}
		selectedReverse = append(selectedReverse, item)
		remaining -= cost
	}

	result := make([]domain.Evidence, len(selectedReverse))
	for index := range selectedReverse {
		result[len(selectedReverse)-1-index] = selectedReverse[index]
	}
	return result
}

func investigationEnvInt(name string, fallback, minimum, maximum int) int {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed < minimum || parsed > maximum {
		return fallback
	}
	return parsed
}

func availableInvestigationTools(env domain.Environment) []string {
	tools := []string{"system.info", "system.disk", "system.cpu", "system.memory"}
	caps := map[string]bool{}
	for _, capability := range env.Capabilities {
		caps[strings.ToLower(capability)] = true
	}
	if caps["docker"] {
		tools = append(tools, "docker.list", "docker.logs", "docker.inspect", "docker.stats")
	}
	if caps["systemd"] {
		tools = append(tools, "system.service_status", "system.journal")
	}
	if caps["git"] {
		tools = append(tools, "git.status", "git.log", "git.diff", "git.show_commit")
	}
	if caps["http"] {
		tools = append(tools, "http.health_check")
	}
	if caps["dns"] {
		tools = append(tools, "dns.lookup")
	}
	return tools
}

func validateInvestigationToolRequest(request domain.ToolRequest, available []string) error {
	allowed := false
	for _, tool := range available {
		if request.Tool == tool {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("tool %q is not available for this environment", request.Tool)
	}
	requiredArgument := map[string]string{
		"docker.logs": "container", "docker.inspect": "container", "docker.stats": "container",
		"system.service_status": "service", "system.journal": "service",
		"git.show_commit": "commit", "http.health_check": "url", "dns.lookup": "host",
	}[request.Tool]
	if requiredArgument != "" && strings.TrimSpace(request.Arguments[requiredArgument]) == "" {
		return fmt.Errorf("tool %q requires argument %q", request.Tool, requiredArgument)
	}
	if request.Tool == "docker.restart" || request.Tool == "system.service_restart" {
		return errors.New("mutation tools are not valid investigation tools")
	}
	return nil
}

func toolRequestKey(request domain.ToolRequest) string {
	keys := make([]string, 0, len(request.Arguments))
	for key := range request.Arguments {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := []string{request.Tool}
	for _, key := range keys {
		parts = append(parts, key+"="+request.Arguments[key])
	}
	return strings.Join(parts, "|")
}

func normalizeEvidenceTiming(items []domain.Evidence) []domain.Evidence {
	result := make([]domain.Evidence, len(items))
	for index, item := range items {
		if item.OccurredAt.IsZero() {
			item.OccurredAt = time.Now().UTC()
		}
		if item.DurationMS < 0 {
			item.DurationMS = 0
		}
		result[index] = item
	}
	return result
}
