package clients

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
	"golang.org/x/crypto/ssh"
)

type SSHCredentialProvider interface {
	GetSSH(context.Context, string) (domain.SSHCredential, error)
}

type SSHClient struct {
	credentials SSHCredentialProvider
	timeout     time.Duration
}

func NewSSHClient(credentials SSHCredentialProvider) *SSHClient {
	return &SSHClient{credentials: credentials, timeout: 8 * time.Second}
}

func (c *SSHClient) Discover(ctx context.Context, env domain.Environment) (domain.Discovery, error) {
	client, err := c.connect(ctx, env)
	if err != nil { return domain.Discovery{}, err }
	defer client.Close()

	osName, err := runSSH(client, "uname -s")
	if err != nil { return domain.Discovery{}, err }
	hostname, err := runSSH(client, "hostname")
	if err != nil { return domain.Discovery{}, err }
	cpu, _ := runSSH(client, "nproc")
	memory, _ := runSSH(client, "free -m")
	disk, _ := runSSH(client, "df -h /")

	caps := []string{}
	for name, command := range map[string]string{
		"docker": "command -v docker",
		"systemd": "command -v systemctl",
		"nginx": "command -v nginx",
		"git": "command -v git",
		"http": "command -v curl",
		"dns": "command -v getent",
	} {
		if _, err := runSSH(client, command); err == nil { caps = append(caps, name) }
	}

	containers := []string{}
	if containsCapability(caps, "docker") {
		if output, listErr := runSSH(client, "docker ps --format '{{.Names}}\\t{{.Status}}'"); listErr == nil {
			containers = remoteDiscoveryLines(output, 100)
		}
	}
	cpuSummary := strings.TrimSpace(cpu)
	if cpuSummary != "" {
		cpuSummary += " cores"
	}
	return domain.Discovery{
		OS: strings.TrimSpace(strings.ToLower(osName)),
		Hostname: strings.TrimSpace(hostname),
		CPU: cpuSummary,
		Memory: boundedRemoteDiscoveryOutput(memory),
		Disk: boundedRemoteDiscoveryOutput(disk),
		Containers: containers,
		Capabilities: caps,
	}, nil
}

func boundedRemoteDiscoveryOutput(value string) string {
	value = strings.TrimSpace(value)
	const limit = 2048
	if len(value) > limit {
		return value[:limit] + "…"
	}
	return value
}

func remoteDiscoveryLines(value string, limit int) []string {
	lines := strings.Split(strings.TrimSpace(value), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		result = append(result, line)
		if len(result) >= limit {
			break
		}
	}
	return result
}

func containsCapability(items []string, expected string) bool {
	for _, item := range items {
		if item == expected {
			return true
		}
	}
	return false
}

func (c *SSHClient) Collect(ctx context.Context, env domain.Environment) ([]domain.Evidence, error) {
	client, err := c.connect(ctx, env)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	requests := []domain.ToolRequest{
		{Tool: "system.info", Arguments: map[string]string{}},
		{Tool: "system.disk", Arguments: map[string]string{}},
		{Tool: "system.cpu", Arguments: map[string]string{}},
		{Tool: "system.memory", Arguments: map[string]string{}},
		{Tool: "docker.list", Arguments: map[string]string{}},
	}
	evidence := make([]domain.Evidence, 0, len(requests))
	for _, request := range requests {
		startedAt := time.Now().UTC()
		command, commandErr := sshReadToolCommand(request)
		if commandErr != nil {
			evidence = append(evidence, domain.Evidence{
				Source: request.Tool, Success: false, Output: commandErr.Error(),
				OccurredAt: startedAt, DurationMS: time.Since(startedAt).Milliseconds(),
			})
			continue
		}
		output, runErr := runSSH(client, command)
		evidence = append(evidence, domain.Evidence{
			Source: toolEvidenceSource(request), Success: runErr == nil, Output: outputOrError(output, runErr),
			OccurredAt: startedAt, DurationMS: time.Since(startedAt).Milliseconds(),
		})
	}
	return evidence, nil
}

