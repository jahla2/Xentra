package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

const (
	maxAgentSteps           = 3
	maxToolRequestsPerStep  = 3
)

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
	redactor *EvidenceRedactor
}

func NewInvestigationService(repo EnvironmentRepository, tools ToolClient, ai AIClient) *InvestigationService {
	return &InvestigationService{repo: repo, tools: tools, ai: ai, redactor: NewEvidenceRedactor()}
}

func (s *InvestigationService) Investigate(ctx context.Context, organizationID, environmentID, question string) (domain.InvestigationResult, error) {
	return s.InvestigateWithEvidence(ctx, organizationID, environmentID, question, nil)
}

func (s *InvestigationService) InvestigateWithEvidence(ctx context.Context, organizationID, environmentID, question string, contextualEvidence []domain.Evidence) (domain.InvestigationResult, error) {
	env, err := s.repo.Get(ctx, organizationID, environmentID)
	if err != nil {
		return domain.InvestigationResult{}, err
	}
	evidence, err := s.tools.Collect(ctx, env)
	if err != nil {
		return domain.InvestigationResult{}, fmt.Errorf("collect evidence: %w", err)
	}
	evidence = append(evidence, contextualEvidence...)
	safeEvidence := s.redactor.RedactEvidence(normalizeEvidenceTiming(evidence))
	availableTools := availableInvestigationTools(env)
	seen := map[string]bool{}

	for step := 0; step < maxAgentSteps; step++ {
		decision, decisionErr := s.ai.Next(ctx, domain.AgentInvestigationRequest{
			Environment: env, Question: question, Evidence: safeEvidence,
			AvailableTools: availableTools, RemainingSteps: maxAgentSteps - step,
		})
		if decisionErr != nil {
			break
		}
		if decision.Mode == "complete" && decision.Finding != nil {
			result := *decision.Finding
			result.Evidence = safeEvidence
			return result, nil
		}
		if decision.Mode != "tools" || len(decision.ToolRequests) == 0 {
			break
		}

		requests := decision.ToolRequests
		if len(requests) > maxToolRequestsPerStep {
			requests = requests[:maxToolRequestsPerStep]
		}
		executedAny := false
		for _, request := range requests {
			key := toolRequestKey(request)
			if seen[key] {
				continue
			}
			seen[key] = true
			if err := validateInvestigationToolRequest(request, availableTools); err != nil {
				safeEvidence = append(safeEvidence, domain.Evidence{
					Source: "tool.request_rejected", Output: err.Error(), Success: false,
					OccurredAt: time.Now().UTC(),
				})
				continue
			}
			item, toolErr := s.tools.ExecuteReadTool(ctx, env, request)
			if toolErr != nil {
				item = domain.Evidence{Source: request.Tool, Output: toolErr.Error(), Success: false, OccurredAt: time.Now().UTC()}
			}
			safeEvidence = append(safeEvidence, s.redactor.RedactEvidence(normalizeEvidenceTiming([]domain.Evidence{item}))[0])
			executedAny = true
		}
		if !executedAny {
			break
		}
	}

	result, err := s.ai.Investigate(ctx, domain.InvestigationRequest{
		Environment: env, Question: question, Evidence: safeEvidence,
	})
	if err != nil {
		return domain.InvestigationResult{}, fmt.Errorf("investigate: %w", err)
	}
	result.Evidence = safeEvidence
	return result, nil
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
