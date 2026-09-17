package domain

type Environment struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	RunnerURL    string   `json:"runnerUrl"`
	OS           string   `json:"os"`
	Hostname     string   `json:"hostname"`
	Capabilities []string `json:"capabilities"`
}

type Discovery struct {
	OS           string   `json:"os"`
	Hostname     string   `json:"hostname"`
	Capabilities []string `json:"capabilities"`
}

type Evidence struct {
	Source  string `json:"source"`
	Output  string `json:"output"`
	Success bool   `json:"success"`
}

type InvestigationRequest struct {
	Environment Environment `json:"environment"`
	Question    string      `json:"question"`
	Evidence    []Evidence  `json:"evidence"`
}

type InvestigationResult struct {
	Summary           string     `json:"summary"`
	Confidence        string     `json:"confidence"`
	ProbableRootCause string     `json:"probableRootCause"`
	RecommendedAction string     `json:"recommendedAction"`
	Evidence          []Evidence `json:"evidence"`
}
