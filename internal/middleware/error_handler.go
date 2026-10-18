package middleware

import (
	"errors"
	"log/slog"

	"github.com/gofiber/fiber/v2"
)

// Standard error codes defined in TDD Section 5.1
const (
	ErrCodeInvalidRequest       = "INVALID_REQUEST"
	ErrCodeInvalidFileType      = "INVALID_FILE_TYPE"
	ErrCodeInvalidCredentials   = "INVALID_CREDENTIALS"
	ErrCodeTokenExpired         = "TOKEN_EXPIRED"
	ErrCodeForbidden            = "FORBIDDEN"
	ErrCodeResourceNotFound     = "RESOURCE_NOT_FOUND"
	ErrCodeResourceConflict     = "RESOURCE_CONFLICT"
	ErrCodeDuplicateRequest     = "DUPLICATE_REQUEST"
	ErrCodeFileTooLarge         = "FILE_TOO_LARGE"
	ErrCodeDocumentNotReady     = "DOCUMENT_NOT_READY"
	ErrCodeInsufficientEvidence = "INSUFFICIENT_EVIDENCE"
	ErrCodeRateLimited          = "RATE_LIMITED"
	ErrCodeQuotaExceeded        = "QUOTA_EXCEEDED"
	ErrCodeLLMProviderError     = "LLM_PROVIDER_ERROR"
	ErrCodeDependencyUnavailable = "DEPENDENCY_UNAVAILABLE"
	ErrCodeUpstreamTimeout      = "UPSTREAM_TIMEOUT"
	ErrCodeInternalError        = "INTERNAL_ERROR"
)

// ErrorDetail represents the nested error envelope specified by the TDD.
type ErrorDetail struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
}

// ErrorResponse represents the outer response body for all API errors.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// AppError is an explicit domain error with an HTTP status code and an API error code.
type AppError struct {
	StatusCode int
	Code       string
	Message    string
	Internal   error
}

func (e *AppError) Error() string {
	if e.Internal != nil {
		return e.Message + ": " + e.Internal.Error()
	}
	return e.Message
}

// NewAppError creates a new domain AppError.
func NewAppError(statusCode int, code, message string, internal error) *AppError {
	return &AppError{
		StatusCode: statusCode,
		Code:       code,
		Message:    message,
		Internal:   internal,
	}
}

// FiberErrorHandler catches all unhandled errors and formats them into the standard error envelope.
func FiberErrorHandler(logger *slog.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		reqID, _ := c.Locals("requestid").(string)

		var appErr *AppError
		var fiberErr *fiber.Error

		statusCode := fiber.StatusInternalServerError
		code := ErrCodeInternalError
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
				code = ErrCodeInvalidRequest
			case fiber.StatusNotFound:
				code = ErrCodeResourceNotFound
			case fiber.StatusForbidden:
				code = ErrCodeForbidden
			case fiber.StatusUnauthorized:
				code = ErrCodeInvalidCredentials
			case fiber.StatusRequestEntityTooLarge:
				code = ErrCodeFileTooLarge
			case fiber.StatusTooManyRequests:
				code = ErrCodeRateLimited
			case fiber.StatusServiceUnavailable:
				code = ErrCodeDependencyUnavailable
			default:
				code = ErrCodeInternalError
			}
		} else {
			// Never expose raw SQL errors, stack traces or credentials to clients
			logger.Error("Unhandled internal error",
				slog.String("request_id", reqID),
				slog.String("error", err.Error()),
				slog.String("path", c.Path()),
			)
		}

		return c.Status(statusCode).JSON(ErrorResponse{
			Error: ErrorDetail{
				Code:      code,
				Message:   message,
				RequestID: reqID,
			},
		})
	}
}
