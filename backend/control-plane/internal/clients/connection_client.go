package clients

import (
	"context"
	"errors"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type ConnectionClient struct {
	runner *RunnerClient
	ssh    *SSHClient
}

func NewConnectionClient(runner *RunnerClient, ssh *SSHClient) *ConnectionClient {
	return &ConnectionClient{runner: runner, ssh: ssh}
}

func (c *ConnectionClient) Discover(ctx context.Context, env domain.Environment) (domain.Discovery, error) {
	switch env.ConnectionType {
	case "", "runner":
		return c.runner.Discover(ctx, env)
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
	case "ssh":
		return c.ssh.Collect(ctx, env)
	default:
		return nil, errors.New("unsupported connection type")
	}
}

func (c *ConnectionClient) ExecuteAction(ctx context.Context, env domain.Environment, action, target string) (string, error) {
	switch env.ConnectionType {
	case "", "runner":
		return c.runner.ExecuteAction(ctx, env, action, target)
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
	case "ssh":
		return c.ssh.VerifyAction(ctx, env, action, target)
	default:
		return domain.VerificationResult{}, errors.New("unsupported connection type")
	}
}
