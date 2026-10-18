package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds all runtime configuration for the application and worker processes.
type Config struct {
	AppEnv   string
	APIAddr  string
	LogLevel string

	// PostgreSQL
	DatabaseURL string

	// Redis
	RedisAddr     string
	RedisPassword string

	// JWT & Authentication
	JWTIssuer       string
	JWTAudience     string
	JWTAccessTTL    time.Duration
	JWTSigningKey   string
	RefreshTokenTTL time.Duration

	// Gemini LLM & Embeddings
	GeminiAPIKey             string
	GeminiGenerationModel    string
	GeminiEmbeddingModel     string
	GeminiEmbeddingDimension int

	// File Storage
	StorageDriver    string
	LocalStoragePath string
	MaxUploadBytes   int64

	// Ingestion Worker
	IngestionMaxRetries int
	IngestionTimeout    time.Duration
	WorkerConcurrency   int
}

// Load reads configuration from environment variables and sets documented defaults.
func Load() (*Config, error) {
	cfg := &Config{
		AppEnv:                   getEnv("APP_ENV", "development"),
		APIAddr:                  getEnv("API_ADDR", ":8080"),
		LogLevel:                 getEnv("LOG_LEVEL", "debug"),
		DatabaseURL:              getEnv("DATABASE_URL", "postgres://kb_user:kb_password@localhost:5432/knowledgebase?sslmode=disable"),
		RedisAddr:                getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:            getEnv("REDIS_PASSWORD", ""),
		JWTIssuer:                getEnv("JWT_ISSUER", "knowledgebase-ai"),
		JWTAudience:              getEnv("JWT_AUDIENCE", "knowledgebase-api"),
		JWTAccessTTL:             getDurationEnv("JWT_ACCESS_TTL", 15*time.Minute),
		JWTSigningKey:            getEnv("JWT_SIGNING_KEY", "dev-insecure-secret-key-at-least-32-chars-long!"),
		RefreshTokenTTL:          getDurationEnv("REFRESH_TOKEN_TTL", 720*time.Hour),
		GeminiAPIKey:             getEnv("GEMINI_API_KEY", ""),
		GeminiGenerationModel:    getEnv("GEMINI_GENERATION_MODEL", "gemini-1.5-pro"),
		GeminiEmbeddingModel:     getEnv("GEMINI_EMBEDDING_MODEL", "gemini-embedding-001"),
		GeminiEmbeddingDimension: getIntEnv("GEMINI_EMBEDDING_DIMENSIONS", 768),
		StorageDriver:            getEnv("STORAGE_DRIVER", "local"),
		LocalStoragePath:         getEnv("LOCAL_STORAGE_PATH", "./uploads"),
		MaxUploadBytes:           getInt64Env("MAX_UPLOAD_BYTES", 10*1024*1024), // 10MB default
		IngestionMaxRetries:      getIntEnv("INGESTION_MAX_RETRIES", 5),
		IngestionTimeout:         getDurationEnv("INGESTION_TIMEOUT", 10*time.Minute),
		WorkerConcurrency:        getIntEnv("WORKER_CONCURRENCY", 4),
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// Validate checks that required fields meet operational constraints.
func (c *Config) Validate() error {
	if c.APIAddr == "" {
		return errors.New("API_ADDR cannot be empty")
	}
	if c.DatabaseURL == "" {
		return errors.New("DATABASE_URL cannot be empty")
	}
	if c.RedisAddr == "" {
		return errors.New("REDIS_ADDR cannot be empty")
	}
	if c.AppEnv == "production" {
		if c.JWTSigningKey == "" || len(c.JWTSigningKey) < 32 {
			return errors.New("JWT_SIGNING_KEY must be at least 32 characters in production")
		}
		if c.GeminiAPIKey == "" {
			return errors.New("GEMINI_API_KEY is required in production")
		}
	}
	if c.GeminiEmbeddingDimension <= 0 {
		return errors.New("GEMINI_EMBEDDING_DIMENSIONS must be greater than 0")
	}
	if c.MaxUploadBytes <= 0 {
		return errors.New("MAX_UPLOAD_BYTES must be greater than 0")
	}
	return nil
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func getIntEnv(key string, fallback int) int {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return fallback
}

func getInt64Env(key string, fallback int64) int64 {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if parsed, err := strconv.ParseInt(val, 10, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		if parsed, err := time.ParseDuration(val); err == nil {
			return parsed
		}
	}
	return fallback
}
