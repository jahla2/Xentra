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
		Action: action, Target: target, Reason: reason, Status: "pending_approval", CreatedAt: time.Now().UTC(),
	}
	if err := s.actions.Save(ctx, item); err != nil {
		return domain.ActionRequest{}, err
	}
	_ = s.appendAudit(ctx, organizationID, environmentID, "xentra-ai", "action_proposed", action+" "+target, true)
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
	item.ApprovedBy = approvedBy
	_ = s.actions.Save(ctx, item)
	_ = s.appendAudit(ctx, organizationID, item.EnvironmentID, approvedBy, "action_approved", item.Action+" "+item.Target, true)

	result, execErr := s.remediation.ExecuteAction(ctx, env, item.Action, item.Target)
	now := time.Now().UTC()
	item.ExecutedAt = &now
	item.Result = result
	if execErr != nil {
		item.Status = "failed"
		item.Result = execErr.Error()
		_ = s.actions.Save(ctx, item)
		_ = s.appendAudit(ctx, organizationID, item.EnvironmentID, "xentra-runner", "action_executed", execErr.Error(), false)
		return item, nil
	}

	verification, verifyErr := s.remediation.VerifyAction(ctx, env, item.Action, item.Target)
	if verifyErr != nil {
		item.Status = "verification_failed"
		item.Verification = domain.VerificationResult{Healthy: false, Summary: verifyErr.Error()}
	} else {
		item.Verification = verification
		if verification.Healthy {
			item.Status = "completed"
		} else {
			item.Status = "verification_failed"
		}
	}
	if err := s.actions.Save(ctx, item); err != nil {
		return domain.ActionRequest{}, err
	}

	success := item.Status == "completed"
	_ = s.appendAudit(ctx, organizationID, item.EnvironmentID, "xentra-runner", "action_executed", item.Result, success)
	if success && item.IncidentID != "" {
		if incident, getErr := s.incidents.Get(ctx, organizationID, item.IncidentID); getErr == nil {
			incident.Status = "resolved"
			_ = s.incidents.Save(ctx, incident)
		}
	}
	return item, nil
}

func (s *ActionService) Audit(ctx context.Context, organizationID string) ([]domain.AuditEvent, error) {
	return s.audit.List(ctx, organizationID)
}

func (s *ActionService) appendAudit(ctx context.Context, organizationID, environmentID, actor, eventType, detail string, success bool) error {
	id, err := newResourceID("audit")
	if err != nil {
		return err
	}
	return s.audit.Append(ctx, domain.AuditEvent{
		ID: id, OrganizationID: organizationID, EnvironmentID: environmentID, Actor: actor,
		EventType: eventType, Detail: detail, Success: success, CreatedAt: time.Now().UTC(),
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
