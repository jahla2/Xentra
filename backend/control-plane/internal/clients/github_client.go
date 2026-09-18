package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type GitHubTokenProvider interface {
	GetGitHubToken(context.Context, string) (string, error)
}

type GitHubClient struct {
	credentials GitHubTokenProvider
	baseURL     string
	http        *http.Client
}

func NewGitHubClient(credentials GitHubTokenProvider) *GitHubClient {
	return &GitHubClient{credentials: credentials, baseURL: "https://api.github.com", http: &http.Client{Timeout: 10 * time.Second}}
}

func NewGitHubClientWithBaseURL(credentials GitHubTokenProvider, baseURL string) *GitHubClient {
	return &GitHubClient{credentials: credentials, baseURL: strings.TrimRight(baseURL, "/"), http: &http.Client{Timeout: 10 * time.Second}}
}

func (c *GitHubClient) FetchTimeline(ctx context.Context, integration domain.RepositoryIntegration) ([]domain.TimelineEvent, error) {
	token, err := c.credentials.GetGitHubToken(ctx, integration.CredentialID)
	if err != nil {
		return nil, err
	}
	commits, err := c.fetchCommits(ctx, integration, token)
	if err != nil {
		return nil, err
	}
	runs, err := c.fetchWorkflowRuns(ctx, integration, token)
	if err != nil {
		return nil, err
	}
	events := append(commits, runs...)
	sort.Slice(events, func(i, j int) bool { return events[i].OccurredAt.Before(events[j].OccurredAt) })
	return events, nil
}

func (c *GitHubClient) fetchCommits(ctx context.Context, integration domain.RepositoryIntegration, token string) ([]domain.TimelineEvent, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/commits?per_page=10", c.baseURL, url.PathEscape(integration.Owner), url.PathEscape(integration.Repo))
	var payload []struct {
		HTMLURL string `json:"html_url"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := c.getJSON(ctx, endpoint, token, &payload); err != nil {
		return nil, err
	}
	events := make([]domain.TimelineEvent, 0, len(payload))
	for _, item := range payload {
		message := strings.Split(item.Commit.Message, "
")[0]
		events = append(events, domain.TimelineEvent{Source: "github", Kind: "commit", Summary: message, URL: item.HTMLURL, OccurredAt: item.Commit.Author.Date})
	}
	return events, nil
}

func (c *GitHubClient) fetchWorkflowRuns(ctx context.Context, integration domain.RepositoryIntegration, token string) ([]domain.TimelineEvent, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/actions/runs?per_page=10", c.baseURL, url.PathEscape(integration.Owner), url.PathEscape(integration.Repo))
	var payload struct {
		WorkflowRuns []struct {
			Name       string    `json:"name"`
			Status     string    `json:"status"`
			Conclusion string    `json:"conclusion"`
			HTMLURL    string    `json:"html_url"`
			CreatedAt  time.Time `json:"created_at"`
		} `json:"workflow_runs"`
	}
	if err := c.getJSON(ctx, endpoint, token, &payload); err != nil {
		return nil, err
	}
	events := make([]domain.TimelineEvent, 0, len(payload.WorkflowRuns))
	for _, run := range payload.WorkflowRuns {
		state := run.Conclusion
		if state == "" {
			state = run.Status
		}
		events = append(events, domain.TimelineEvent{Source: "github", Kind: "workflow", Summary: run.Name + ": " + state, URL: run.HTMLURL, OccurredAt: run.CreatedAt})
	}
	return events, nil
}

func (c *GitHubClient) getJSON(ctx context.Context, endpoint, token string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("github status %d", res.StatusCode)
	}
	return json.NewDecoder(res.Body).Decode(target)
}
