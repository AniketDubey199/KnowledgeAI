package apperror

import (
	"errors"
)

// Standard error codes defined in TDD Section 5.1
const (
	ErrCodeInvalidRequest        = "INVALID_REQUEST"
	ErrCodeInvalidFileType       = "INVALID_FILE_TYPE"
	ErrCodeInvalidCredentials    = "INVALID_CREDENTIALS"
	ErrCodeTokenExpired          = "TOKEN_EXPIRED"
	ErrCodeForbidden             = "FORBIDDEN"
	ErrCodeResourceNotFound      = "RESOURCE_NOT_FOUND"
	ErrCodeResourceConflict      = "RESOURCE_CONFLICT"
	ErrCodeDuplicateRequest      = "DUPLICATE_REQUEST"
	ErrCodeFileTooLarge          = "FILE_TOO_LARGE"
	ErrCodeDocumentNotReady      = "DOCUMENT_NOT_READY"
	ErrCodeInsufficientEvidence  = "INSUFFICIENT_EVIDENCE"
	ErrCodeRateLimited           = "RATE_LIMITED"
	ErrCodeQuotaExceeded         = "QUOTA_EXCEEDED"
	ErrCodeLLMProviderError      = "LLM_PROVIDER_ERROR"
	ErrCodeDependencyUnavailable = "DEPENDENCY_UNAVAILABLE"
	ErrCodeUpstreamTimeout       = "UPSTREAM_TIMEOUT"
	ErrCodeInternalError         = "INTERNAL_ERROR"
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

// AsAppError attempts to unpack an AppError from an error chain.
func AsAppError(err error) (*AppError, bool) {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}
