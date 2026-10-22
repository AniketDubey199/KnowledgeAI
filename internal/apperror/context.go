package apperror

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// GetAuthenticatedUserID extracts and validates the authenticated user's UUID from Fiber context locals.
func GetAuthenticatedUserID(c *fiber.Ctx) (uuid.UUID, error) {
	val := c.Locals("user_id")
	if val == nil {
		return uuid.Nil, NewAppError(
			fiber.StatusUnauthorized,
			ErrCodeInvalidCredentials,
			"Authentication required",
			nil,
		)
	}

	userID, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, NewAppError(
			fiber.StatusUnauthorized,
			ErrCodeInvalidCredentials,
			"Invalid authentication context",
			nil,
		)
	}

	return userID, nil
}
