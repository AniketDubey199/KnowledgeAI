package auth

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"

	"knowledgebase-ai/internal/apperror"
	"knowledgebase-ai/internal/identity"
)

// UserResponse represents safe user profile data returned to clients.
type UserResponse struct {
	ID          uuid.UUID `json:"id"`
	Email       string    `json:"email"`
	DisplayName string    `json:"display_name"`
	CreatedAt   time.Time `json:"created_at"`
}

// LoginResponse contains the access token, expiry, and user data upon successful login.
type LoginResponse struct {
	AccessToken  string        `json:"access_token"`
	TokenType    string        `json:"token_type"`
	ExpiresIn    int64         `json:"expires_in"` // seconds
	RefreshToken string        `json:"refresh_token"`
	User         *UserResponse `json:"user"`
}

// TokenResponse represents a refreshed access token payload.
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

// UserProfileResponse returns user identity plus organization memberships.
type UserProfileResponse struct {
	User          *UserResponse                `json:"user"`
	Organizations []*identity.UserOrganization `json:"organizations"`
}

// Service coordinates authentication workflows, password security, and token lifecycle.
type Service struct {
	repo            identity.Repository
	jwtManager      *JWTManager
	refreshTokenTTL time.Duration
}

// NewService creates a new Service instance.
func NewService(repo identity.Repository, jwtManager *JWTManager, refreshTokenTTL time.Duration) *Service {
	return &Service{
		repo:            repo,
		jwtManager:      jwtManager,
		refreshTokenTTL: refreshTokenTTL,
	}
}

// Register validates registration parameters, hashes the password, and provisions the user.
func (s *Service) Register(ctx context.Context, email, password, displayName string) (*UserResponse, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	if _, err := mail.ParseAddress(normEmail); err != nil {
		return nil, apperror.NewAppError(400, apperror.ErrCodeInvalidRequest, "Invalid email address format", err)
	}

	if len(password) < 8 {
		return nil, apperror.NewAppError(400, apperror.ErrCodeInvalidRequest, "Password must be at least 8 characters", nil)
	}
	if len(password) > 72 {
		return nil, apperror.NewAppError(400, apperror.ErrCodeInvalidRequest, "Password exceeds maximum permitted length", nil)
	}

	trimmedName := strings.TrimSpace(displayName)
	if trimmedName == "" {
		return nil, apperror.NewAppError(400, apperror.ErrCodeInvalidRequest, "Display name cannot be blank", nil)
	}

	hashedPassword, err := HashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	user, err := s.repo.CreateUser(ctx, normEmail, hashedPassword, trimmedName)
	if err != nil {
		if errors.Is(err, identity.ErrUserAlreadyExists) {
			return nil, apperror.NewAppError(409, apperror.ErrCodeResourceConflict, "Email already registered", err)
		}
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	return &UserResponse{
		ID:          user.ID,
		Email:       user.Email,
		DisplayName: user.DisplayName,
		CreatedAt:   user.CreatedAt,
	}, nil
}

// Login authenticates a user by email and password, returning tokens and initiating a refresh family.
func (s *Service) Login(ctx context.Context, email, password string) (*LoginResponse, string, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	if normEmail == "" || password == "" {
		return nil, "", apperror.NewAppError(400, apperror.ErrCodeInvalidRequest, "Email and password are required", nil)
	}

	user, err := s.repo.GetUserByEmail(ctx, normEmail)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			return nil, "", apperror.NewAppError(401, apperror.ErrCodeInvalidCredentials, "Invalid email or password", nil)
		}
		return nil, "", fmt.Errorf("failed to fetch user: %w", err)
	}

	valid, err := VerifyPassword(password, user.PasswordHash)
	if err != nil || !valid {
		return nil, "", apperror.NewAppError(401, apperror.ErrCodeInvalidCredentials, "Invalid email or password", nil)
	}

	// 1. Generate opaque refresh token string
	rawRefreshToken, err := GenerateSecureToken(32)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	tokenHash := HashToken(rawRefreshToken)
	familyID := uuid.New()
	sessionID := uuid.New()
	now := time.Now().UTC()
	expiresAt := now.Add(s.refreshTokenTTL)

	session := &identity.RefreshSession{
		ID:        sessionID,
		UserID:    user.ID,
		TokenHash: tokenHash,
		FamilyID:  familyID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}

	if err := s.repo.CreateRefreshSession(ctx, session); err != nil {
		return nil, "", fmt.Errorf("failed to persist refresh session: %w", err)
	}

	// 2. Generate short-lived JWT access token
	accessToken, exp, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate access token: %w", err)
	}

	expiresIn := int64(time.Until(exp).Seconds())

	resp := &LoginResponse{
		AccessToken:  accessToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		RefreshToken: rawRefreshToken,
		User: &UserResponse{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			CreatedAt:   user.CreatedAt,
		},
	}

	return resp, rawRefreshToken, nil
}

