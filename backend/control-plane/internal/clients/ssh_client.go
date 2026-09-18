package clients

import (
	"context"
	"errors"
	"fmt"
	"net"
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

	caps := []string{}
	for name, command := range map[string]string{
		"docker": "command -v docker",
		"systemd": "command -v systemctl",
		"nginx": "command -v nginx",
		"git": "command -v git",
	} {
		if _, err := runSSH(client, command); err == nil { caps = append(caps, name) }
	}
	return domain.Discovery{OS: strings.TrimSpace(strings.ToLower(osName)), Hostname: strings.TrimSpace(hostname), Capabilities: caps}, nil
}

func (c *SSHClient) Collect(ctx context.Context, env domain.Environment) ([]domain.Evidence, error) {
	client, err := c.connect(ctx, env)
	if err != nil { return nil, err }
	defer client.Close()

	evidence := []domain.Evidence{}
	for source, command := range map[string]string{
		"system.info": "uname -a",
		"system.disk": "df -h",
		"docker.list": "docker ps -a --format '{{.Names}}\t{{.Status}}\t{{.Image}}'",
	} {
		output, runErr := runSSH(client, command)
		evidence = append(evidence, domain.Evidence{Source: source, Success: runErr == nil, Output: outputOrError(output, runErr)})
	}

	names, err := runSSH(client, "docker ps --format '{{.Names}}'")
	if err == nil {
		for index, name := range strings.Fields(names) {
			if index >= 5 { break }
			if !safeMutationTarget(name) { continue }
			output, logErr := runSSH(client, "docker logs --tail 100 "+name)
			evidence = append(evidence, domain.Evidence{Source: "docker.logs:" + name, Success: logErr == nil, Output: outputOrError(output, logErr)})
		}
	}
	return evidence, nil
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

var mutationTargetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@:-]*$`)

func safeMutationTarget(name string) bool {
	return name != "" && mutationTargetPattern.MatchString(name)
}
