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
	"github.com/jahla2/Xentra/backend/control-plane/internal/observability"
)

const (
	maxCorrelationFiles      = 20
	maxPatchCharsPerFile     = 3000
	maxCorrelationPatchChars = 12000
)

type GitHubAccessTokenProvider interface {
	Token(context.Context, domain.RepositoryIntegration) (string, error)
}

type GitHubClient struct {
	auth    GitHubAccessTokenProvider
	baseURL string
	http    *http.Client
}

func NewGitHubClient(auth GitHubAccessTokenProvider) *GitHubClient {
	return NewGitHubClientWithBaseURL(auth, "https://api.github.com")
}

func NewGitHubClientWithBaseURL(auth GitHubAccessTokenProvider, baseURL string) *GitHubClient {
	return &GitHubClient{
		auth: auth,
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{Timeout: 10 * time.Second, Transport: observability.NewTracingTransport(nil)},
	}
}

func (c *GitHubClient) FetchTimeline(ctx context.Context, integration domain.RepositoryIntegration) ([]domain.TimelineEvent, error) {
	repositoryContext, err := c.FetchContext(ctx, integration)
	if err != nil {
		return nil, err
	}
	return repositoryContext.Timeline, nil
}

func (c *GitHubClient) FetchContext(ctx context.Context, integration domain.RepositoryIntegration) (domain.RepositoryContext, error) {
	token, err := c.auth.Token(ctx, integration)
	if err != nil {
		return domain.RepositoryContext{}, err
	}

	commits, err := c.fetchCommits(ctx, integration, token)
	if err != nil {
		return domain.RepositoryContext{}, err
	}
	runs, err := c.fetchWorkflowRuns(ctx, integration, token)
	if err != nil {
		return domain.RepositoryContext{}, err
	}

	result := domain.RepositoryContext{
		Timeline: append([]domain.TimelineEvent{}, commits.Events...),
		Evidence: append([]domain.Evidence{}, runs.Evidence...),
	}
	result.Timeline = append(result.Timeline, runs.Events...)

	selectedRun := selectCorrelationRun(runs.Runs)
	if selectedRun.HeadSHA != "" {
		detail, detailErr := c.fetchCommitDetail(ctx, integration, token, selectedRun.HeadSHA)
		if detailErr != nil {
			result.Evidence = append(result.Evidence, domain.Evidence{
				Source: "github.commit_diff", Output: detailErr.Error(), Success: false,
				OccurredAt: time.Now().UTC(),
			})
		} else {
			result.Timeline = append(result.Timeline, detail.Events...)
			result.Evidence = append(result.Evidence, detail.Evidence...)
		}

		pulls, pullsErr := c.fetchPullRequestsForCommit(ctx, integration, token, selectedRun.HeadSHA)
		if pullsErr != nil {
			result.Evidence = append(result.Evidence, domain.Evidence{
				Source: "github.pull_requests", Output: pullsErr.Error(), Success: false,
				OccurredAt: time.Now().UTC(),
			})
		} else {
			result.Timeline = append(result.Timeline, pulls.Events...)
			result.Evidence = append(result.Evidence, pulls.Evidence...)
		}
	}

	sort.SliceStable(result.Timeline, func(i, j int) bool {
		return result.Timeline[i].OccurredAt.Before(result.Timeline[j].OccurredAt)
	})
	return result, nil
}

type workflowRun struct {
	Name       string
	Status     string
	Conclusion string
	HTMLURL    string
	HeadSHA    string
	HeadBranch string
	CreatedAt  time.Time
}

type workflowRunResult struct {
	Runs     []workflowRun
	Events   []domain.TimelineEvent
	Evidence []domain.Evidence
}

