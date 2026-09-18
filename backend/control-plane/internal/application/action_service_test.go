package application

import (
	"context"
	"testing"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type fakeRemediation struct{ executions int }

func (f *fakeRemediation) ExecuteAction(context.Context, domain.Environment, string, string) (string, error) {
	f.executions++
	return "restarted", nil
}

func (f *fakeRemediation) VerifyAction(context.Context, domain.Environment, string, string) (domain.VerificationResult, error) {
	return domain.VerificationResult{Healthy: true, Summary: "service healthy"}, nil
}

func TestActionRequiresApprovalBeforeExecution(t *testing.T) {
	envs := NewMemoryEnvironmentRepository()
	_ = envs.Save(context.Background(), domain.Environment{ID: "env-1", OrganizationID: "org-a"})
	incidents := NewMemoryIncidentRepository()
	_ = incidents.Save(context.Background(), domain.Incident{ID: "inc-1", OrganizationID: "org-a", EnvironmentID: "env-1", Status: "action_required"})
	remediation := &fakeRemediation{}
	service := NewActionService(NewMemoryActionRepository(), NewMemoryAuditRepository(), envs, incidents, remediation)

	action, err := service.Propose(context.Background(), "org-a", "inc-1", "env-1", "docker.restart", "api-prod", "recover unhealthy API")
	if err != nil {
		t.Fatal(err)
	}
	if remediation.executions != 0 || action.Status != "pending_approval" {
		t.Fatalf("executed before approval: %#v", action)
	}

	action, err = service.Approve(context.Background(), "org-a", action.ID, "owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if remediation.executions != 1 || action.Status != "completed" || !action.Verification.Healthy {
		t.Fatalf("unexpected action: %#v", action)
	}
	incident, _ := incidents.Get(context.Background(), "org-a", "inc-1")
	if incident.Status != "resolved" {
		t.Fatalf("incident not resolved: %#v", incident)
	}
}

func TestActionRejectsCrossOrganizationEnvironment(t *testing.T) {
	envs := NewMemoryEnvironmentRepository()
	_ = envs.Save(context.Background(), domain.Environment{ID: "env-1", OrganizationID: "org-a"})
	service := NewActionService(NewMemoryActionRepository(), NewMemoryAuditRepository(), envs, NewMemoryIncidentRepository(), &fakeRemediation{})

	if _, err := service.Propose(context.Background(), "org-b", "", "env-1", "docker.restart", "api", "test"); err == nil {
		t.Fatal("expected cross-organization action proposal to fail")
	}
}

func TestActionRejectsNonAllowlistedMutation(t *testing.T) {
	envs := NewMemoryEnvironmentRepository()
	_ = envs.Save(context.Background(), domain.Environment{ID: "env-1", OrganizationID: "org-a"})
	service := NewActionService(NewMemoryActionRepository(), NewMemoryAuditRepository(), envs, NewMemoryIncidentRepository(), &fakeRemediation{})

	if _, err := service.Propose(context.Background(), "org-a", "", "env-1", "shell.exec", "anything", "test"); err == nil {
		t.Fatal("expected non-allowlisted action to be rejected")
	}
}


func TestRejectedActionNeverExecutesAndIsAudited(t *testing.T) {
	envs := NewMemoryEnvironmentRepository()
	_ = envs.Save(context.Background(), domain.Environment{ID: "env-1", OrganizationID: "org-a"})
	incidents := NewMemoryIncidentRepository()
	_ = incidents.Save(context.Background(), domain.Incident{ID: "inc-1", OrganizationID: "org-a", EnvironmentID: "env-1", Status: "action_required"})
	remediation := &fakeRemediation{}
	audit := NewMemoryAuditRepository()
	service := NewActionService(NewMemoryActionRepository(), audit, envs, incidents, remediation)

	action, err := service.Propose(context.Background(), "org-a", "inc-1", "env-1", "docker.restart", "api-prod", "recover unhealthy API")
	if err != nil {
		t.Fatal(err)
	}
	action, err = service.Reject(context.Background(), "org-a", action.ID, "owner@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if action.Status != "rejected" || action.RejectedBy != "owner@example.com" {
		t.Fatalf("unexpected rejected action: %#v", action)
	}
	if remediation.executions != 0 {
		t.Fatalf("rejected action executed %d times", remediation.executions)
	}
	incident, _ := incidents.Get(context.Background(), "org-a", "inc-1")
	if incident.Status == "resolved" {
		t.Fatalf("rejected action must not resolve incident: %#v", incident)
	}
	events, err := audit.List(context.Background(), "org-a")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, event := range events {
		if event.EventType == "action_rejected" && event.Actor == "owner@example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("rejection audit event missing: %#v", events)
	}
	if _, err := service.Approve(context.Background(), "org-a", action.ID, "owner@example.com"); err == nil {
		t.Fatal("rejected action should no longer be approvable")
	}
}
