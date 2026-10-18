package unit

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"

	"knowledgebase-ai/internal/middleware"
	"knowledgebase-ai/internal/observability"
)

func TestStandardErrorEnvelope(t *testing.T) {
	logger := observability.NewLogger("test", "error")
	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.FiberErrorHandler(logger),
	})

	app.Use(middleware.RequestID())

	// Route returning AppError
	app.Get("/test-app-error", func(c *fiber.Ctx) error {
		return middleware.NewAppError(
			fiber.StatusUnprocessableEntity,
			middleware.ErrCodeDocumentNotReady,
			"The document has not finished processing.",
			nil,
		)
	})

	// Route returning unhandled error
	app.Get("/test-unhandled-error", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusForbidden, "Access forbidden to this resource")
	})

	t.Run("AppError maps code, message, and request_id", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-app-error", nil)
		req.Header.Set("X-Request-ID", "req_custom_123")
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected request error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != fiber.StatusUnprocessableEntity {
			t.Errorf("expected 422, got %d", resp.StatusCode)
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		var errResp middleware.ErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
			t.Fatalf("failed to parse json error response: %v", err)
		}

		if errResp.Error.Code != middleware.ErrCodeDocumentNotReady {
			t.Errorf("expected code %s, got %s", middleware.ErrCodeDocumentNotReady, errResp.Error.Code)
		}
		if errResp.Error.Message != "The document has not finished processing." {
			t.Errorf("expected specific message, got %s", errResp.Error.Message)
		}
		if errResp.Error.RequestID != "req_custom_123" {
			t.Errorf("expected request_id 'req_custom_123', got '%s'", errResp.Error.RequestID)
		}
	})

	t.Run("Fiber error maps to standard envelope", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/test-unhandled-error", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected request error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != fiber.StatusForbidden {
			t.Errorf("expected 403, got %d", resp.StatusCode)
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		var errResp middleware.ErrorResponse
		if err := json.Unmarshal(bodyBytes, &errResp); err != nil {
			t.Fatalf("failed to parse json error response: %v", err)
		}

		if errResp.Error.Code != middleware.ErrCodeForbidden {
			t.Errorf("expected code %s, got %s", middleware.ErrCodeForbidden, errResp.Error.Code)
		}
	})
}