func (c *GitHubClient) fetchWorkflowRuns(ctx context.Context, integration domain.RepositoryIntegration, token string) (workflowRunResult, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/actions/runs?per_page=10", c.baseURL, url.PathEscape(integration.Owner), url.PathEscape(integration.Repo))
	var payload struct {
		WorkflowRuns []struct {
			Name       string    `json:"name"`
			Status     string    `json:"status"`
			Conclusion string    `json:"conclusion"`
			HTMLURL    string    `json:"html_url"`
			HeadSHA    string    `json:"head_sha"`
			HeadBranch string    `json:"head_branch"`
			CreatedAt  time.Time `json:"created_at"`
		} `json:"workflow_runs"`
	}
	startedAt := time.Now().UTC()
	if err := c.getJSON(ctx, endpoint, token, &payload); err != nil {
		return workflowRunResult{}, err
	}
	duration := time.Since(startedAt).Milliseconds()

	result := workflowRunResult{}
	for _, run := range payload.WorkflowRuns {
		state := run.Conclusion
		if state == "" {
			state = run.Status
		}
		item := workflowRun{
			Name: run.Name, Status: run.Status, Conclusion: run.Conclusion,
			HTMLURL: run.HTMLURL, HeadSHA: run.HeadSHA, HeadBranch: run.HeadBranch,
			CreatedAt: run.CreatedAt,
		}
		result.Runs = append(result.Runs, item)
		result.Events = append(result.Events, domain.TimelineEvent{
			Source: "github", Kind: "workflow",
			Summary: fmt.Sprintf("%s: %s (%s)", run.Name, state, shortSHA(run.HeadSHA)),
			URL: run.HTMLURL, OccurredAt: run.CreatedAt,
		})
	}
	if selected := selectCorrelationRun(result.Runs); selected.Name != "" {
		result.Evidence = append(result.Evidence, domain.Evidence{
			Source: "github.workflow",
			Output: fmt.Sprintf(
				"name=%s\nstatus=%s\nconclusion=%s\nbranch=%s\nhead_sha=%s\nurl=%s",
				selected.Name, selected.Status, selected.Conclusion, selected.HeadBranch, selected.HeadSHA, selected.HTMLURL,
			),
			Success: true, OccurredAt: selected.CreatedAt, DurationMS: duration,
		})
	}
	return result, nil
}

type commitListResult struct {
	Events []domain.TimelineEvent
}

