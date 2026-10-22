package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrUserNotFound          = errors.New("user not found")
	ErrUserAlreadyExists     = errors.New("user email already exists")
	ErrOrgNotFound           = errors.New("organization not found")
	ErrMembershipNotFound    = errors.New("membership not found")
	ErrMembershipExists      = errors.New("user is already a member of this organization")
	ErrSessionNotFound       = errors.New("refresh session not found")
	ErrSessionInvalidOrUsed  = errors.New("refresh token invalid, revoked, or already reused")
	ErrCannotRemoveLastOwner = errors.New("cannot remove the last owner of an organization")
)

// Repository defines the storage contract for users, refresh tokens, organizations, and memberships.
type Repository interface {
	// User operations
	CreateUser(ctx context.Context, email, passwordHash, displayName string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (*User, error)

	// Refresh token operations
	CreateRefreshSession(ctx context.Context, session *RefreshSession) error
	GetRefreshSessionByHash(ctx context.Context, tokenHash []byte) (*RefreshSession, error)
	RotateRefreshSession(ctx context.Context, oldSessionID uuid.UUID, newSession *RefreshSession) error
	RevokeRefreshFamily(ctx context.Context, familyID uuid.UUID) error
	RevokeRefreshSessionByHash(ctx context.Context, tokenHash []byte) error

	// Organization & Membership operations
	CreateOrganization(ctx context.Context, name string, creatorID uuid.UUID) (*Organization, error)
	GetOrganizationByID(ctx context.Context, orgID uuid.UUID) (*Organization, error)
	ListUserOrganizations(ctx context.Context, userID uuid.UUID) ([]*UserOrganization, error)
	GetMembership(ctx context.Context, orgID, userID uuid.UUID) (*Membership, error)
	AddMember(ctx context.Context, orgID, userID uuid.UUID, role string) (*Membership, error)
	ListMembers(ctx context.Context, orgID uuid.UUID) ([]*MemberDetail, error)
	RemoveMember(ctx context.Context, orgID, userID uuid.UUID) error
	CountOwners(ctx context.Context, orgID uuid.UUID) (int, error)
}
