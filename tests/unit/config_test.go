package unit

import (
	"os"
	"testing"
	"time"

	"knowledgebase-ai/internal/config"
)

func TestConfigDefaults(t *testing.T) {
	// Clear any environment overrides
	os.Unsetenv("APP_ENV")
	os.Unsetenv("API_ADDR")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("expected config to load with defaults, got error: %v", err)
	}

	if cfg.AppEnv != "development" {
		t.Errorf("expected AppEnv 'development', got '%s'", cfg.AppEnv)
	}
	if cfg.APIAddr != ":8080" {
		t.Errorf("expected APIAddr ':8080', got '%s'", cfg.APIAddr)
	}
	if cfg.JWTAccessTTL != 15*time.Minute {
		t.Errorf("expected JWTAccessTTL 15m, got %v", cfg.JWTAccessTTL)
	}
	if cfg.RefreshTokenTTL != 720*time.Hour {
		t.Errorf("expected RefreshTokenTTL 720h, got %v", cfg.RefreshTokenTTL)
	}
	if cfg.GeminiEmbeddingDimension != 768 {
		t.Errorf("expected GeminiEmbeddingDimension 768, got %d", cfg.GeminiEmbeddingDimension)
	}
	if cfg.MaxUploadBytes != 10*1024*1024 {
		t.Errorf("expected MaxUploadBytes 10MB, got %d", cfg.MaxUploadBytes)
	}
}

func TestConfigProductionValidation(t *testing.T) {
	os.Setenv("APP_ENV", "production")
	os.Setenv("JWT_SIGNING_KEY", "short")
	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("JWT_SIGNING_KEY")
	}()

	_, err := config.Load()
	if err == nil {
		t.Fatalf("expected error when validating weak JWT_SIGNING_KEY in production, got nil")
	}
}
