package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/application"
	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type routerRemediation struct{}

func (routerRemediation) ExecuteAction(context.Context, domain.Environment, string, string) (string, error) {
	return "restarted", nil
}
func (routerRemediation) VerifyAction(context.Context, domain.Environment, string, string) (domain.VerificationResult, error) {
	return domain.VerificationResult{Healthy:true, Summary:"healthy"}, nil
}

func TestActionEndpointRequiresApprovalThenExecutes(t *testing.T) {
	envs := application.NewMemoryEnvironmentRepository()
	_ = envs.Save(context.Background(), domain.Environment{ID:"env-1"})
	incidents := application.NewMemoryIncidentRepository()
	_ = incidents.Save(context.Background(), domain.Incident{ID:"inc-1", EnvironmentID:"env-1", Status:"action_required"})
	actions := application.NewActionService(application.NewMemoryActionRepository(), application.NewMemoryAuditRepository(), envs, incidents, routerRemediation{})
	router := NewRouter(nil, nil, Services{Actions:actions})

	body := bytes.NewBufferString(`{"incidentId":"inc-1","environmentId":"env-1","action":"docker.restart","target":"api","reason":"recover"}`)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/actions", body))
	if rec.Code != http.StatusCreated { t.Fatalf("propose status=%d body=%s", rec.Code, rec.Body.String()) }

	var proposed domain.ActionRequest
	if err := json.NewDecoder(rec.Body).Decode(&proposed); err != nil { t.Fatal(err) }
	if proposed.Status != "pending_approval" { t.Fatalf("unexpected proposed action: %#v", proposed) }

	rec = httptest.NewRecorder()
	approveBody := bytes.NewBufferString(`{"approvedBy":"rey"}`)
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/actions/"+proposed.ID+"/approve", approveBody))
	if rec.Code != http.StatusOK { t.Fatalf("approve status=%d body=%s", rec.Code, rec.Body.String()) }

	var approved domain.ActionRequest
	if err := json.NewDecoder(rec.Body).Decode(&approved); err != nil { t.Fatal(err) }
	if approved.Status != "completed" || !approved.Verification.Healthy { t.Fatalf("unexpected approved action: %#v", approved) }
}
