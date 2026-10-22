package identity

import (
	"time"

	"github.com/google/uuid"
)

const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// User represents a registered system user.
type User struct {
	ID              uuid.UUID  `json:"id"`
	Email           string     `json:"email"`
	PasswordHash    string     `json:"-"`
	DisplayName     string     `json:"display_name"`
	EmailVerifiedAt *time.Time `json:"email_verified_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// Organization represents a multi-tenant workspace boundary.
type Organization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	CreatedBy uuid.UUID `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// Membership establishes a user's role within an organization.
type Membership struct {
	OrganizationID uuid.UUID `json:"organization_id"`
	UserID         uuid.UUID `json:"user_id"`
	Role           string    `json:"role"`
	CreatedAt      time.Time `json:"created_at"`
}

// UserOrganization combines organization metadata with the user's role in that organization.
type UserOrganization struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// MemberDetail provides user profile information along with their membership role in an organization.
type MemberDetail struct {
	UserID      uuid.UUID `json:"user_id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	JoinedAt    time.Time `json:"joined_at"`
}

// RefreshSession tracks an opaque refresh token family and replacement chain.
type RefreshSession struct {
	ID         uuid.UUID  `json:"id"`
	UserID     uuid.UUID  `json:"user_id"`
	TokenHash  []byte     `json:"-"`
	FamilyID   uuid.UUID  `json:"family_id"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	ReplacedBy *uuid.UUID `json:"replaced_by,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// IsActive returns true if the session is unrevoked, unreplaced, and not expired.
func (s *RefreshSession) IsActive() bool {
	if s.RevokedAt != nil || s.ReplacedBy != nil {
		return false
	}
	return time.Now().UTC().Before(s.ExpiresAt)
}
