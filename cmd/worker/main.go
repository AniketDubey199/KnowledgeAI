package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hibiken/asynq"

	"knowledgebase-ai/internal/config"
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
	logger.Info("Starting KnowledgeBase AI Asynq Worker service",
		slog.String("env", cfg.AppEnv),
		slog.Int("concurrency", cfg.WorkerConcurrency),
	)

	// 3. PostgreSQL connectivity verification
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pgClient, err := postgres.NewClient(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Warn("Worker running without active PostgreSQL connection", slog.String("error", err.Error()))
	} else {
		logger.Info("Worker PostgreSQL connection established")
		defer pgClient.Close()
	}

	// 4. Redis ping verification
	redisClient, err := redis.NewClient(ctx, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		logger.Warn("Worker Redis ping check warning", slog.String("error", err.Error()))
	} else {
		logger.Info("Worker Redis connectivity confirmed")
		defer func() { _ = redisClient.Close() }()
	}

	// 5. Asynq server configuration
	redisOpt := asynq.RedisClientOpt{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	}

	srv := asynq.NewServer(
		redisOpt,
		asynq.Config{
			Concurrency: cfg.WorkerConcurrency,
			Queues: map[string]int{
				"critical":  6,
				"default":   3,
				"low":       1,
			},
			ErrorHandler: asynq.ErrorHandlerFunc(func(ctx context.Context, task *asynq.Task, err error) {
				logger.Error("Asynq task processing failed",
					slog.String("task_type", task.Type()),
					slog.String("payload", string(task.Payload())),
					slog.String("error", err.Error()),
				)
			}),
		},
	)

	// 6. Router initialization
	mux := asynq.NewServeMux()

	// Initial placeholder task handler for health/ping jobs
	mux.HandleFunc("health:ping", func(ctx context.Context, t *asynq.Task) error {
		logger.Info("Received health:ping worker task", slog.String("payload", string(t.Payload())))
		return nil
	})

	// 7. Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		logger.Info("Shutdown signal received, gracefully terminating worker", slog.String("signal", sig.String()))
		srv.Shutdown()
	}()

	logger.Info("Worker is ready and listening for tasks")
	if err := srv.Run(mux); err != nil {
		logger.Error("Asynq worker stopped with error", slog.String("error", err.Error()))
		os.Exit(1)
	}

	// Brief cool down
	time.Sleep(100 * time.Millisecond)
	logger.Info("Asynq worker service cleanly terminated")
}
