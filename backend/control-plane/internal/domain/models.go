package domain

import "time"

type Environment struct {
	ID                    string   `json:"id"`
	OrganizationID        string   `json:"-"`
	ProjectID             string   `json:"projectId"`
	Name                  string   `json:"name"`
	Type                  string   `json:"type"`
	ConnectionType        string   `json:"connectionType"`
	RunnerURL             string   `json:"runnerUrl,omitempty"`
	SSHHost               string   `json:"sshHost,omitempty"`
	SSHPort               int      `json:"sshPort,omitempty"`
	SSHUser               string   `json:"sshUser,omitempty"`
	SSHHostKeyFingerprint string   `json:"sshHostKeyFingerprint,omitempty"`
	CredentialID          string   `json:"credentialId,omitempty"`
	OS                    string   `json:"os"`
	Hostname              string   `json:"hostname"`
	Capabilities          []string `json:"capabilities"`
}

type Discovery struct {
	OS           string   `json:"os"`
	Hostname     string   `json:"hostname"`
	Capabilities []string `json:"capabilities"`
}

type SSHCredential struct {
	PrivateKey string `json:"privateKey"`
	Passphrase string `json:"passphrase,omitempty"`
}

type CredentialRecord struct {
	ID         string
	Kind       string
	Ciphertext []byte
}

type Evidence struct {
	Source     string    `json:"source"`
	Output     string    `json:"output"`
	Success    bool      `json:"success"`
	OccurredAt time.Time `json:"occurredAt"`
	DurationMS int64     `json:"durationMs"`
}

type ToolRequest struct {
	Tool      string            `json:"tool"`
	Arguments map[string]string `json:"arguments"`
}

type InvestigationRequest struct {
	Environment Environment `json:"environment"`
	Question    string      `json:"question"`
	Evidence    []Evidence  `json:"evidence"`
}

type AgentInvestigationRequest struct {
	Environment    Environment `json:"environment"`
	Question       string      `json:"question"`
	Evidence       []Evidence  `json:"evidence"`
	AvailableTools []string    `json:"availableTools"`
	RemainingSteps int         `json:"remainingSteps"`
}

type AgentDecision struct {
	Mode         string               `json:"mode"`
	ToolRequests []ToolRequest        `json:"toolRequests,omitempty"`
	Finding      *InvestigationResult `json:"finding,omitempty"`
}

type InvestigationResult struct {
	Summary           string     `json:"summary"`
	Confidence        string     `json:"confidence"`
	ProbableRootCause string     `json:"probableRootCause"`
	RecommendedAction string     `json:"recommendedAction"`
	Evidence          []Evidence `json:"evidence"`
}