// Refresh handles token rotation and enforces token family reuse detection.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (*TokenResponse, string, error) {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" {
		return nil, "", apperror.NewAppError(401, apperror.ErrCodeInvalidCredentials, "Refresh token is missing", nil)
	}

	tokenHash := HashToken(rawRefreshToken)
	session, err := s.repo.GetRefreshSessionByHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, identity.ErrSessionNotFound) {
			return nil, "", apperror.NewAppError(401, apperror.ErrCodeInvalidCredentials, "Invalid refresh token", nil)
		}
		return nil, "", fmt.Errorf("failed to query refresh session: %w", err)
	}

	// REUSE DETECTION: If token was already revoked or replaced, revoke entire family!
	if session.RevokedAt != nil || session.ReplacedBy != nil {
		_ = s.repo.RevokeRefreshFamily(ctx, session.FamilyID)
		return nil, "", apperror.NewAppError(401, apperror.ErrCodeInvalidCredentials, "Refresh token reuse detected; all sessions in family revoked", nil)
	}

	// EXPIRY CHECK: If expired, revoke session and return error
	if time.Now().UTC().After(session.ExpiresAt) {
		_ = s.repo.RevokeRefreshSessionByHash(ctx, tokenHash)
		return nil, "", apperror.NewAppError(401, apperror.ErrCodeTokenExpired, "Refresh token has expired", nil)
	}

	// Valid session: fetch user for fresh token issuance
	user, err := s.repo.GetUserByID(ctx, session.UserID)
	if err != nil {
		return nil, "", apperror.NewAppError(401, apperror.ErrCodeInvalidCredentials, "User account no longer active", err)
	}

	// Generate replacement refresh token in the same family
	newRawToken, err := GenerateSecureToken(32)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate new refresh token: %w", err)
	}

	newTokenHash := HashToken(newRawToken)
	newSessionID := uuid.New()
	now := time.Now().UTC()
	newExpiresAt := now.Add(s.refreshTokenTTL)

	newSession := &identity.RefreshSession{
		ID:        newSessionID,
		UserID:    session.UserID,
		TokenHash: newTokenHash,
		FamilyID:  session.FamilyID,
		ExpiresAt: newExpiresAt,
		CreatedAt: now,
	}

	if err := s.repo.RotateRefreshSession(ctx, session.ID, newSession); err != nil {
		return nil, "", fmt.Errorf("failed to rotate refresh session: %w", err)
	}

	// Mint new JWT access token
	newAccessToken, exp, err := s.jwtManager.GenerateAccessToken(user.ID, user.Email)
	if err != nil {
		return nil, "", fmt.Errorf("failed to issue access token: %w", err)
	}

	expiresIn := int64(time.Until(exp).Seconds())

	resp := &TokenResponse{
		AccessToken:  newAccessToken,
		TokenType:    "Bearer",
		ExpiresIn:    expiresIn,
		RefreshToken: newRawToken,
	}

	return resp, newRawToken, nil
}

// Logout revokes the session associated with the provided refresh token.
func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	rawRefreshToken = strings.TrimSpace(rawRefreshToken)
	if rawRefreshToken == "" {
		return nil
	}
	tokenHash := HashToken(rawRefreshToken)
	return s.repo.RevokeRefreshSessionByHash(ctx, tokenHash)
}

// GetMe retrieves the current user's details and active memberships.
func (s *Service) GetMe(ctx context.Context, userID uuid.UUID) (*UserProfileResponse, error) {
	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil {
		if errors.Is(err, identity.ErrUserNotFound) {
			return nil, apperror.NewAppError(404, apperror.ErrCodeResourceNotFound, "User not found", nil)
		}
		return nil, fmt.Errorf("failed to retrieve user: %w", err)
	}

	orgs, err := s.repo.ListUserOrganizations(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list user organizations: %w", err)
	}
	if orgs == nil {
		orgs = []*identity.UserOrganization{}
	}

	return &UserProfileResponse{
		User: &UserResponse{
			ID:          user.ID,
			Email:       user.Email,
			DisplayName: user.DisplayName,
			CreatedAt:   user.CreatedAt,
		},
		Organizations: orgs,
	}, nil
}
