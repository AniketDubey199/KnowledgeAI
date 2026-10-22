package identity

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"knowledgebase-ai/internal/apperror"
)

// Service encapsulates organization creation and RBAC membership governance.
type Service struct {
	repo Repository
}

// NewService creates a new organization Service instance.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// CreateOrg provisions a new organization and assigns the creator as owner.
func (s *Service) CreateOrg(ctx context.Context, name string, creatorID uuid.UUID) (*Organization, error) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		return nil, apperror.NewAppError(400, apperror.ErrCodeInvalidRequest, "Organization name cannot be blank", nil)
	}

	org, err := s.repo.CreateOrganization(ctx, trimmedName, creatorID)
	if err != nil {
		return nil, fmt.Errorf("failed to create organization: %w", err)
	}
	return org, nil
}

// ListUserOrgs returns all organizations the specified user belongs to.
func (s *Service) ListUserOrgs(ctx context.Context, userID uuid.UUID) ([]*UserOrganization, error) {
	orgs, err := s.repo.ListUserOrganizations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user organizations: %w", err)
	}
	if orgs == nil {
		orgs = []*UserOrganization{}
	}
	return orgs, nil
}

// GetOrg retrieves an organization by ID.
func (s *Service) GetOrg(ctx context.Context, orgID uuid.UUID) (*Organization, error) {
	org, err := s.repo.GetOrganizationByID(ctx, orgID)
	if err != nil {
		if errors.Is(err, ErrOrgNotFound) {
			return nil, apperror.NewAppError(404, apperror.ErrCodeResourceNotFound, "Organization not found", err)
		}
		return nil, fmt.Errorf("failed to get organization: %w", err)
	}
	return org, nil
}

// AddMember adds an existing user to an organization with the specified role.
func (s *Service) AddMember(ctx context.Context, orgID, inviterID uuid.UUID, email, role string) (*Membership, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role != RoleAdmin && role != RoleMember {
		return nil, apperror.NewAppError(400, apperror.ErrCodeInvalidRequest, "Role must be 'admin' or 'member'", nil)
	}

	normEmail := strings.ToLower(strings.TrimSpace(email))
	if normEmail == "" {
		return nil, apperror.NewAppError(400, apperror.ErrCodeInvalidRequest, "Email is required", nil)
	}

	targetUser, err := s.repo.GetUserByEmail(ctx, normEmail)
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			return nil, apperror.NewAppError(404, apperror.ErrCodeResourceNotFound, "User with specified email not found", nil)
		}
		return nil, fmt.Errorf("failed to find user: %w", err)
	}

	membership, err := s.repo.AddMember(ctx, orgID, targetUser.ID, role)
	if err != nil {
		if errors.Is(err, ErrMembershipExists) {
			return nil, apperror.NewAppError(409, apperror.ErrCodeResourceConflict, "User is already a member of this organization", err)
		}
		return nil, fmt.Errorf("failed to add member: %w", err)
	}

	return membership, nil
}

// ListMembers lists all members of an organization.
func (s *Service) ListMembers(ctx context.Context, orgID uuid.UUID) ([]*MemberDetail, error) {
	members, err := s.repo.ListMembers(ctx, orgID)
	if err != nil {
		if errors.Is(err, ErrOrgNotFound) {
			return nil, apperror.NewAppError(404, apperror.ErrCodeResourceNotFound, "Organization not found", err)
		}
		return nil, fmt.Errorf("failed to list members: %w", err)
	}
	if members == nil {
		members = []*MemberDetail{}
	}
	return members, nil
}

// RemoveMember removes a user from an organization with last-owner safety.
func (s *Service) RemoveMember(ctx context.Context, orgID, operatorID, targetUserID uuid.UUID) error {
	operatorMem, err := s.repo.GetMembership(ctx, orgID, operatorID)
	if err != nil {
		return apperror.NewAppError(403, apperror.ErrCodeForbidden, "Operator is not a member of this organization", nil)
	}

	targetMem, err := s.repo.GetMembership(ctx, orgID, targetUserID)
	if err != nil {
		if errors.Is(err, ErrMembershipNotFound) {
			return apperror.NewAppError(404, apperror.ErrCodeResourceNotFound, "Membership not found", nil)
		}
		return fmt.Errorf("failed to check target membership: %w", err)
	}

	// Operators with 'admin' role cannot remove 'owner' or other 'admin' members
	if operatorMem.Role == RoleAdmin && (targetMem.Role == RoleOwner || targetMem.Role == RoleAdmin) && operatorID != targetUserID {
		return apperror.NewAppError(403, apperror.ErrCodeForbidden, "Admins cannot remove owners or fellow administrators", nil)
	}

	// Prevent removing the last owner
	if targetMem.Role == RoleOwner {
		ownersCount, err := s.repo.CountOwners(ctx, orgID)
		if err != nil {
			return fmt.Errorf("failed to count owners: %w", err)
		}
		if ownersCount <= 1 {
			return apperror.NewAppError(400, apperror.ErrCodeInvalidRequest, "Cannot remove the last owner of an organization", ErrCannotRemoveLastOwner)
		}
	}

	if err := s.repo.RemoveMember(ctx, orgID, targetUserID); err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}
	return nil
}
