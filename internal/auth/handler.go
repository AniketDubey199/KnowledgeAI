package auth

import (
	"time"

	"github.com/gofiber/fiber/v2"

	"knowledgebase-ai/internal/apperror"
)

// Handler exposes HTTP handlers for user authentication and session management.
type Handler struct {
	service         *Service
	refreshTokenTTL time.Duration
	isSecureCookie  bool
}

// NewHandler initializes a new auth Handler.
func NewHandler(service *Service, refreshTokenTTL time.Duration, isSecureCookie bool) *Handler {
	return &Handler{
		service:         service,
		refreshTokenTTL: refreshTokenTTL,
		isSecureCookie:  isSecureCookie,
	}
}

// RegisterRequest holds client input for user registration.
type RegisterRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"display_name"`
}

// LoginRequest holds client credentials.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register handles user signup: POST /api/v1/auth/register
func (h *Handler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return apperror.NewAppError(
			fiber.StatusBadRequest,
			apperror.ErrCodeInvalidRequest,
			"Malformed JSON request body",
			err,
		)
	}

	userResp, err := h.service.Register(c.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"user": userResp,
		},
	})
}

// Login authenticates a user, sets an HttpOnly refresh cookie, and returns a short-lived access JWT: POST /api/v1/auth/login
func (h *Handler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return apperror.NewAppError(
			fiber.StatusBadRequest,
			apperror.ErrCodeInvalidRequest,
			"Malformed JSON request body",
			err,
		)
	}

	loginResp, rawRefreshToken, err := h.service.Login(c.Context(), req.Email, req.Password)
	if err != nil {
		return err
	}

	// TDD §5.2: Set refresh token separately in an HttpOnly, Secure cookie. Never in JSON body.
	h.setRefreshCookie(c, rawRefreshToken, h.refreshTokenTTL)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"access_token": loginResp.AccessToken,
			"token_type":   loginResp.TokenType,
			"expires_in":   loginResp.ExpiresIn,
			"user":         loginResp.User,
		},
	})
}

// Refresh rotates the refresh token and returns a fresh access JWT: POST /api/v1/auth/refresh
func (h *Handler) Refresh(c *fiber.Ctx) error {
	rawRefreshToken := c.Cookies("refresh_token")
	if rawRefreshToken == "" {
		// Also allow fallback from X-Refresh-Token header for non-browser programmatic clients
		rawRefreshToken = c.Get("X-Refresh-Token")
	}

	if rawRefreshToken == "" {
		return apperror.NewAppError(
			fiber.StatusUnauthorized,
			apperror.ErrCodeInvalidCredentials,
			"Refresh token cookie or header missing",
			nil,
		)
	}

	tokenResp, newRawRefreshToken, err := h.service.Refresh(c.Context(), rawRefreshToken)
	if err != nil {
		// Clear cookie on invalid/revoked refresh attempt
		h.clearRefreshCookie(c)
		return err
	}

	// Update HttpOnly cookie with rotated token
	h.setRefreshCookie(c, newRawRefreshToken, h.refreshTokenTTL)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"access_token": tokenResp.AccessToken,
			"token_type":   tokenResp.TokenType,
			"expires_in":   tokenResp.ExpiresIn,
		},
	})
}

// Logout revokes the current session and clears the refresh cookie: POST /api/v1/auth/logout
func (h *Handler) Logout(c *fiber.Ctx) error {
	rawRefreshToken := c.Cookies("refresh_token")
	if rawRefreshToken == "" {
		rawRefreshToken = c.Get("X-Refresh-Token")
	}

	if rawRefreshToken != "" {
		_ = h.service.Logout(c.Context(), rawRefreshToken)
	}

	h.clearRefreshCookie(c)
	return c.SendStatus(fiber.StatusNoContent)
}

// Me returns the authenticated user profile and organizations: GET /api/v1/auth/me
func (h *Handler) Me(c *fiber.Ctx) error {
	userID, err := GetAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	profile, err := h.service.GetMe(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": profile,
	})
}

// RegisterRoutes mounts all authentication endpoints onto a Fiber router group.
func (h *Handler) RegisterRoutes(router fiber.Router, jwtAuthMiddleware fiber.Handler) {
	authGroup := router.Group("/auth")

	authGroup.Post("/register", h.Register)
	authGroup.Post("/login", h.Login)
	authGroup.Post("/refresh", h.Refresh)
	authGroup.Post("/logout", h.Logout)
	authGroup.Get("/me", jwtAuthMiddleware, h.Me)
}

func (h *Handler) setRefreshCookie(c *fiber.Ctx, token string, ttl time.Duration) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    token,
		Path:     "/api/v1/auth",
		Expires:  time.Now().Add(ttl),
		HTTPOnly: true,
		Secure:   h.isSecureCookie,
		SameSite: "Lax",
	})
}

func (h *Handler) clearRefreshCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     "refresh_token",
		Value:    "",
		Path:     "/api/v1/auth",
		Expires:  time.Now().Add(-24 * time.Hour),
		HTTPOnly: true,
		Secure:   h.isSecureCookie,
		SameSite: "Lax",
		MaxAge:   -1,
	})
}
