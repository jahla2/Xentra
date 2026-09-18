package clients

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type ssmAPI interface {
	SendCommand(context.Context, *ssm.SendCommandInput, ...func(*ssm.Options)) (*ssm.SendCommandOutput, error)
	GetCommandInvocation(context.Context, *ssm.GetCommandInvocationInput, ...func(*ssm.Options)) (*ssm.GetCommandInvocationOutput, error)
}

type SSMClient struct {
	mu      sync.Mutex
	clients map[string]ssmAPI
	timeout time.Duration
	factory func(context.Context, string) (ssmAPI, error)
}

func NewSSMClient() *SSMClient {
	return &SSMClient{
		clients: map[string]ssmAPI{},
		timeout: 30 * time.Second,
		factory: func(ctx context.Context, region string) (ssmAPI, error) {
			cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(region))
			if err != nil {
				return nil, err
			}
			return ssm.NewFromConfig(cfg), nil
		},
	}
}

func (c *SSMClient) Discover(ctx context.Context, env domain.Environment) (domain.Discovery, error) {
	if err := validateSSMEnvironment(env); err != nil {
		return domain.Discovery{}, err
	}
	output, err := c.run(ctx, env, ssmDiscoveryScript)
	if err != nil {
		return domain.Discovery{}, err
	}
	sections := parseSSMSections(output)
	caps := remoteDiscoveryLines(sections["CAPABILITIES"], 20)
	containers := remoteDiscoveryLines(sections["CONTAINERS"], 100)
	cpu := strings.TrimSpace(sections["CPU"])
	if cpu != "" {
		cpu += " cores"
	}
	return domain.Discovery{
		OS: strings.ToLower(strings.TrimSpace(sections["OS"])),
		Hostname: strings.TrimSpace(sections["HOSTNAME"]),
		CPU: cpu,
		Memory: boundedRemoteDiscoveryOutput(sections["MEMORY"]),
		Disk: boundedRemoteDiscoveryOutput(sections["DISK"]),
		Containers: containers,
		Capabilities: caps,
	}, nil
}

func (c *SSMClient) Collect(ctx context.Context, env domain.Environment) ([]domain.Evidence, error) {
	requests := []domain.ToolRequest{
		{Tool: "system.info", Arguments: map[string]string{}},
		{Tool: "system.disk", Arguments: map[string]string{}},
		{Tool: "system.cpu", Arguments: map[string]string{}},
		{Tool: "system.memory", Arguments: map[string]string{}},
		{Tool: "docker.list", Arguments: map[string]string{}},
	}
	evidence := make([]domain.Evidence, 0, len(requests))
	for _, request := range requests {
		item, err := c.ExecuteReadTool(ctx, env, request)
		if err != nil {
			item = domain.Evidence{Source: request.Tool, Output: err.Error(), Success: false, OccurredAt: time.Now().UTC()}
		}
		evidence = append(evidence, item)
	}
	return evidence, nil
}

func (c *SSMClient) ExecuteReadTool(ctx context.Context, env domain.Environment, request domain.ToolRequest) (domain.Evidence, error) {
	startedAt := time.Now().UTC()
	command, err := sshReadToolCommand(request)
	if err != nil {
		return domain.Evidence{}, err
	}
	output, runErr := c.run(ctx, env, command)
	return domain.Evidence{
		Source: toolEvidenceSource(request),
		Output: outputOrError(output, runErr),
		Success: runErr == nil,
		OccurredAt: startedAt,
		DurationMS: time.Since(startedAt).Milliseconds(),
	}, nil
}

func (c *SSMClient) ExecuteAction(ctx context.Context, env domain.Environment, action, target string) (string, error) {
	if !safeMutationTarget(target) {
		return "", errors.New("unsafe remediation target")
	}
	var command string
	switch action {
	case "docker.restart":
		command = "docker restart " + shellQuote(target)
	case "system.service_restart":
		command = "systemctl restart " + shellQuote(target)
	default:
		return "", errors.New("action is not allowlisted")
	}
	return c.run(ctx, env, command)
}

func (c *SSMClient) VerifyAction(ctx context.Context, env domain.Environment, action, target string) (domain.VerificationResult, error) {
	if !safeMutationTarget(target) {
		return domain.VerificationResult{}, errors.New("unsafe verification target")
	}
	var source, command string
	switch action {
	case "docker.restart":
		source = "docker.status"
		command = "docker inspect -f '{{.State.Running}}' " + shellQuote(target)
	case "system.service_restart":
		source = "system.service_status"
		command = "systemctl is-active " + shellQuote(target)
	default:
		return domain.VerificationResult{}, errors.New("action is not verifiable")
	}

	output, runErr := c.run(ctx, env, command)
	normalized := strings.TrimSpace(strings.ToLower(output))
	statusHealthy := runErr == nil && (normalized == "true" || normalized == "active" || normalized == "running")
	evidence := []domain.Evidence{{
		Source: source, Output: outputOrError(output, runErr), Success: runErr == nil,
		OccurredAt: time.Now().UTC(),
	}}
	healthy := statusHealthy

	if env.HealthURL != "" {
		healthCommand, commandErr := sshReadToolCommand(domain.ToolRequest{
			Tool: "http.health_check",
			Arguments: map[string]string{"url": env.HealthURL},
		})
		if commandErr != nil {
			return domain.VerificationResult{}, commandErr
		}
		healthOutput, healthErr := c.run(ctx, env, healthCommand)
		healthSuccess := healthErr == nil
		healthy = statusHealthy && httpHealthHealthy(healthOutput, healthSuccess)
		evidence = append(evidence, domain.Evidence{
			Source: "http.health_check:" + env.HealthURL,
			Output: outputOrError(healthOutput, healthErr),
			Success: healthSuccess,
			OccurredAt: time.Now().UTC(),
		})
	}

	return domain.VerificationResult{
		Healthy: healthy,
		Summary: verificationSummaryForEnvironment(healthy, target, env.HealthURL),
		Evidence: evidence,
	}, nil
}

