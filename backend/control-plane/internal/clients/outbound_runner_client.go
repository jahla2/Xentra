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
	tools := []string{"system.info", "system.disk", "system.cpu", "system.memory", "docker.list"}
	result := make([]domain.Evidence, 0, len(tools))
	for _, tool := range tools {
		item, err := c.ExecuteReadTool(ctx, env, domain.ToolRequest{Tool: tool, Arguments: map[string]string{}})
		if err != nil {
			item = domain.Evidence{Source: tool, Output: err.Error(), Success: false, OccurredAt: time.Now().UTC()}
		}
		result = append(result, item)
	}
	return result, nil
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
	healthy := item.Success && (normalized == "true" || normalized == "active" || normalized == "running")
	return domain.VerificationResult{
		Healthy: healthy, Summary: verificationSummary(healthy, target),
		Evidence: []domain.Evidence{item},
	}, nil
}
