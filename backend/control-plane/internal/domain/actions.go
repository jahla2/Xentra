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
	CreatedAt      time.Time          `json:"createdAt"`
	ExecutedAt     *time.Time         `json:"executedAt,omitempty"`
}

type AuditEvent struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"-"`
	EnvironmentID  string    `json:"environmentId"`
	Actor          string    `json:"actor"`
	EventType      string    `json:"eventType"`
	Detail         string    `json:"detail"`
	Success        bool      `json:"success"`
	CreatedAt      time.Time `json:"createdAt"`
}
