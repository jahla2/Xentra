package domain

import "time"

type VerificationResult struct {
	Healthy  bool       `json:"healthy"`
	Summary  string     `json:"summary"`
	Evidence []Evidence `json:"evidence"`
}

type ActionRequest struct {
	ID             string             `json:"id"`
	OrganizationID string             `json:"-"`
	IncidentID     string             `json:"incidentId,omitempty"`
	EnvironmentID  string             `json:"environmentId"`
	Action         string             `json:"action"`
	Target         string             `json:"target"`
	Reason         string             `json:"reason"`
	Status         string             `json:"status"`
	ApprovedBy     string             `json:"approvedBy,omitempty"`
	RejectedBy     string             `json:"rejectedBy,omitempty"`
	Result         string             `json:"result,omitempty"`
	Verification   VerificationResult `json:"verification"`
	ExecutionStage string             `json:"executionStage"`
	DurationMS     int64              `json:"durationMs"`
	CreatedAt      time.Time          `json:"createdAt"`
	StartedAt      *time.Time         `json:"startedAt,omitempty"`
	ExecutedAt     *time.Time         `json:"executedAt,omitempty"`
	CompletedAt    *time.Time         `json:"completedAt,omitempty"`
}

type AuditEvent struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"-"`
	EnvironmentID  string    `json:"environmentId"`
	Actor          string    `json:"actor"`
	EventType      string    `json:"eventType"`
	Detail         string    `json:"detail"`
	Success        bool      `json:"success"`
	ActionID       string    `json:"actionId,omitempty"`
	Tool           string    `json:"tool,omitempty"`
	Target         string    `json:"target,omitempty"`
	Approval       string    `json:"approval,omitempty"`
	DurationMS     int64     `json:"durationMs"`
	Result         string    `json:"result,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}