func (c *SSHClient) ExecuteReadTool(ctx context.Context, env domain.Environment, request domain.ToolRequest) (domain.Evidence, error) {
	startedAt := time.Now().UTC()
	command, err := sshReadToolCommand(request)
	if err != nil {
		return domain.Evidence{}, err
	}
	client, err := c.connect(ctx, env)
	if err != nil {
		return domain.Evidence{}, err
	}
	defer client.Close()
	output, runErr := runSSH(client, command)
	return domain.Evidence{
		Source: toolEvidenceSource(request), Success: runErr == nil, Output: outputOrError(output, runErr),
		OccurredAt: startedAt, DurationMS: time.Since(startedAt).Milliseconds(),
	}, nil
}

func sshReadToolCommand(request domain.ToolRequest) (string, error) {
	switch request.Tool {
	case "system.info":
		return "uname -a", nil
	case "system.disk":
		return "df -h", nil
	case "system.cpu":
		return "lscpu", nil
	case "system.memory":
		return "free -m", nil
	case "docker.list":
		return "docker ps -a --format '{{.Names}}\\t{{.Status}}\\t{{.Image}}'", nil
	case "docker.logs":
		target := request.Arguments["container"]
		if !safeMutationTarget(target) { return "", errors.New("valid container is required") }
		return "docker logs --tail 200 " + shellQuote(target), nil
	case "docker.inspect":
		target := request.Arguments["container"]
		if !safeMutationTarget(target) { return "", errors.New("valid container is required") }
		return "docker inspect " + shellQuote(target), nil
	case "docker.stats":
		target := request.Arguments["container"]
		if !safeMutationTarget(target) { return "", errors.New("valid container is required") }
		return "docker stats --no-stream --format '{{.Name}}\\t{{.CPUPerc}}\\t{{.MemUsage}}\\t{{.NetIO}}\\t{{.BlockIO}}' " + shellQuote(target), nil
	case "system.service_status":
		target := request.Arguments["service"]
		if !safeMutationTarget(target) { return "", errors.New("valid service is required") }
		return "systemctl is-active " + shellQuote(target), nil
	case "system.journal":
		target := request.Arguments["service"]
		if !safeMutationTarget(target) { return "", errors.New("valid service is required") }
		return "journalctl -u " + shellQuote(target) + " -n 200 --no-pager", nil
	case "git.status":
		repoPath, ok := safeSSHRepoPath(request.Arguments["path"])
		if !ok { return "", errors.New("valid repository path is required") }
		return "git -C " + shellQuote(repoPath) + " status --short --branch", nil
	case "git.log":
		repoPath, ok := safeSSHRepoPath(request.Arguments["path"])
		if !ok { return "", errors.New("valid repository path is required") }
		return "git -C " + shellQuote(repoPath) + " log -n 10 --oneline --decorate", nil
	case "git.diff":
		repoPath, ok := safeSSHRepoPath(request.Arguments["path"])
		if !ok { return "", errors.New("valid repository path is required") }
		return "git -C " + shellQuote(repoPath) + " diff --stat", nil
	case "git.show_commit":
		repoPath, ok := safeSSHRepoPath(request.Arguments["path"])
		if !ok { return "", errors.New("valid repository path is required") }
		commit := strings.TrimSpace(request.Arguments["commit"])
		if !safeCommitSHA.MatchString(commit) { return "", errors.New("valid commit SHA is required") }
		return "git -C " + shellQuote(repoPath) + " show --stat --oneline --decorate --no-renames " + commit, nil
	case "http.health_check":
		targetURL := strings.TrimSpace(request.Arguments["url"])
		if !safeSSHHTTPURL(targetURL) { return "", errors.New("valid http/https URL is required") }
		return "curl --fail --silent --show-error --location --max-time 8 -o /dev/null -w '%{http_code}' " + shellQuote(targetURL), nil
	case "dns.lookup":
		host := strings.TrimSpace(request.Arguments["host"])
		if !safeMutationTarget(host) { return "", errors.New("valid hostname is required") }
		return "getent hosts " + shellQuote(host), nil
	default:
		return "", errors.New("tool is not an allowlisted read-only SSH tool")
	}
}

