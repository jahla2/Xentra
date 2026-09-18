package clients

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeGitHubAccessTokenProvider struct{}

func (fakeGitHubAccessTokenProvider) Token(context.Context, domain.RepositoryIntegration) (string, error) {
	return "token", nil
}

func TestGitHubClientBuildsRepositoryContextForFailedWorkflow(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer token" {
			t.Fatal("missing token")
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/acme/api/commits":
			fmt.Fprint(w, `[{"sha":"abc123456789","html_url":"https://github/commit/1","commit":{"message":"fix prod\nbody","author":{"date":"2026-09-18T00:00:00Z"}}}]`)
		case "/repos/acme/api/actions/runs":
			fmt.Fprint(w, `{"workflow_runs":[{"name":"deploy","status":"completed","conclusion":"failure","html_url":"https://github/run/1","head_sha":"abc123456789","head_branch":"main","created_at":"2026-09-18T00:01:00Z"}]}`)
		case "/repos/acme/api/commits/abc123456789":
			fmt.Fprint(w, `{"sha":"abc123456789","html_url":"https://github/commit/1","commit":{"message":"fix prod config","author":{"date":"2026-09-18T00:00:00Z"}},"files":[{"filename":"config/app.env","status":"modified","additions":1,"deletions":1,"changes":2,"patch":"-DB_HOST=old\n+DB_HOST=bad"}]}`)
		case "/repos/acme/api/commits/abc123456789/pulls":
			fmt.Fprint(w, `[{"number":17,"title":"Deploy config change","state":"closed","html_url":"https://github/pr/17","updated_at":"2026-09-18T00:00:30Z","merged_at":"2026-09-18T00:00:40Z"}]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewGitHubClientWithBaseURL(fakeGitHubAccessTokenProvider{}, server.URL)
	contextResult, err := client.FetchContext(context.Background(), domain.RepositoryIntegration{Owner: "acme", Repo: "api", AuthMode: "token"})
	if err != nil {
		t.Fatal(err)
	}
	if len(contextResult.Timeline) != 4 {
		t.Fatalf("unexpected timeline: %#v", contextResult.Timeline)
	}
	sources := map[string]bool{}
	var diffOutput string
	for _, item := range contextResult.Evidence {
		sources[item.Source] = true
		if item.Source == "github.commit_diff" {
			diffOutput = item.Output
		}
	}
	for _, expected := range []string{"github.workflow", "github.commit_diff", "github.pull_requests"} {
		if !sources[expected] {
			t.Fatalf("missing correlation evidence %s in %#v", expected, contextResult.Evidence)
		}
	}
	if !strings.Contains(diffOutput, "config/app.env") || !strings.Contains(diffOutput, "DB_HOST=bad") {
		t.Fatalf("diff evidence missing changed file/patch: %q", diffOutput)
	}
}

func TestGitHubClientBoundsPatchEvidence(t *testing.T) {
	largePatch := strings.Repeat("x", maxPatchCharsPerFile+500)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/repos/acme/api/commits":
			fmt.Fprint(w, `[]`)
		case "/repos/acme/api/actions/runs":
			fmt.Fprint(w, `{"workflow_runs":[{"name":"deploy","status":"completed","conclusion":"failure","head_sha":"sha1","created_at":"2026-09-18T00:01:00Z"}]}`)
		case "/repos/acme/api/commits/sha1":
			fmt.Fprintf(w, `{"sha":"sha1","commit":{"message":"change","author":{"date":"2026-09-18T00:00:00Z"}},"files":[{"filename":"large.txt","status":"modified","patch":%q}]}`, largePatch)
		case "/repos/acme/api/commits/sha1/pulls":
			fmt.Fprint(w, `[]`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewGitHubClientWithBaseURL(fakeGitHubAccessTokenProvider{}, server.URL)
	result, err := client.FetchContext(context.Background(), domain.RepositoryIntegration{Owner: "acme", Repo: "api"})
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range result.Evidence {
		if item.Source == "github.commit_diff" && strings.Count(item.Output, "x") > maxPatchCharsPerFile {
			t.Fatalf("patch evidence exceeded per-file bound")
		}
	}
}
