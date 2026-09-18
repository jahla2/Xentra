package httpapi

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type runnerRouterTokenStore struct{ token string }

func (s *runnerRouterTokenStore) StoreRunnerToken(context.Context, string) (string, error) {
	return "cred-runner-router", nil
}

func (s *runnerRouterTokenStore) GetRunnerToken(context.Context, string) (string, error) {
	return s.token, nil
}

func TestDedicatedRunnerRouterAuthenticatesRunnerToken(t *testing.T) {
	ctx := context.Background()
	projects := application.NewMemoryProjectRepository()
	if err := projects.Save(ctx, domain.Project{ID: "prj-1", OrganizationID: "org-a", Name: "Core"}); err != nil {
		t.Fatal(err)
	}
	environments := application.NewMemoryEnvironmentRepository()
	repo := application.NewMemoryRunnerControlRepository()
	tokens := &runnerRouterTokenStore{}
	service := application.NewRunnerControlService(repo, environments, projects, tokens)

	enrollment, err := service.CreateEnrollment(ctx, "org-a", "prj-1", "Production", "production")
	if err != nil {
		t.Fatal(err)
	}
	tokens.token = enrollment.RunnerToken
	router := NewRunnerControlRouter(service)

	bad := httptest.NewRequest(
		http.MethodPost,
		"/api/runners/"+enrollment.RunnerID+"/poll",
		bytes.NewBufferString(`{"os":"linux","hostname":"prod","capabilities":["docker"]}`),
	)
	bad.Header.Set("Authorization", "Runner wrong-token")
	bad.Header.Set("Content-Type", "application/json")
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("bad token status=%d body=%s", badRec.Code, badRec.Body.String())
	}

	good := httptest.NewRequest(
		http.MethodPost,
		"/api/runners/"+enrollment.RunnerID+"/poll",
		bytes.NewBufferString(`{"os":"linux","hostname":"prod","capabilities":["docker"]}`),
	)
	good.Header.Set("Authorization", "Runner "+enrollment.RunnerToken)
	good.Header.Set("Content-Type", "application/json")
	goodRec := httptest.NewRecorder()
	router.ServeHTTP(goodRec, good)
	if goodRec.Code != http.StatusNoContent {
		t.Fatalf("valid token status=%d body=%s", goodRec.Code, goodRec.Body.String())
	}

	env, err := environments.Get(ctx, "org-a", enrollment.Environment.ID)
	if err != nil {
		t.Fatal(err)
	}
	if env.Hostname != "prod" || len(env.Capabilities) != 1 || env.Capabilities[0] != "docker" {
		t.Fatalf("discovery was not persisted: %#v", env)
	}
}
