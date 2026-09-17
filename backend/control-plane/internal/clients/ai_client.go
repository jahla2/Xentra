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

type AIHTTPClient struct { baseURL string; http *http.Client }
func NewAIHTTPClient(baseURL string) *AIHTTPClient { return &AIHTTPClient{baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 20 * time.Second}} }
func (c *AIHTTPClient) Investigate(ctx context.Context, request domain.InvestigationRequest) (domain.InvestigationResult, error) {
	body, _ := json.Marshal(request)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/investigate", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	res, err := c.http.Do(req)
	if err != nil { return domain.InvestigationResult{}, err }
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK { return domain.InvestigationResult{}, fmt.Errorf("ai service status %d", res.StatusCode) }
	var result domain.InvestigationResult
	if err := json.NewDecoder(res.Body).Decode(&result); err != nil { return domain.InvestigationResult{}, err }
	return result, nil
}