func (c *SSMClient) run(ctx context.Context, env domain.Environment, command string) (string, error) {
	if err := validateSSMEnvironment(env); err != nil {
		return "", err
	}
	client, err := c.client(ctx, env.AWSRegion)
	if err != nil {
		return "", fmt.Errorf("load AWS credentials/config: %w", err)
	}

	runCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	result, err := client.SendCommand(runCtx, &ssm.SendCommandInput{
		DocumentName: stringPtr("AWS-RunShellScript"),
		InstanceIds: []string{env.AWSInstanceID},
		Parameters: map[string][]string{"commands": {command}},
		TimeoutSeconds: int32Ptr(30),
	})
	if err != nil {
		return "", fmt.Errorf("ssm send command: %w", err)
	}
	if result.Command == nil || result.Command.CommandId == nil || *result.Command.CommandId == "" {
		return "", errors.New("ssm send command returned no command id")
	}

	commandID := *result.Command.CommandId
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-runCtx.Done():
			return "", fmt.Errorf("ssm command %s: %w", commandID, runCtx.Err())
		case <-ticker.C:
			invocation, getErr := client.GetCommandInvocation(runCtx, &ssm.GetCommandInvocationInput{
				CommandId: stringPtr(commandID),
				InstanceId: stringPtr(env.AWSInstanceID),
			})
			if getErr != nil {
				if strings.Contains(getErr.Error(), "InvocationDoesNotExist") {
					continue
				}
				return "", fmt.Errorf("ssm get command invocation: %w", getErr)
			}
			status := string(invocation.Status)
			switch status {
			case "Pending", "InProgress", "Delayed":
				continue
			case "Success":
				return strings.TrimSpace(stringValue(invocation.StandardOutputContent)), nil
			default:
				output := strings.TrimSpace(stringValue(invocation.StandardOutputContent))
				stderr := strings.TrimSpace(stringValue(invocation.StandardErrorContent))
				if output == "" {
					output = stderr
				} else if stderr != "" {
					output += "\n" + stderr
				}
				if output == "" {
					output = status
				}
				return output, fmt.Errorf("ssm command finished with status %s", status)
			}
		}
	}
}

func (c *SSMClient) client(ctx context.Context, region string) (ssmAPI, error) {
	c.mu.Lock()
	if existing := c.clients[region]; existing != nil {
		c.mu.Unlock()
		return existing, nil
	}
	c.mu.Unlock()

	created, err := c.factory(ctx, region)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if existing := c.clients[region]; existing != nil {
		return existing, nil
	}
	c.clients[region] = created
	return created, nil
}

var ssmRegionPattern = regexp.MustCompile(`^[a-z]{2}(?:-[a-z0-9]+)+-[0-9]+$`)
var ssmInstanceIDPattern = regexp.MustCompile(`^i-[0-9A-Fa-f]{8,32}$`)

func validateSSMEnvironment(env domain.Environment) error {
	if !ssmRegionPattern.MatchString(strings.TrimSpace(env.AWSRegion)) {
		return errors.New("valid AWS region is required for aws_ssm")
	}
	if !ssmInstanceIDPattern.MatchString(strings.TrimSpace(env.AWSInstanceID)) {
		return errors.New("valid EC2 instance id is required for aws_ssm")
	}
	return nil
}

func stringPtr(value string) *string { return &value }
func int32Ptr(value int32) *int32 { return &value }
func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

const ssmDiscoveryScript = `set +e
printf '__XENTRA_OS__\n'
uname -s
printf '__XENTRA_HOSTNAME__\n'
hostname
printf '__XENTRA_CPU__\n'
nproc
printf '__XENTRA_MEMORY__\n'
free -m
printf '__XENTRA_DISK__\n'
df -h /
printf '__XENTRA_CAPABILITIES__\n'
command -v docker >/dev/null 2>&1 && echo docker
command -v systemctl >/dev/null 2>&1 && echo systemd
command -v nginx >/dev/null 2>&1 && echo nginx
command -v git >/dev/null 2>&1 && echo git
command -v curl >/dev/null 2>&1 && echo http
command -v getent >/dev/null 2>&1 && echo dns
printf '__XENTRA_CONTAINERS__\n'
if command -v docker >/dev/null 2>&1; then
  docker ps --format '{{.Names}}\t{{.Status}}'
fi
`

func parseSSMSections(output string) map[string]string {
	result := map[string]string{}
	current := ""
	var lines []string
	flush := func() {
		if current != "" {
			result[current] = strings.TrimSpace(strings.Join(lines, "\n"))
		}
		lines = nil
	}
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "__XENTRA_") && strings.HasSuffix(line, "__") {
			flush()
			current = strings.TrimSuffix(strings.TrimPrefix(line, "__XENTRA_"), "__")
			continue
		}
		if current != "" {
			lines = append(lines, line)
		}
	}
	flush()
	return result
}
