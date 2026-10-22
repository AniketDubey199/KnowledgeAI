package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func setupTenancyTestApp() (*fiber.App, identity.Repository, *auth.JWTManager, *identity.Service) {
	logger := observability.NewLogger("test", "error")
	repo := identity.NewMemoryRepository()
	jwtManager := auth.NewJWTManager("test-super-secret-signing-key-32-chars!!", "kb-test", "kb-test-api", 15*time.Minute)
	identityService := identity.NewService(repo)
	identityHandler := identity.NewHandler(identityService)

	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.FiberErrorHandler(logger),
	})
	app.Use(middleware.RequestID())

	v1 := app.Group("/api/v1")
	jwtAuthMiddleware := auth.JWTMiddleware(jwtManager)
	identityHandler.RegisterRoutes(v1, jwtAuthMiddleware, repo)

	return app, repo, jwtManager, identityService
}

func TestMultiTenancyAndCrossTenantIsolation(t *testing.T) {
	app, repo, jwtManager, _ := setupTenancyTestApp()
	ctx := context.Background()

	// Provision User A
	userA, err := repo.CreateUser(ctx, "userA@example.com", "hashA", "User A")
	if err != nil {
		t.Fatalf("failed to create user A: %v", err)
	}
	tokenA, _, _ := jwtManager.GenerateAccessToken(userA.ID, userA.Email)

	// Provision User B
	userB, err := repo.CreateUser(ctx, "userB@example.com", "hashB", "User B")
	if err != nil {
		t.Fatalf("failed to create user B: %v", err)
	}
	tokenB, _, _ := jwtManager.GenerateAccessToken(userB.ID, userB.Email)

	// 1. User A creates Organization A
	createOrgPayload := map[string]string{"name": "Acme Corp"}
	bodyA, _ := json.Marshal(createOrgPayload)
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/organizations/", bytes.NewReader(bodyA))
	reqCreate.Header.Set("Authorization", "Bearer "+tokenA)
	reqCreate.Header.Set("Content-Type", "application/json")

	respCreate, err := app.Test(reqCreate)
	if err != nil {
		t.Fatalf("create org request error: %v", err)
	}
	defer respCreate.Body.Close()

	if respCreate.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created on org creation, got %d", respCreate.StatusCode)
	}

	var orgResp struct {
		Data struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Role string `json:"role"`
		} `json:"data"`
	}
	createBytes, _ := io.ReadAll(respCreate.Body)
	_ = json.Unmarshal(createBytes, &orgResp)
	orgAID := orgResp.Data.ID

	if orgResp.Data.Role != identity.RoleOwner {
		t.Fatalf("expected creator to have role owner, got %s", orgResp.Data.Role)
	}

	// 2. User A accesses Org A -> 200 OK
	reqGetA := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/organizations/%s", orgAID), nil)
	reqGetA.Header.Set("Authorization", "Bearer "+tokenA)
	respGetA, _ := app.Test(reqGetA)
	defer respGetA.Body.Close()

	if respGetA.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for member accessing org, got %d", respGetA.StatusCode)
	}

	// 3. CROSS-TENANT ISOLATION CHECK: User B attempts to access Org A -> 403 Forbidden!
	reqGetB := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/organizations/%s", orgAID), nil)
	reqGetB.Header.Set("Authorization", "Bearer "+tokenB)
	respGetB, _ := app.Test(reqGetB)
	defer respGetB.Body.Close()

	if respGetB.StatusCode != http.StatusForbidden {
		t.Fatalf("SECURITY VIOLATION: expected 403 Forbidden for cross-tenant access, got %d", respGetB.StatusCode)
	}

	// 4. User A (Owner) invites User B to Org A as 'member'
	addMemberPayload := map[string]string{
		"email": userB.Email,
		"role":  identity.RoleMember,
	}
	bodyAdd, _ := json.Marshal(addMemberPayload)
	reqAdd := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/organizations/%s/members", orgAID), bytes.NewReader(bodyAdd))
	reqAdd.Header.Set("Authorization", "Bearer "+tokenA)
	reqAdd.Header.Set("Content-Type", "application/json")

	respAdd, _ := app.Test(reqAdd)
	defer respAdd.Body.Close()

	if respAdd.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created on adding member, got %d", respAdd.StatusCode)
	}

	// 5. User B now accesses Org A -> 200 OK
	reqGetBNow := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/organizations/%s", orgAID), nil)
	reqGetBNow.Header.Set("Authorization", "Bearer "+tokenB)
	respGetBNow, _ := app.Test(reqGetBNow)
	defer respGetBNow.Body.Close()

	if respGetBNow.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for invited member, got %d", respGetBNow.StatusCode)
	}

	// 6. RBAC ENFORCEMENT: User B (role: member) attempts to add another member -> 403 Forbidden!
	userC, _ := repo.CreateUser(ctx, "userC@example.com", "hashC", "User C")
	addMemberPayloadC := map[string]string{
		"email": userC.Email,
		"role":  identity.RoleMember,
	}
	bodyAddC, _ := json.Marshal(addMemberPayloadC)
	reqAddByMember := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/organizations/%s/members", orgAID), bytes.NewReader(bodyAddC))
	reqAddByMember.Header.Set("Authorization", "Bearer "+tokenB)
	reqAddByMember.Header.Set("Content-Type", "application/json")

	respAddByMember, _ := app.Test(reqAddByMember)
	defer respAddByMember.Body.Close()

	if respAddByMember.StatusCode != http.StatusForbidden {
		t.Fatalf("RBAC VIOLATION: expected 403 Forbidden when member tries to invite, got %d", respAddByMember.StatusCode)
	}

	// 7. LAST OWNER PROTECTION: Attempting to remove the only owner (User A) must be blocked (400 Bad Request)
	reqRemoveOwner := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/organizations/%s/members/%s", orgAID, userA.ID), nil)
	reqRemoveOwner.Header.Set("Authorization", "Bearer "+tokenA)

	respRemoveOwner, _ := app.Test(reqRemoveOwner)
	defer respRemoveOwner.Body.Close()

	if respRemoveOwner.StatusCode != http.StatusBadRequest {
		t.Fatalf("INVARIANT VIOLATION: expected 400 Bad Request when removing last owner, got %d", respRemoveOwner.StatusCode)
	}
}
