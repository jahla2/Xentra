package domain

import "time"

type RunnerRegistration struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"-"`
	EnvironmentID  string     `json:"environmentId"`
	CredentialID   string     `json:"-"`
	LastSeenAt     *time.Time `json:"lastSeenAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type RunnerEnrollment struct {
	Environment Environment `json:"environment"`
	RunnerID    string      `json:"runnerId"`
	RunnerToken string      `json:"runnerToken"`
	ControlPath string      `json:"controlPath"`
}

type RunnerTask struct {
	ID        string            `json:"id"`
	RunnerID  string            `json:"runnerId"`
	Tool      string            `json:"tool"`
	Arguments map[string]string `json:"arguments"`
	Status      string            `json:"status"`
	TraceParent string            `json:"traceParent,omitempty"`
	TraceState  string            `json:"traceState,omitempty"`
	CreatedAt   time.Time         `json:"createdAt"`
	ClaimedAt *time.Time        `json:"claimedAt,omitempty"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
	Result    RunnerTaskResult  `json:"result"`
}

type RunnerTaskResult struct {
	Success bool   `json:"success"`
	Output  string `json:"output"`
	Error   string `json:"error,omitempty"`
}
