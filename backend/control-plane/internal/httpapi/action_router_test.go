package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type routerRemediation struct{}

func (routerRemediation) ExecuteAction(context.Context, domain.Environment, string, string) (string, error) {
	return "restarted", nil
}

func (routerRemediation) VerifyAction(context.Context, domain.Environment, string, string) (domain.VerificationResult, error) {
	return domain.VerificationResult{Healthy: true, Summary: "healthy"}, nil
}

func TestActionEndpointRequiresOwnerApprovalThenExecutes(t *testing.T) {
	auth := application.NewAuthService(application.NewMemoryAuthRepository(), time.Hour)
	owner, err := auth.Register(context.Background(), "owner@example.com", "very-secure-password", "Acme")
	if err != nil {
		t.Fatal(err)
	}

	envs := application.NewMemoryEnvironmentRepository()
	_ = envs.Save(context.Background(), domain.Environment{ID: "env-1", OrganizationID: owner.Principal.OrganizationID})
	incidents := application.NewMemoryIncidentRepository()
	_ = incidents.Save(context.Background(), domain.Incident{ID: "inc-1", OrganizationID: owner.Principal.OrganizationID, EnvironmentID: "env-1", Status: "action_required"})
	actions := application.NewActionService(application.NewMemoryActionRepository(), application.NewMemoryAuditRepository(), envs, incidents, routerRemediation{})
	router := NewRouter(nil, nil, Services{Auth: auth, Actions: actions})

	body := bytes.NewBufferString(`{"incidentId":"inc-1","environmentId":"env-1","action":"docker.restart","target":"api","reason":"recover"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/actions", body)
	req.Header.Set("Authorization", "Bearer "+owner.Token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("propose status=%d body=%s", rec.Code, rec.Body.String())
	}

	var proposed domain.ActionRequest
	if err := json.NewDecoder(rec.Body).Decode(&proposed); err != nil {
		t.Fatal(err)
	}
	if proposed.Status != "pending_approval" {
		t.Fatalf("unexpected proposed action: %#v", proposed)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/actions/"+proposed.ID+"/approve", nil)
	req.Header.Set("Authorization", "Bearer "+owner.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve status=%d body=%s", rec.Code, rec.Body.String())
	}

	var approved domain.ActionRequest
	if err := json.NewDecoder(rec.Body).Decode(&approved); err != nil {
		t.Fatal(err)
	}
	if approved.Status != "completed" || approved.ApprovedBy != "owner@example.com" || !approved.Verification.Healthy {
		t.Fatalf("unexpected approved action: %#v", approved)
	}
}


func TestActionEndpointAllowsOwnerToRejectWithoutExecution(t *testing.T) {
	auth := application.NewAuthService(application.NewMemoryAuthRepository(), time.Hour)
	owner, err := auth.Register(context.Background(), "owner@example.com", "very-secure-password", "Acme")
	if err != nil {
		t.Fatal(err)
	}

	envs := application.NewMemoryEnvironmentRepository()
	_ = envs.Save(context.Background(), domain.Environment{ID: "env-1", OrganizationID: owner.Principal.OrganizationID})
	incidents := application.NewMemoryIncidentRepository()
	_ = incidents.Save(context.Background(), domain.Incident{ID: "inc-1", OrganizationID: owner.Principal.OrganizationID, EnvironmentID: "env-1", Status: "action_required"})
	remediation := &countingRouterRemediation{}
	actions := application.NewActionService(application.NewMemoryActionRepository(), application.NewMemoryAuditRepository(), envs, incidents, remediation)
	router := NewRouter(nil, nil, Services{Auth: auth, Actions: actions})

	body := bytes.NewBufferString(`{"incidentId":"inc-1","environmentId":"env-1","action":"docker.restart","target":"api","reason":"recover"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/actions", body)
	req.Header.Set("Authorization", "Bearer "+owner.Token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("propose status=%d body=%s", rec.Code, rec.Body.String())
	}
	var proposed domain.ActionRequest
	if err := json.NewDecoder(rec.Body).Decode(&proposed); err != nil {
		t.Fatal(err)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/actions/"+proposed.ID+"/reject", nil)
	req.Header.Set("Authorization", "Bearer "+owner.Token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reject status=%d body=%s", rec.Code, rec.Body.String())
	}
	var rejected domain.ActionRequest
	if err := json.NewDecoder(rec.Body).Decode(&rejected); err != nil {
		t.Fatal(err)
	}
	if rejected.Status != "rejected" || rejected.RejectedBy != "owner@example.com" {
		t.Fatalf("unexpected rejected action: %#v", rejected)
	}
	if remediation.executions != 0 {
		t.Fatalf("rejected endpoint executed remediation %d times", remediation.executions)
	}
}

type countingRouterRemediation struct{ executions int }

func (r *countingRouterRemediation) ExecuteAction(context.Context, domain.Environment, string, string) (string, error) {
	r.executions++
	return "restarted", nil
}

func (r *countingRouterRemediation) VerifyAction(context.Context, domain.Environment, string, string) (domain.VerificationResult, error) {
	return domain.VerificationResult{Healthy: true, Summary: "healthy"}, nil
}