func (c *SSHClient) ExecuteAction(ctx context.Context, env domain.Environment, action, target string) (string, error) {
	if !safeMutationTarget(target) { return "", errors.New("unsafe remediation target") }
	client, err := c.connect(ctx, env)
	if err != nil { return "", err }
	defer client.Close()

	var command string
	switch action {
	case "docker.restart":
		command = "docker restart " + target
	case "system.service_restart":
		command = "systemctl restart " + target
	default:
		return "", errors.New("action is not allowlisted")
	}
	return runSSH(client, command)
}

func (c *SSHClient) VerifyAction(ctx context.Context, env domain.Environment, action, target string) (domain.VerificationResult, error) {
	if !safeMutationTarget(target) { return domain.VerificationResult{}, errors.New("unsafe verification target") }
	client, err := c.connect(ctx, env)
	if err != nil { return domain.VerificationResult{}, err }
	defer client.Close()

	var source, command string
	switch action {
	case "docker.restart":
		source = "docker.status"
		command = "docker inspect -f '{{.State.Running}}' " + target
	case "system.service_restart":
		source = "system.service_status"
		command = "systemctl is-active " + target
	default:
		return domain.VerificationResult{}, errors.New("action is not verifiable")
	}

	output, runErr := runSSH(client, command)
	normalized := strings.TrimSpace(strings.ToLower(output))
	healthy := runErr == nil && (normalized == "true" || normalized == "active" || normalized == "running")
	return domain.VerificationResult{
		Healthy: healthy,
		Summary: verificationSummary(healthy, target),
		Evidence: []domain.Evidence{{Source: source, Output: outputOrError(output, runErr), Success: runErr == nil}},
	}, nil
}

func (c *SSHClient) connect(ctx context.Context, env domain.Environment) (*ssh.Client, error) {
	if c.credentials == nil { return nil, errors.New("ssh credential provider unavailable") }
	credential, err := c.credentials.GetSSH(ctx, env.CredentialID)
	if err != nil { return nil, fmt.Errorf("load ssh credential: %w", err) }
	signer, err := parseSigner(credential)
	if err != nil { return nil, err }

	port := env.SSHPort
	if port == 0 { port = 22 }
	config := &ssh.ClientConfig{
		User: env.SSHUser,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
		HostKeyCallback: func(_ string, _ net.Addr, key ssh.PublicKey) error {
			actual := ssh.FingerprintSHA256(key)
			if actual != env.SSHHostKeyFingerprint { return fmt.Errorf("ssh host key mismatch: got %s", actual) }
			return nil
		},
		Timeout: c.timeout,
	}
	address := net.JoinHostPort(env.SSHHost, strconv.Itoa(port))
	dialer := net.Dialer{Timeout: c.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", address)
	if err != nil { return nil, err }
	cc, channels, requests, err := ssh.NewClientConn(conn, address, config)
	if err != nil {
		conn.Close()
		return nil, err
	}
	return ssh.NewClient(cc, channels, requests), nil
}

func parseSigner(credential domain.SSHCredential) (ssh.Signer, error) {
	key := []byte(credential.PrivateKey)
	if credential.Passphrase != "" { return ssh.ParsePrivateKeyWithPassphrase(key, []byte(credential.Passphrase)) }
	return ssh.ParsePrivateKey(key)
}

func runSSH(client *ssh.Client, command string) (string, error) {
	session, err := client.NewSession()
	if err != nil { return "", err }
	defer session.Close()
	output, err := session.CombinedOutput(command)
	return string(output), err
}

func outputOrError(output string, err error) string {
	if output != "" { return output }
	if err != nil { return err.Error() }
	return ""
}

var safeCommitSHA = regexp.MustCompile(`^[A-Fa-f0-9]{7,64}$`)

func safeSSHRepoPath(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "."
	}
	if strings.ContainsAny(value, "\x00\r\n") {
		return "", false
	}
	return value, true
}

func safeSSHHTTPURL(value string) bool {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return false
	}
	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

var mutationTargetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@:-]*$`)

func safeMutationTarget(name string) bool {
	return name != "" && mutationTargetPattern.MatchString(name)
}
