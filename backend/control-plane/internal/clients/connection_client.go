package clients

import (
	"context"
	"errors"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type ConnectionClient struct {
	runner   *RunnerClient
	ssh      *SSHClient
	ssm      *SSMClient
	outbound *OutboundRunnerClient
}

func NewConnectionClient(runner *RunnerClient, ssh *SSHClient, ssm *SSMClient, outbound *OutboundRunnerClient) *ConnectionClient {
	return &ConnectionClient{runner: runner, ssh: ssh, ssm: ssm, outbound: outbound}
}

func (c *ConnectionClient) Discover(ctx context.Context, env domain.Environment) (domain.Discovery, error) {
	switch env.ConnectionType {
	case "", "runner":
		return c.runner.Discover(ctx, env)
	case "runner_outbound":
		return domain.Discovery{OS: env.OS, Hostname: env.Hostname, Capabilities: env.Capabilities}, nil
	case "aws_ssm":
		return c.ssm.Discover(ctx, env)
	case "ssh":
		return c.ssh.Discover(ctx, env)
	default:
		return domain.Discovery{}, errors.New("unsupported connection type")
	}
}

func (c *ConnectionClient) Collect(ctx context.Context, env domain.Environment) ([]domain.Evidence, error) {
	switch env.ConnectionType {
	case "", "runner":
		return c.runner.Collect(ctx, env)
	case "runner_outbound":
		if c.outbound == nil {
			return nil, errors.New("outbound runner client unavailable")
		}
		return c.outbound.Collect(ctx, env)
	case "aws_ssm":
		return c.ssm.Collect(ctx, env)
	case "ssh":
		return c.ssh.Collect(ctx, env)
	default:
		return nil, errors.New("unsupported connection type")
	}
}

func (c *ConnectionClient) ExecuteReadTool(ctx context.Context, env domain.Environment, request domain.ToolRequest) (domain.Evidence, error) {
	switch env.ConnectionType {
	case "", "runner":
		return c.runner.ExecuteReadTool(ctx, env, request)
	case "runner_outbound":
		if c.outbound == nil {
			return domain.Evidence{}, errors.New("outbound runner client unavailable")
		}
		return c.outbound.ExecuteReadTool(ctx, env, request)
	case "aws_ssm":
		return c.ssm.ExecuteReadTool(ctx, env, request)
	case "ssh":
		return c.ssh.ExecuteReadTool(ctx, env, request)
	default:
		return domain.Evidence{}, errors.New("unsupported connection type")
	}
}

func (c *ConnectionClient) ExecuteAction(ctx context.Context, env domain.Environment, action, target string) (string, error) {
	switch env.ConnectionType {
	case "", "runner":
		return c.runner.ExecuteAction(ctx, env, action, target)
	case "runner_outbound":
		if c.outbound == nil {
			return "", errors.New("outbound runner client unavailable")
		}
		return c.outbound.ExecuteAction(ctx, env, action, target)
	case "aws_ssm":
		return c.ssm.ExecuteAction(ctx, env, action, target)
	case "ssh":
		return c.ssh.ExecuteAction(ctx, env, action, target)
	default:
		return "", errors.New("unsupported connection type")
	}
}

func (c *ConnectionClient) VerifyAction(ctx context.Context, env domain.Environment, action, target string) (domain.VerificationResult, error) {
	switch env.ConnectionType {
	case "", "runner":
		return c.runner.VerifyAction(ctx, env, action, target)
	case "runner_outbound":
		if c.outbound == nil {
			return domain.VerificationResult{}, errors.New("outbound runner client unavailable")
		}
		return c.outbound.VerifyAction(ctx, env, action, target)
	case "aws_ssm":
		return c.ssm.VerifyAction(ctx, env, action, target)
	case "ssh":
		return c.ssh.VerifyAction(ctx, env, action, target)
	default:
		return domain.VerificationResult{}, errors.New("unsupported connection type")
	}
}
