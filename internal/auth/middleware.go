package auth

import (
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"knowledgebase-ai/internal/apperror"
)

// JWTMiddleware enforces valid Bearer JWT authentication on protected routes.
func JWTMiddleware(jwtManager *JWTManager) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := strings.TrimSpace(c.Get("Authorization"))
		if authHeader == "" {
			return apperror.NewAppError(
				fiber.StatusUnauthorized,
				apperror.ErrCodeInvalidCredentials,
				"Authorization header is missing",
				nil,
			)
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			return apperror.NewAppError(
				fiber.StatusUnauthorized,
				apperror.ErrCodeInvalidCredentials,
				"Authorization header must use Bearer scheme",
				nil,
			)
		}

		tokenString := strings.TrimSpace(parts[1])
		if tokenString == "" {
			return apperror.NewAppError(
				fiber.StatusUnauthorized,
				apperror.ErrCodeInvalidCredentials,
				"Bearer token cannot be empty",
				nil,
			)
		}

		claims, err := jwtManager.ValidateAccessToken(tokenString)
		if err != nil {
			if errors.Is(err, ErrExpiredToken) {
				return apperror.NewAppError(
					fiber.StatusUnauthorized,
					apperror.ErrCodeTokenExpired,
					"Access token has expired",
					err,
				)
			}
			return apperror.NewAppError(
				fiber.StatusUnauthorized,
				apperror.ErrCodeInvalidCredentials,
				"Invalid access token",
				err,
			)
		}

		c.Locals("user_id", claims.UserID)
		c.Locals("user_email", claims.Email)

		return c.Next()
	}
}

// GetAuthenticatedUserID safely retrieves the authenticated user's UUID from Fiber context.
func GetAuthenticatedUserID(c *fiber.Ctx) (uuid.UUID, error) {
	val := c.Locals("user_id")
	if val == nil {
		return uuid.Nil, apperror.NewAppError(fiber.StatusUnauthorized, apperror.ErrCodeInvalidCredentials, "Authentication required", nil)
	}

	userID, ok := val.(uuid.UUID)
	if !ok {
		return uuid.Nil, apperror.NewAppError(fiber.StatusUnauthorized, apperror.ErrCodeInvalidCredentials, "Invalid authentication context", nil)
	}

	return userID, nil
}