func (c *GitHubClient) fetchCommits(ctx context.Context, integration domain.RepositoryIntegration, token string) (commitListResult, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/commits?per_page=10", c.baseURL, url.PathEscape(integration.Owner), url.PathEscape(integration.Repo))
	var payload []struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
		Commit  struct {
			Message string `json:"message"`
			Author  struct {
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
	}
	if err := c.getJSON(ctx, endpoint, token, &payload); err != nil {
		return commitListResult{}, err
	}
	result := commitListResult{}
	for _, item := range payload {
		message := strings.Split(item.Commit.Message, "\n")[0]
		result.Events = append(result.Events, domain.TimelineEvent{
			Source: "github", Kind: "commit",
			Summary: fmt.Sprintf("%s %s", shortSHA(item.SHA), message),
			URL: item.HTMLURL, OccurredAt: item.Commit.Author.Date,
		})
	}
	return result, nil
}

type correlationResult struct {
	Events   []domain.TimelineEvent
	Evidence []domain.Evidence
}

func (c *GitHubClient) fetchCommitDetail(ctx context.Context, integration domain.RepositoryIntegration, token, sha string) (correlationResult, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/commits/%s", c.baseURL, url.PathEscape(integration.Owner), url.PathEscape(integration.Repo), url.PathEscape(sha))
	var payload struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
		Commit  struct {
			Message string `json:"message"`
			Author  struct {
				Date time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
		Files []struct {
			Filename  string `json:"filename"`
			Status    string `json:"status"`
			Additions int    `json:"additions"`
			Deletions int    `json:"deletions"`
			Changes   int    `json:"changes"`
			Patch     string `json:"patch"`
		} `json:"files"`
	}
	startedAt := time.Now().UTC()
	if err := c.getJSON(ctx, endpoint, token, &payload); err != nil {
		return correlationResult{}, err
	}
	duration := time.Since(startedAt).Milliseconds()
	occurredAt := payload.Commit.Author.Date
	if occurredAt.IsZero() {
		occurredAt = startedAt
	}

	var builder strings.Builder
	fmt.Fprintf(&builder, "commit=%s\nmessage=%s\nurl=%s\nchanged_files=%d\n", payload.SHA, payload.Commit.Message, payload.HTMLURL, len(payload.Files))
	remainingPatchChars := maxCorrelationPatchChars
	for index, file := range payload.Files {
		if index >= maxCorrelationFiles {
			fmt.Fprintf(&builder, "... %d additional files omitted\n", len(payload.Files)-index)
			break
		}
		fmt.Fprintf(&builder, "\nfile=%s status=%s additions=%d deletions=%d changes=%d\n", file.Filename, file.Status, file.Additions, file.Deletions, file.Changes)
		if file.Patch != "" && remainingPatchChars > 0 {
			limit := minInt(len(file.Patch), maxPatchCharsPerFile, remainingPatchChars)
			builder.WriteString("patch:\n")
			builder.WriteString(file.Patch[:limit])
			builder.WriteString("\n")
			remainingPatchChars -= limit
		}
	}

	return correlationResult{
		Events: []domain.TimelineEvent{{
			Source: "github", Kind: "change",
			Summary: fmt.Sprintf("%s changed %d files", shortSHA(payload.SHA), len(payload.Files)),
			URL: payload.HTMLURL, OccurredAt: occurredAt,
		}},
		Evidence: []domain.Evidence{{
			Source: "github.commit_diff", Output: builder.String(), Success: true,
			OccurredAt: occurredAt, DurationMS: duration,
		}},
	}, nil
}

func (c *GitHubClient) fetchPullRequestsForCommit(ctx context.Context, integration domain.RepositoryIntegration, token, sha string) (correlationResult, error) {
	endpoint := fmt.Sprintf("%s/repos/%s/%s/commits/%s/pulls", c.baseURL, url.PathEscape(integration.Owner), url.PathEscape(integration.Repo), url.PathEscape(sha))
	var payload []struct {
		Number    int        `json:"number"`
		Title     string     `json:"title"`
		State     string     `json:"state"`
		HTMLURL   string     `json:"html_url"`
		UpdatedAt time.Time  `json:"updated_at"`
		MergedAt  *time.Time `json:"merged_at"`
	}
	startedAt := time.Now().UTC()
	if err := c.getJSON(ctx, endpoint, token, &payload); err != nil {
		return correlationResult{}, err
	}
	duration := time.Since(startedAt).Milliseconds()
	if len(payload) == 0 {
		return correlationResult{}, nil
	}

	result := correlationResult{}
	var builder strings.Builder
	for _, pull := range payload {
		state := pull.State
		if pull.MergedAt != nil {
			state = "merged"
		}
		fmt.Fprintf(&builder, "PR #%d %s [%s] %s\n", pull.Number, pull.Title, state, pull.HTMLURL)
		result.Events = append(result.Events, domain.TimelineEvent{
			Source: "github", Kind: "pull_request",
			Summary: fmt.Sprintf("PR #%d %s [%s]", pull.Number, pull.Title, state),
			URL: pull.HTMLURL, OccurredAt: pull.UpdatedAt,
		})
	}
	result.Evidence = append(result.Evidence, domain.Evidence{
		Source: "github.pull_requests", Output: builder.String(), Success: true,
		OccurredAt: startedAt, DurationMS: duration,
	})
	return result, nil
}

func selectCorrelationRun(runs []workflowRun) workflowRun {
	for _, run := range runs {
		if run.Conclusion == "failure" {
			return run
		}
	}
	if len(runs) > 0 {
		return runs[0]
	}
	return workflowRun{}
}

func (c *GitHubClient) getJSON(ctx context.Context, endpoint, token string, target any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("github status %d for %s", res.StatusCode, req.URL.Path)
	}
	return json.NewDecoder(res.Body).Decode(target)
}

func shortSHA(sha string) string {
	if len(sha) <= 8 {
		return sha
	}
	return sha[:8]
}

func minInt(values ...int) int {
	if len(values) == 0 {
		return 0
	}
	result := values[0]
	for _, value := range values[1:] {
		if value < result {
			result = value
		}
	}
	return result
}
