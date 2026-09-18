package domain

import "time"

type Project struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"-"`
	Name           string    `json:"name"`
	Description    string    `json:"description,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}
