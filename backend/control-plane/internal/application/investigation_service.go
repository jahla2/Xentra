package application

import (
	"context"
	"fmt"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type ToolClient interface {
	Collect(context.Context, domain.Environment) ([]domain.Evidence, error)
}

type AIClient interface {
	Investigate(context.Context, domain.InvestigationRequest) (domain.InvestigationResult, error)
}

type InvestigationService struct {
	repo  EnvironmentRepository
	tools ToolClient
	ai    AIClient
}

func NewInvestigationService(repo EnvironmentRepository, tools ToolClient, ai AIClient) *InvestigationService {
	return &InvestigationService{repo: repo, tools: tools, ai: ai}
}

func (s *InvestigationService) Investigate(ctx context.Context, organizationID, environmentID, question string) (domain.InvestigationResult, error) {
	env, err := s.repo.Get(ctx, organizationID, environmentID)
	if err != nil {
		return domain.InvestigationResult{}, err
	}
	evidence, err := s.tools.Collect(ctx, env)
	if err != nil {
		return domain.InvestigationResult{}, fmt.Errorf("collect evidence: %w", err)
	}
	result, err := s.ai.Investigate(ctx, domain.InvestigationRequest{Environment: env, Question: question, Evidence: evidence})
	if err != nil {
		return domain.InvestigationResult{}, fmt.Errorf("investigate: %w", err)
	}
	result.Evidence = evidence
	return result, nil
}
