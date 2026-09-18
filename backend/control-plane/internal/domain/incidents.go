package domain
import "time"

type RepositoryIntegration struct {
	ID string `json:"id"`
	EnvironmentID string `json:"environmentId"`
	Provider string `json:"provider"`
	Owner string `json:"owner"`
	Repo string `json:"repo"`
	CredentialID string `json:"-"`
}
type TimelineEvent struct {
	Source string `json:"source"`
	Kind string `json:"kind"`
	Summary string `json:"summary"`
	URL string `json:"url,omitempty"`
	OccurredAt time.Time `json:"occurredAt"`
}
type Incident struct {
	ID string `json:"id"`
	EnvironmentID string `json:"environmentId"`
	Question string `json:"question"`
	Status string `json:"status"`
	Summary string `json:"summary"`
	RootCause string `json:"rootCause"`
	Confidence string `json:"confidence"`
	RecommendedAction string `json:"recommendedAction"`
	Evidence []Evidence `json:"evidence"`
	Timeline []TimelineEvent `json:"timeline"`
	CreatedAt time.Time `json:"createdAt"`
}
