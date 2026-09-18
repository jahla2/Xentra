package clients

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type RunnerClient struct {
	http       *http.Client
	requireTLS bool
}

func NewRunnerClient() *RunnerClient {
	return &RunnerClient{http: &http.Client{Timeout: 8 * time.Second}}
}

func NewRunnerClientFromEnv() (*RunnerClient, error) {
	if envFlag("XENTRA_RUNNER_INSECURE_DEV") {
		return NewRunnerClient(), nil
	}

	certFile := os.Getenv("XENTRA_RUNNER_CLIENT_CERT")
	keyFile := os.Getenv("XENTRA_RUNNER_CLIENT_KEY")
	caFile := os.Getenv("XENTRA_RUNNER_SERVER_CA")
	if certFile == "" || keyFile == "" || caFile == "" {
		return nil, errors.New("XENTRA_RUNNER_CLIENT_CERT, XENTRA_RUNNER_CLIENT_KEY and XENTRA_RUNNER_SERVER_CA are required unless XENTRA_RUNNER_INSECURE_DEV=true")
	}

	certificate, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("load runner client certificate: %w", err)
	}
	caPEM, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("read runner server CA: %w", err)
	}
	rootCAs := x509.NewCertPool()
	if !rootCAs.AppendCertsFromPEM(caPEM) {
		return nil, errors.New("runner server CA contains no valid certificates")
	}

	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			MinVersion:   tls.VersionTLS13,
			Certificates: []tls.Certificate{certificate},
			RootCAs:      rootCAs,
		},
	}
	return &RunnerClient{
		http:       &http.Client{Timeout: 8 * time.Second, Transport: transport},
		requireTLS: true,
	}, nil
}

func NewRunnerClientWithHTTPClient(client *http.Client, requireTLS bool) *RunnerClient {
	return &RunnerClient{http: client, requireTLS: requireTLS}
}

func (c *RunnerClient) Discover(ctx context.Context, env domain.Environment) (domain.Discovery, error) {
	endpoint, err := c.endpoint(env.RunnerURL, "/v1/discovery")
	if err != nil {
		return domain.Discovery{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.Discovery{}, err
	}
	res, err := c.http.Do(req)
	if err != nil {
		return domain.Discovery{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return domain.Discovery{}, fmt.Errorf("runner discovery status %d", res.StatusCode)
	}
	var result domain.Discovery
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return domain.Discovery{}, err
	}
	return result, nil
}

func (c *RunnerClient) Collect(ctx context.Context, env domain.Environment) ([]domain.Evidence, error) {
	tools := []string{"system.info", "system.disk", "docker.list"}
	evidence := make([]domain.Evidence, 0, len(tools))
	for _, tool := range tools {
		result, err := c.executeTool(ctx, env, tool, map[string]string{})
		if err != nil {
			evidence = append(evidence, domain.Evidence{Source: tool, Success: false, Output: err.Error()})
			continue
		}
		evidence = append(evidence, domain.Evidence{Source: tool, Success: result.Success, Output: result.Output})
	}
	return evidence, nil
}

func (c *RunnerClient) ExecuteAction(ctx context.Context, env domain.Environment, action, target string) (string, error) {
	args, err := actionArguments(action, target)
	if err != nil {
		return "", err
	}
	result, err := c.executeTool(ctx, env, action, args)
	if err != nil {
		return "", err
	}
	if !result.Success {
		return result.Output, errors.New(result.Output)
	}
	return result.Output, nil
}

func (c *RunnerClient) VerifyAction(ctx context.Context, env domain.Environment, action, target string) (domain.VerificationResult, error) {
	var tool string
	var args map[string]string
	switch action {
	case "docker.restart":
		tool, args = "docker.status", map[string]string{"container": target}
	case "system.service_restart":
		tool, args = "system.service_status", map[string]string{"service": target}
	default:
		return domain.VerificationResult{}, errors.New("action is not verifiable")
	}
	result, err := c.executeTool(ctx, env, tool, args)
	if err != nil {
		return domain.VerificationResult{}, err
	}
	output := strings.TrimSpace(strings.ToLower(result.Output))
	healthy := result.Success && (output == "true" || output == "active" || output == "running")
	return domain.VerificationResult{
		Healthy: healthy,
		Summary: verificationSummary(healthy, target),
		Evidence: []domain.Evidence{{Source: tool, Output: result.Output, Success: result.Success}},
	}, nil
}

type runnerToolResult struct {
	Tool    string `json:"tool"`
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error"`
}

func (c *RunnerClient) executeTool(ctx context.Context, env domain.Environment, tool string, args map[string]string) (runnerToolResult, error) {
	endpoint, err := c.endpoint(env.RunnerURL, "/v1/tools/execute")
	if err != nil {
		return runnerToolResult{}, err
	}
	payload, _ := json.Marshal(map[string]any{"tool": tool, "arguments": args})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return runnerToolResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return runnerToolResult{}, err
	}
	defer res.Body.Close()
	var result runnerToolResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil {
		return runnerToolResult{}, err
	}
	if result.Output == "" {
		result.Output = result.Error
	}
	if res.StatusCode >= 500 {
		return result, fmt.Errorf("runner status %d: %s", res.StatusCode, result.Output)
	}
	return result, nil
}

func (c *RunnerClient) endpoint(baseURL, path string) (string, error) {
	parsed, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", fmt.Errorf("invalid runner URL: %w", err)
	}
	if c.requireTLS && parsed.Scheme != "https" {
		return "", errors.New("runner URL must use https when mutual TLS is enabled")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("runner URL must use http or https")
	}
	return parsed.String() + path, nil
}

func actionArguments(action, target string) (map[string]string, error) {
	switch action {
	case "docker.restart":
		return map[string]string{"container": target}, nil
	case "system.service_restart":
		return map[string]string{"service": target}, nil
	default:
		return nil, errors.New("action is not allowlisted")
	}
}

func verificationSummary(healthy bool, target string) string {
	if healthy {
		return target + " is healthy after remediation"
	}
	return target + " did not pass post-action verification"
}

func envFlag(name string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(name))) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
