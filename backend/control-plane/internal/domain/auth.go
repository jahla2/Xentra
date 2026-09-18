package domain

import "time"

const (
	RoleOwner  = "owner"
	RoleMember = "member"
)

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

type Membership struct {
	UserID         string `json:"userId"`
	OrganizationID string `json:"organizationId"`
	Role           string `json:"role"`
}

type Session struct {
	ID             string
	TokenHash      string
	UserID         string
	OrganizationID string
	ExpiresAt      time.Time
	CreatedAt      time.Time
}

type Principal struct {
	UserID           string `json:"userId"`
	Email            string `json:"email"`
	OrganizationID   string `json:"organizationId"`
	OrganizationName string `json:"organizationName"`
	Role             string `json:"role"`
}

type AuthSession struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
	Principal Principal `json:"principal"`
}
