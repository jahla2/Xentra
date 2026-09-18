package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type AIHTTPClient struct {
	baseURL string
	http    *http.Client
}

func NewAIHTTPClient(baseURL string) *AIHTTPClient {
	return &AIHTTPClient{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 20 * time.Second}}
}

func (c *AIHTTPClient) Investigate(ctx context.Context, request domain.InvestigationRequest) (domain.InvestigationResult, error) {
	var result domain.InvestigationResult
	if err := c.postJSON(ctx, "/v1/investigate", request, &result); err != nil {
		return domain.InvestigationResult{}, err
	}
	return result, nil
}

func (c *AIHTTPClient) Next(ctx context.Context, request domain.AgentInvestigationRequest) (domain.AgentDecision, error) {
	var decision domain.AgentDecision
	if err := c.postJSON(ctx, "/v1/investigate/next", request, &decision); err != nil {
		return domain.AgentDecision{}, err
	}
	return decision, nil
}

func (c *AIHTTPClient) postJSON(ctx context.Context, path string, requestBody, responseBody any) error {
	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("ai service status %d", res.StatusCode)
	}
	if err := json.NewDecoder(res.Body).Decode(responseBody); err != nil {
		return err
	}
	return nil
}
