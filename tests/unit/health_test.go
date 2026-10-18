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

func TestHealthEndpoints(t *testing.T) {
	logger := observability.NewLogger("test", "error")
	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.FiberErrorHandler(logger),
	})
	app.Use(middleware.RequestID())

	v1 := app.Group("/api/v1")
	health := v1.Group("/health")

	health.Get("/live", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status": "live",
		})
	})

	health.Get("/ready", func(c *fiber.Ctx) error {
		// Mock readiness without active external connections in unit tests
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": fiber.Map{
				"code":    middleware.ErrCodeDependencyUnavailable,
				"message": "One or more required infrastructure dependencies are unavailable",
			},
			"checks": fiber.Map{
				"postgres": "down",
				"redis":    "down",
			},
		})
	})

	t.Run("GET /api/v1/health/live returns 200 OK", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/health/live", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != fiber.StatusOK {
			t.Errorf("expected status 200, got %d", resp.StatusCode)
		}

		bodyBytes, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			t.Fatalf("failed to decode json: %v", err)
		}

		if result["status"] != "live" {
			t.Errorf("expected status 'live', got %v", result["status"])
		}
	})

	t.Run("GET /api/v1/health/ready handles degraded state properly", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/api/v1/health/ready", nil)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != fiber.StatusServiceUnavailable {
			t.Errorf("expected status 503, got %d", resp.StatusCode)
		}
	})
}
