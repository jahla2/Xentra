package clients

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type OutboundRunnerDispatcher interface {
	Dispatch(context.Context, domain.Environment, domain.ToolRequest) (domain.RunnerTaskResult, error)
}

type OutboundRunnerClient struct {
	dispatcher OutboundRunnerDispatcher
}

func NewOutboundRunnerClient(dispatcher OutboundRunnerDispatcher) *OutboundRunnerClient {
	return &OutboundRunnerClient{dispatcher: dispatcher}
}

func (c *OutboundRunnerClient) Collect(ctx context.Context, env domain.Environment) ([]domain.Evidence, error) {
	requests := []domain.ToolRequest{
		{Tool: "system.info", Arguments: map[string]string{}},
		{Tool: "system.disk", Arguments: map[string]string{}},
		{Tool: "system.cpu", Arguments: map[string]string{}},
		{Tool: "system.memory", Arguments: map[string]string{}},
		{Tool: "docker.list", Arguments: map[string]string{}},
	}
	return collectEvidenceParallel(ctx, requests, func(ctx context.Context, request domain.ToolRequest) (domain.Evidence, error) {
		return c.ExecuteReadTool(ctx, env, request)
	}), nil
}

func (c *OutboundRunnerClient) ExecuteReadTool(ctx context.Context, env domain.Environment, request domain.ToolRequest) (domain.Evidence, error) {
	if !runnerReadTools[request.Tool] {
		return domain.Evidence{}, errors.New("tool is not an allowlisted read-only Runner tool")
	}
	started := time.Now().UTC()
	result, err := c.dispatcher.Dispatch(ctx, env, request)
	if err != nil {
		return domain.Evidence{}, err
	}
	output := result.Output
	if output == "" {
		output = result.Error
	}
	return domain.Evidence{
		Source: toolEvidenceSource(request), Output: output, Success: result.Success,
		OccurredAt: started, DurationMS: time.Since(started).Milliseconds(),
	}, nil
}

func (c *OutboundRunnerClient) ExecuteAction(ctx context.Context, env domain.Environment, action, target string) (string, error) {
	args, err := actionArguments(action, target)
	if err != nil {
		return "", err
	}
	result, err := c.dispatcher.Dispatch(ctx, env, domain.ToolRequest{Tool: action, Arguments: args})
	if err != nil {
		return "", err
	}
	if !result.Success {
		if result.Error != "" {
			return result.Output, errors.New(result.Error)
		}
		return result.Output, errors.New(result.Output)
	}
	return result.Output, nil
}

func (c *OutboundRunnerClient) VerifyAction(ctx context.Context, env domain.Environment, action, target string) (domain.VerificationResult, error) {
	var request domain.ToolRequest
	switch action {
	case "docker.restart":
		request = domain.ToolRequest{Tool: "docker.status", Arguments: map[string]string{"container": target}}
	case "system.service_restart":
		request = domain.ToolRequest{Tool: "system.service_status", Arguments: map[string]string{"service": target}}
	default:
		return domain.VerificationResult{}, errors.New("action is not verifiable")
	}
	item, err := c.ExecuteReadTool(ctx, env, request)
	if err != nil {
		return domain.VerificationResult{}, err
	}
	normalized := strings.TrimSpace(strings.ToLower(item.Output))
	statusHealthy := item.Success && (normalized == "true" || normalized == "active" || normalized == "running")
	evidence := []domain.Evidence{item}
	healthy := statusHealthy
	if env.HealthURL != "" {
		healthItem, healthErr := c.ExecuteReadTool(ctx, env, domain.ToolRequest{Tool: "http.health_check", Arguments: map[string]string{"url": env.HealthURL}})
		if healthErr != nil {
			return domain.VerificationResult{}, healthErr
		}
		healthy = statusHealthy && httpHealthHealthy(healthItem.Output, healthItem.Success)
		evidence = append(evidence, healthItem)
	}
	return domain.VerificationResult{
		Healthy: healthy, Summary: verificationSummaryForEnvironment(healthy, target, env.HealthURL),
		Evidence: evidence,
	}, nil
}
