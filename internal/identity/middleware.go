package identity

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"knowledgebase-ai/internal/apperror"
)

// Role weights for hierarchy comparison
var roleWeights = map[string]int{
	RoleMember: 1,
	RoleAdmin:  2,
	RoleOwner:  3,
}

// RequireOrgMembership verifies that the authenticated user belongs to the organization specified in the route param `:orgID`.
// If minRole is provided, it verifies the user's role meets or exceeds the required privilege level.
func RequireOrgMembership(repo Repository, minRole ...string) fiber.Handler {
	requiredMinRole := ""
	if len(minRole) > 0 {
		requiredMinRole = strings.ToLower(minRole[0])
	}

	return func(c *fiber.Ctx) error {
		userID, err := apperror.GetAuthenticatedUserID(c)
		if err != nil {
			return err
		}

		orgIDParam := c.Params("orgID")
		if orgIDParam == "" {
			return apperror.NewAppError(
				fiber.StatusBadRequest,
				apperror.ErrCodeInvalidRequest,
				"Missing organization ID parameter in route",
				nil,
			)
		}

		orgID, err := uuid.Parse(orgIDParam)
		if err != nil {
			return apperror.NewAppError(
				fiber.StatusBadRequest,
				apperror.ErrCodeInvalidRequest,
				"Invalid organization ID format (must be UUID)",
				err,
			)
		}

		// Verify membership directly against database repository
		membership, err := repo.GetMembership(c.Context(), orgID, userID)
		if err != nil {
			if errors.Is(err, ErrMembershipNotFound) {
				return apperror.NewAppError(
					fiber.StatusForbidden,
					apperror.ErrCodeForbidden,
					"You are not a member of this organization",
					nil,
				)
			}
			return err
		}

		// Check role privilege level if minRole constraint is specified
		if requiredMinRole != "" {
			userWeight := roleWeights[membership.Role]
			requiredWeight := roleWeights[requiredMinRole]

			if userWeight < requiredWeight {
				return apperror.NewAppError(
					fiber.StatusForbidden,
					apperror.ErrCodeForbidden,
					"Insufficient privileges for this organization action",
					nil,
				)
			}
		}

		// Set validated tenant context
		c.Locals("org_id", orgID)
		c.Locals("user_role", membership.Role)

		return c.Next()
	}
}

// GetValidatedOrgID retrieves the verified organization UUID from context.
func GetValidatedOrgID(c *fiber.Ctx) (uuid.UUID, error) {
	val := c.Locals("org_id")
	if val == nil {
		return uuid.Nil, apperror.NewAppError(fiber.StatusBadRequest, apperror.ErrCodeInvalidRequest, "Organization context missing", nil)
	}

	orgID, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, apperror.NewAppError(fiber.StatusBadRequest, apperror.ErrCodeInvalidRequest, "Invalid organization context", nil)
	}

	return orgID, nil
}
