package middleware

import (
	"log/slog"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// RequestID attaches an X-Request-ID to context and response headers.
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		reqID := c.Get("X-Request-ID")
		if reqID == "" {
			reqID = "req_" + uuid.New().String()
		}
		c.Locals("requestid", reqID)
		c.Set("X-Request-ID", reqID)
		return c.Next()
	}
}

// HTTPLogger logs HTTP requests with latency, status code, and request ID.
func HTTPLogger(logger *slog.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()
		latency := time.Since(start)

		reqID, _ := c.Locals("requestid").(string)
		status := c.Response().StatusCode()

		logger.Info("HTTP request",
			slog.String("request_id", reqID),
			slog.String("method", c.Method()),
			slog.String("path", c.Path()),
			slog.Int("status", status),
			slog.Duration("latency", latency),
			slog.String("ip", c.IP()),
		)

		return err
	}
}
