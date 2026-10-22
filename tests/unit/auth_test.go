package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"

	"knowledgebase-ai/internal/auth"
	"knowledgebase-ai/internal/identity"
	"knowledgebase-ai/internal/middleware"
	"knowledgebase-ai/internal/observability"
)

func setupAuthTestApp() (*fiber.App, *auth.Service, identity.Repository, *auth.JWTManager) {
	logger := observability.NewLogger("test", "error")
	repo := identity.NewMemoryRepository()
	jwtManager := auth.NewJWTManager("test-super-secret-signing-key-32-chars!!", "kb-test", "kb-test-api", 15*time.Minute)
	authService := auth.NewService(repo, jwtManager, 720*time.Hour)
	authHandler := auth.NewHandler(authService, 720*time.Hour, false)

	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.FiberErrorHandler(logger),
	})
	app.Use(middleware.RequestID())

	v1 := app.Group("/api/v1")
	jwtAuthMiddleware := auth.JWTMiddleware(jwtManager)
	authHandler.RegisterRoutes(v1, jwtAuthMiddleware)

	return app, authService, repo, jwtManager
}

func TestPasswordHashing(t *testing.T) {
	password := "SecretPassword123!"
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	valid, err := auth.VerifyPassword(password, hash)
	if err != nil || !valid {
		t.Fatalf("expected password verification to succeed, got valid=%v, err=%v", valid, err)
	}

	invalid, err := auth.VerifyPassword("WrongPassword!", hash)
	if err != nil || invalid {
		t.Fatalf("expected incorrect password to fail, got valid=%v, err=%v", invalid, err)
	}
}

func TestJWTTokenIssuanceAndValidation(t *testing.T) {
	jwtManager := auth.NewJWTManager("test-key-must-be-at-least-32-bytes-long!", "issuer", "audience", 1*time.Second)
	_, _, repo, _ := setupAuthTestApp()
	ctx := context.Background()

	user, err := repo.CreateUser(ctx, "jwt@example.com", "hash", "JWT User")
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	token, exp, err := jwtManager.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		t.Fatalf("failed to generate access token: %v", err)
	}
	if token == "" || exp.Before(time.Now()) {
		t.Fatalf("invalid generated token")
	}

	claims, err := jwtManager.ValidateAccessToken(token)
	if err != nil {
		t.Fatalf("expected valid token validation: %v", err)
	}
	if claims.UserID != user.ID || claims.Email != user.Email {
		t.Fatalf("claims mismatch")
	}

	// Test tamper detection
	tamperedToken := token + "tampered"
	_, err = jwtManager.ValidateAccessToken(tamperedToken)
	if err == nil {
		t.Fatalf("expected error on tampered token, got nil")
	}

	// Test expiration
	shortLivedManager := auth.NewJWTManager("test-key-must-be-at-least-32-bytes-long!", "issuer", "audience", 50*time.Millisecond)
	expiredToken, _, _ := shortLivedManager.GenerateAccessToken(user.ID, user.Email)
	time.Sleep(100 * time.Millisecond)
	_, err = shortLivedManager.ValidateAccessToken(expiredToken)
	if err == nil {
		t.Fatalf("expected error on expired token, got nil")
	}
}

func TestAuthRegistrationAndLoginFlow(t *testing.T) {
	app, _, _, _ := setupAuthTestApp()

	// 1. Successful Registration
	regPayload := map[string]string{
		"email":        "Alice@Example.com", // should normalize to lowercase
		"password":     "password123",
		"display_name": "Alice Smith",
	}
	body, _ := json.Marshal(regPayload)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("register request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", resp.StatusCode)
	}

	// 2. Duplicate Registration Conflict (409)
	reqDup := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", bytes.NewReader(body))
	reqDup.Header.Set("Content-Type", "application/json")
	respDup, _ := app.Test(reqDup)
	defer respDup.Body.Close()

	if respDup.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict on duplicate registration, got %d", respDup.StatusCode)
	}

	// 3. Login with Correct Credentials
	loginPayload := map[string]string{
		"email":    "alice@example.com",
		"password": "password123",
	}
	loginBody, _ := json.Marshal(loginPayload)
	reqLogin := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", bytes.NewReader(loginBody))
	reqLogin.Header.Set("Content-Type", "application/json")

	respLogin, err := app.Test(reqLogin)
	if err != nil {
		t.Fatalf("login request error: %v", err)
	}
	defer respLogin.Body.Close()

	if respLogin.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on login, got %d", respLogin.StatusCode)
	}

	// Verify Refresh Token Cookie is set and HttpOnly
	cookies := respLogin.Cookies()
	var refreshCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "refresh_token" {
			refreshCookie = c
			break
		}
	}
	if refreshCookie == nil {
		t.Fatalf("expected refresh_token cookie to be set")
	}
	if !refreshCookie.HttpOnly {
		t.Fatalf("expected refresh_token cookie to be HttpOnly")
	}

	// Parse login response
	loginRespBytes, _ := io.ReadAll(respLogin.Body)
	var loginData struct {
		Data struct {
			AccessToken string `json:"access_token"`
			ExpiresIn   int64  `json:"expires_in"`
		} `json:"data"`
	}
	if err := json.Unmarshal(loginRespBytes, &loginData); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginData.Data.AccessToken == "" {
		t.Fatalf("access token is empty")
	}

	// 4. Test Authenticated /api/v1/auth/me
	reqMe := httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil)
	reqMe.Header.Set("Authorization", "Bearer "+loginData.Data.AccessToken)
	respMe, _ := app.Test(reqMe)
	defer respMe.Body.Close()

	if respMe.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on /me endpoint, got %d", respMe.StatusCode)
	}

	// 5. Test Token Rotation via /api/v1/auth/refresh
	reqRefresh := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	reqRefresh.AddCookie(refreshCookie)

	respRefresh, err := app.Test(reqRefresh)
	if err != nil {
		t.Fatalf("refresh request error: %v", err)
	}
	defer respRefresh.Body.Close()

	if respRefresh.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK on refresh, got %d", respRefresh.StatusCode)
	}

	// Check new rotated cookie
	var rotatedCookie *http.Cookie
	for _, c := range respRefresh.Cookies() {
		if c.Name == "refresh_token" {
			rotatedCookie = c
			break
		}
	}
	if rotatedCookie == nil || rotatedCookie.Value == refreshCookie.Value {
		t.Fatalf("expected rotated refresh token cookie")
	}

	// 6. Test REUSE DETECTION: Using the old, already-rotated refresh token MUST trigger family revocation!
	reqReused := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	reqReused.AddCookie(refreshCookie) // old token!
	respReused, _ := app.Test(reqReused)
	defer respReused.Body.Close()

	if respReused.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized on reused refresh token, got %d", respReused.StatusCode)
	}

	// 7. Verify that the newer token in the same family is now ALSO revoked due to detected reuse
	reqVictim := httptest.NewRequest(http.MethodPost, "/api/v1/auth/refresh", nil)
	reqVictim.AddCookie(rotatedCookie)
	respVictim, _ := app.Test(reqVictim)
	defer respVictim.Body.Close()

	if respVictim.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected rotated token to be revoked as well after reuse detection, got %d", respVictim.StatusCode)
	}
}
