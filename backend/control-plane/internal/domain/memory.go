package domain

import "time"

const IncidentMemoryDimensions = 384

type IncidentMemory struct {
	ID             string
	OrganizationID string
	EnvironmentID  string
	IncidentID     string
	Content        string
	Embedding      []float64
	CreatedAt      time.Time
}

type IncidentMemoryMatch struct {
	Memory   IncidentMemory
	Distance float64
}
