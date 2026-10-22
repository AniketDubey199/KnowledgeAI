package middleware

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"

	"knowledgebase-ai/internal/apperror"
)

// Re-export common error codes for backward compatibility
const (
	ErrCodeInvalidRequest        = apperror.ErrCodeInvalidRequest
	ErrCodeInvalidFileType       = apperror.ErrCodeInvalidFileType
	ErrCodeInvalidCredentials    = apperror.ErrCodeInvalidCredentials
	ErrCodeTokenExpired          = apperror.ErrCodeTokenExpired
	ErrCodeForbidden             = apperror.ErrCodeForbidden
	ErrCodeResourceNotFound      = apperror.ErrCodeResourceNotFound
	ErrCodeResourceConflict      = apperror.ErrCodeResourceConflict
	ErrCodeDuplicateRequest      = apperror.ErrCodeDuplicateRequest
	ErrCodeFileTooLarge          = apperror.ErrCodeFileTooLarge
	ErrCodeDocumentNotReady      = apperror.ErrCodeDocumentNotReady
	ErrCodeInsufficientEvidence  = apperror.ErrCodeInsufficientEvidence
	ErrCodeRateLimited           = apperror.ErrCodeRateLimited
	ErrCodeQuotaExceeded         = apperror.ErrCodeQuotaExceeded
	ErrCodeLLMProviderError      = apperror.ErrCodeLLMProviderError
	ErrCodeDependencyUnavailable = apperror.ErrCodeDependencyUnavailable
	ErrCodeUpstreamTimeout       = apperror.ErrCodeUpstreamTimeout
	ErrCodeInternalError         = apperror.ErrCodeInternalError
)

// Type aliases for convenience
type AppError = apperror.AppError
type ErrorResponse = apperror.ErrorResponse
type ErrorDetail = apperror.ErrorDetail

var NewAppError = apperror.NewAppError

// FiberErrorHandler catches all unhandled errors and formats them into the standard error envelope.
func FiberErrorHandler(logger *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		reqID, _ := c.Locals("requestid").(string)

		var appErr *apperror.AppError
		var fiberErr *fiber.Error

		statusCode := fiber.StatusInternalServerError
		code := apperror.ErrCodeInternalError
		message := "An internal server error occurred."

		if errors.As(err, &appErr) {
			statusCode = appErr.StatusCode
			code = appErr.Code
			message = appErr.Message
			if appErr.Internal != nil {
				logger.Error("Application error",
					slog.String("request_id", reqID),
					slog.String("code", code),
					slog.String("error", appErr.Internal.Error()),
					slog.String("path", c.Path()),
				)
			}
		} else if errors.As(err, &fiberErr) {
			statusCode = fiberErr.Code
			message = fiberErr.Message
			switch statusCode {
			case fiber.StatusBadRequest:
				code = apperror.ErrCodeInvalidRequest
			case fiber.StatusNotFound:
				code = apperror.ErrCodeResourceNotFound
			case fiber.StatusForbidden:
				code = apperror.ErrCodeForbidden
			case fiber.StatusUnauthorized:
				code = apperror.ErrCodeInvalidCredentials
			case fiber.StatusRequestEntityTooLarge:
				code = apperror.ErrCodeFileTooLarge
			case fiber.StatusTooManyRequests:
				code = apperror.ErrCodeRateLimited
			case fiber.StatusServiceUnavailable:
				code = apperror.ErrCodeDependencyUnavailable
			default:
				code = apperror.ErrCodeInternalError
			}
		} else {
			// Never expose raw SQL errors, stack traces or credentials to clients
			logger.Error("Unhandled internal error",
				slog.String("request_id", reqID),
				slog.String("error", err.Error()),
				slog.String("path", c.Path()),
			)
		}

		return c.Status(statusCode).JSON(apperror.ErrorResponse{
			Error: apperror.ErrorDetail{
				Code:      code,
				Message:   message,
				RequestID: reqID,
			},
		})
	}
}
