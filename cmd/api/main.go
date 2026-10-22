package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"knowledgebase-ai/internal/auth"
	"knowledgebase-ai/internal/config"
	"knowledgebase-ai/internal/identity"
	"knowledgebase-ai/internal/middleware"
	"knowledgebase-ai/internal/observability"
	"knowledgebase-ai/internal/platform/postgres"
	"knowledgebase-ai/internal/platform/redis"
)

func main() {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize structured logger
	logger := observability.NewLogger(cfg.AppEnv, cfg.LogLevel)
	logger.Info("Starting KnowledgeBase AI API service",
		slog.String("env", cfg.AppEnv),
		slog.String("addr", cfg.APIAddr),
	)

	// 3. Initialize background context for connections
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 4. Initialize PostgreSQL connection pool
	var pgClient *postgres.Client
	var identityRepo identity.Repository

	pgClient, err = postgres.NewClient(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Warn("Failed to establish immediate PostgreSQL connection pool; using in-memory identity repo fallback for development", slog.String("error", err.Error()))
		identityRepo = identity.NewMemoryRepository()
	} else {
		logger.Info("PostgreSQL connection pool established")
		defer pgClient.Close()
		identityRepo = identity.NewPostgresRepository(pgClient.Pool)
	}

	// 5. Initialize Redis client
	var redisClient *redis.Client
	redisClient, err = redis.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		logger.Warn("Failed to establish immediate Redis connection", slog.String("error", err.Error()))
	} else {
		logger.Info("Redis client connected")
		defer func() {
			if err := redisClient.Close(); err != nil {
				logger.Error("Error closing Redis client", slog.String("error", err.Error()))
			}
		}()
	}

	// 6. Initialize JWT Manager and Domain Services
	jwtManager := auth.NewJWTManager(cfg.JWTSigningKey, cfg.JWTIssuer, cfg.JWTAudience, cfg.JWTAccessTTL)
	authService := auth.NewService(identityRepo, jwtManager, cfg.RefreshTokenTTL)
	identityService := identity.NewService(identityRepo)

	isSecureCookie := cfg.AppEnv == "production"
	authHandler := auth.NewHandler(authService, cfg.RefreshTokenTTL, isSecureCookie)
	identityHandler := identity.NewHandler(identityService)

	jwtAuthMiddleware := auth.JWTMiddleware(jwtManager)

	// 7. Initialize Fiber application with custom error handler
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          middleware.FiberErrorHandler(logger),
		ReadTimeout:           15 * time.Second,
		WriteTimeout:          15 * time.Second,
		IdleTimeout:           60 * time.Second,
	})

	// 8. Register standard middlewares
	app.Use(recover.New())
	app.Use(cors.New())
	app.Use(middleware.RequestID())
	app.Use(middleware.HTTPLogger(logger))

	// 9. Register API v1 routes
	v1 := app.Group("/api/v1")
	setupHealthRoutes(v1, pgClient, redisClient)
	authHandler.RegisterRoutes(v1, jwtAuthMiddleware)
	identityHandler.RegisterRoutes(v1, jwtAuthMiddleware, identityRepo)

	// 10. Graceful shutdown handler
	shutdownDone := make(chan struct{})
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
		sig := <-sigChan
		logger.Info("Shutdown signal received", slog.String("signal", sig.String()))

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := app.ShutdownWithContext(shutdownCtx); err != nil {
			logger.Error("Fiber shutdown error", slog.String("error", err.Error()))
		}
		close(shutdownDone)
	}()

	// 11. Start HTTP listener
	logger.Info("HTTP server listening", slog.String("addr", cfg.APIAddr))
	if err := app.Listen(cfg.APIAddr); err != nil && !errors.Is(err, fiber.ErrBadGateway) {
		logger.Info("Server stopped accepting connections", slog.String("message", err.Error()))
	}

	<-shutdownDone
	logger.Info("KnowledgeBase AI API service cleanly stopped")
}

func setupHealthRoutes(router fiber.Router, pg *postgres.Client, rdb *redis.Client) {
	health := router.Group("/health")

	// Liveness probe: returns 200 as long as the process is alive
	health.Get("/live", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":    "live",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	})

	// Readiness probe: verifies PostgreSQL and Redis availability
	health.Get("/ready", func(c *fiber.Ctx) error {
		checks := fiber.Map{
			"postgres": "down",
			"redis":    "down",
		}

		allHealthy := true

		// Check PostgreSQL
		if pg != nil {
			checkCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := pg.Ping(checkCtx); err == nil {
				checks["postgres"] = "up"
			} else {
				allHealthy = false
			}
		} else {
			allHealthy = false
		}

		// Check Redis
		if rdb != nil {
			checkCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if err := rdb.Ping(checkCtx); err == nil {
				checks["redis"] = "up"
			} else {
				allHealthy = false
			}
		} else {
			allHealthy = false
		}

		if !allHealthy {
			reqID, _ := c.Locals("requestid").(string)
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error": fiber.Map{
					"code":       middleware.ErrCodeDependencyUnavailable,
					"message":    "One or more required infrastructure dependencies are unavailable",
					"request_id": reqID,
				},
				"checks": checks,
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":    "ready",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"checks":    checks,
		})
	})
}
