package application

import (
	"context"
	"errors"
	"regexp"
	"sync"
	"time"

	"github.com/jahla2/Xentra/backend/control-plane/internal/domain"
)

type ActionRepository interface {
	Save(context.Context, domain.ActionRequest) error
	Get(context.Context, string, string) (domain.ActionRequest, error)
}

type AuditRepository interface {
	Append(context.Context, domain.AuditEvent) error
	List(context.Context, string) ([]domain.AuditEvent, error)
}

type RemediationClient interface {
	ExecuteAction(context.Context, domain.Environment, string, string) (string, error)
	VerifyAction(context.Context, domain.Environment, string, string) (domain.VerificationResult, error)
}

type ActionService struct {
	actions      ActionRepository
	audit        AuditRepository
	environments EnvironmentRepository
	incidents    IncidentRepository
	remediation  RemediationClient
}

func NewActionService(actions ActionRepository, audit AuditRepository, environments EnvironmentRepository, incidents IncidentRepository, remediation RemediationClient) *ActionService {
	return &ActionService{actions: actions, audit: audit, environments: environments, incidents: incidents, remediation: remediation}
}

var safeTargetPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@:-]*$`)
var allowedActions = map[string]bool{"docker.restart": true, "system.service_restart": true}

func (s *ActionService) Propose(ctx context.Context, organizationID, incidentID, environmentID, action, target, reason string) (domain.ActionRequest, error) {
	if organizationID == "" || environmentID == "" || action == "" || target == "" || reason == "" {
		return domain.ActionRequest{}, errors.New("organization, environmentId, action, target and reason are required")
	}
	if !allowedActions[action] {
		return domain.ActionRequest{}, errors.New("action is not allowlisted")
	}
	if !safeTargetPattern.MatchString(target) {
		return domain.ActionRequest{}, errors.New("target contains unsafe characters")
	}
	if _, err := s.environments.Get(ctx, organizationID, environmentID); err != nil {
		return domain.ActionRequest{}, err
	}
	if incidentID != "" {
		incident, err := s.incidents.Get(ctx, organizationID, incidentID)
		if err != nil {
			return domain.ActionRequest{}, err
		}
		if incident.EnvironmentID != environmentID {
			return domain.ActionRequest{}, errors.New("incident does not belong to environment")
		}
	}

	id, err := newResourceID("act")
	if err != nil {
		return domain.ActionRequest{}, err
	}
	item := domain.ActionRequest{
		ID: id, OrganizationID: organizationID, IncidentID: incidentID, EnvironmentID: environmentID,
		Action: action, Target: target, Reason: reason, Status: "pending_approval", ExecutionStage: "awaiting_approval", CreatedAt: time.Now().UTC(),
	}
	if err := s.actions.Save(ctx, item); err != nil {
		return domain.ActionRequest{}, err
	}
	_ = s.appendActionAudit(ctx, item, "xentra-ai", "action_proposed", action+" "+target, true, "pending", reason, 0)
	return item, nil
}

func (s *ActionService) Approve(ctx context.Context, organizationID, id, approvedBy string) (domain.ActionRequest, error) {
	if organizationID == "" || approvedBy == "" {
		return domain.ActionRequest{}, errors.New("organization and approver are required")
	}
	item, err := s.actions.Get(ctx, organizationID, id)
	if err != nil {
		return domain.ActionRequest{}, err
	}
	if item.Status != "pending_approval" {
		return domain.ActionRequest{}, errors.New("action is not pending approval")
	}
	env, err := s.environments.Get(ctx, organizationID, item.EnvironmentID)
	if err != nil {
		return domain.ActionRequest{}, err
	}

	item.Status = "approved"
	item.ExecutionStage = "queued"
	item.ApprovedBy = approvedBy
	if err := s.actions.Save(ctx, item); err != nil {
		return domain.ActionRequest{}, err
	}
	_ = s.appendActionAudit(ctx, item, approvedBy, "action_approved", item.Action+" "+item.Target, true, "approved", "queued for execution", 0)

	go s.executeApproved(item, env)
	return item, nil
}

func (s *ActionService) executeApproved(item domain.ActionRequest, env domain.Environment) {
	runCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	persistCtx := context.Background()

	startedAt := time.Now().UTC()
	item.StartedAt = &startedAt
	item.Status = "executing"
	item.ExecutionStage = "executing"
	_ = s.actions.Save(persistCtx, item)

	result, execErr := s.remediation.ExecuteAction(runCtx, env, item.Action, item.Target)
	executedAt := time.Now().UTC()
	item.ExecutedAt = &executedAt
	item.Result = result
	if execErr != nil {
		item.Status = "failed"
		item.ExecutionStage = "failed"
		item.Result = execErr.Error()
		completedAt := time.Now().UTC()
		item.CompletedAt = &completedAt
		item.DurationMS = completedAt.Sub(startedAt).Milliseconds()
		_ = s.actions.Save(persistCtx, item)
		_ = s.appendActionAudit(persistCtx, item, "xentra-runner", "action_executed", execErr.Error(), false, "approved", item.Result, item.DurationMS)
		return
	}

	item.Status = "verifying"
	item.ExecutionStage = "verifying"
	_ = s.actions.Save(persistCtx, item)

	verification, verifyErr := s.remediation.VerifyAction(runCtx, env, item.Action, item.Target)
	if verifyErr != nil {
		item.Status = "verification_failed"
		item.ExecutionStage = "verification_failed"
		item.Verification = domain.VerificationResult{Healthy: false, Summary: verifyErr.Error()}
	} else {
		item.Verification = verification
		if verification.Healthy {
			item.Status = "completed"
			item.ExecutionStage = "completed"
		} else {
			item.Status = "verification_failed"
			item.ExecutionStage = "verification_failed"
		}
	}

	completedAt := time.Now().UTC()
	item.CompletedAt = &completedAt
	item.DurationMS = completedAt.Sub(startedAt).Milliseconds()
	_ = s.actions.Save(persistCtx, item)

	success := item.Status == "completed"
	auditResult := item.Result
	if item.Verification.Summary != "" {
		if auditResult != "" {
			auditResult += " | "
		}
		auditResult += item.Verification.Summary
	}
	_ = s.appendActionAudit(persistCtx, item, "xentra-runner", "action_executed", auditResult, success, "approved", auditResult, item.DurationMS)
	if success && item.IncidentID != "" {
		if incident, getErr := s.incidents.Get(persistCtx, item.OrganizationID, item.IncidentID); getErr == nil {
			incident.Status = "resolved"
			_ = s.incidents.Save(persistCtx, incident)
		}
	}
}

func (s *ActionService) Get(ctx context.Context, organizationID, id string) (domain.ActionRequest, error) {
	return s.actions.Get(ctx, organizationID, id)
}

func (s *ActionService) Reject(ctx context.Context, organizationID, id, rejectedBy string) (domain.ActionRequest, error) {
	if organizationID == "" || rejectedBy == "" {
		return domain.ActionRequest{}, errors.New("organization and rejector are required")
	}
	item, err := s.actions.Get(ctx, organizationID, id)
	if err != nil {
		return domain.ActionRequest{}, err
	}
	if item.Status != "pending_approval" {
		return domain.ActionRequest{}, errors.New("action is not pending approval")
	}
	item.Status = "rejected"
	item.ExecutionStage = "rejected"
	item.RejectedBy = rejectedBy
	if err := s.actions.Save(ctx, item); err != nil {
		return domain.ActionRequest{}, err
	}
	_ = s.appendActionAudit(ctx, item, rejectedBy, "action_rejected", item.Action+" "+item.Target, true, "rejected", "rejected by owner", 0)
	return item, nil
}

func (s *ActionService) Audit(ctx context.Context, organizationID string) ([]domain.AuditEvent, error) {
	return s.audit.List(ctx, organizationID)
}

func (s *ActionService) appendActionAudit(ctx context.Context, item domain.ActionRequest, actor, eventType, detail string, success bool, approval, result string, durationMS int64) error {
	id, err := newResourceID("audit")
	if err != nil {
		return err
	}
	return s.audit.Append(ctx, domain.AuditEvent{
		ID: id, OrganizationID: item.OrganizationID, EnvironmentID: item.EnvironmentID, Actor: actor,
		EventType: eventType, Detail: detail, Success: success, ActionID: item.ID,
		Tool: item.Action, Target: item.Target, Approval: approval, DurationMS: durationMS,
		Result: result, CreatedAt: time.Now().UTC(),
	})
}

type MemoryActionRepository struct {
	mu   sync.RWMutex
	data map[string]domain.ActionRequest
}

func NewMemoryActionRepository() *MemoryActionRepository {
	return &MemoryActionRepository{data: map[string]domain.ActionRequest{}}
}

func (r *MemoryActionRepository) Save(_ context.Context, item domain.ActionRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.data[item.ID] = item
	return nil
}

func (r *MemoryActionRepository) Get(_ context.Context, organizationID, id string) (domain.ActionRequest, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.data[id]
	if !ok || item.OrganizationID != organizationID {
		return domain.ActionRequest{}, errors.New("action not found")
	}
	return item, nil
}

type MemoryAuditRepository struct {
	mu    sync.RWMutex
	items []domain.AuditEvent
}

func NewMemoryAuditRepository() *MemoryAuditRepository {
	return &MemoryAuditRepository{}
}

func (r *MemoryAuditRepository) Append(_ context.Context, item domain.AuditEvent) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = append(r.items, item)
	return nil
}

func (r *MemoryAuditRepository) List(_ context.Context, organizationID string) ([]domain.AuditEvent, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := []domain.AuditEvent{}
	for _, item := range r.items {
		if item.OrganizationID == organizationID {
			items = append(items, item)
		}
	}
	return items, nil
}
