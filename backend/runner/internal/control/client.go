package control

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

	"github.com/jahla2/Xentra/backend/runner/internal/domain"
)

type Config struct {
	BaseURL     string
	RunnerID    string
	RunnerToken string
	InsecureDev bool
	CertFile    string
	KeyFile     string
	CAFile      string
	PollEvery   time.Duration
}

type Task struct {
	ID        string            `json:"id"`
	RunnerID  string            `json:"runnerId"`
	Tool      string            `json:"tool"`
	Arguments map[string]string `json:"arguments"`
}

type Client struct {
	baseURL     string
	runnerID    string
	runnerToken string
	http        *http.Client
}

func NewClient(config Config) (*Client, error) {
	if strings.TrimSpace(config.BaseURL) == "" || config.RunnerID == "" || config.RunnerToken == "" {
		return nil, errors.New("control URL, Runner ID and Runner token are required")
	}
	parsed, err := url.Parse(strings.TrimRight(config.BaseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse control URL: %w", err)
	}

	transport := &http.Transport{}
	if config.InsecureDev {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return nil, errors.New("development control URL must use http or https")
		}
	} else {
		if parsed.Scheme != "https" {
			return nil, errors.New("production outbound Runner control URL must use https")
		}
		if config.CertFile == "" || config.KeyFile == "" || config.CAFile == "" {
			return nil, errors.New("XENTRA_CONTROL_CLIENT_CERT, XENTRA_CONTROL_CLIENT_KEY and XENTRA_CONTROL_SERVER_CA are required")
		}
		certificate, err := tls.LoadX509KeyPair(config.CertFile, config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("load control client certificate: %w", err)
		}
		caPEM, err := os.ReadFile(config.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read control server CA: %w", err)
		}
		rootCAs := x509.NewCertPool()
		if !rootCAs.AppendCertsFromPEM(caPEM) {
			return nil, errors.New("control server CA contains no valid certificates")
		}
		transport.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}, RootCAs: rootCAs,
		}
	}

	return &Client{
		baseURL: parsed.String(), runnerID: config.RunnerID, runnerToken: config.RunnerToken,
		http: &http.Client{Timeout: 30 * time.Second, Transport: transport},
	}, nil
}

func (c *Client) Poll(ctx context.Context, discovery domain.Discovery) (*Task, error) {
	payload, err := json.Marshal(discovery)
	if err != nil {
		return nil, err
	}
	endpoint := fmt.Sprintf("%s/api/runners/%s/poll", c.baseURL, url.PathEscape(c.runnerID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	c.applyHeaders(req)
	res, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == http.StatusNoContent {
		return nil, nil
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control poll returned status %d", res.StatusCode)
	}
	var task Task
	if err := json.NewDecoder(res.Body).Decode(&task); err != nil {
		return nil, err
	}
	return &task, nil
}

func (c *Client) Complete(ctx context.Context, taskID string, result domain.ToolResult) error {
	payload, err := json.Marshal(map[string]any{
		"success": result.Success, "output": result.Output, "error": result.Error,
	})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf(
		"%s/api/runners/%s/tasks/%s/result",
		c.baseURL, url.PathEscape(c.runnerID), url.PathEscape(taskID),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	c.applyHeaders(req)
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("control result returned status %d", res.StatusCode)
	}
	return nil
}

func (c *Client) applyHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Runner "+c.runnerToken)
	req.Header.Set("Content-Type", "application/json")
}
