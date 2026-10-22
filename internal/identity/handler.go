package identity

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"knowledgebase-ai/internal/apperror"
)

// Handler provides HTTP handlers for organization lifecycle and multi-tenant membership.
type Handler struct {
	service *Service
}

// NewHandler initializes a new identity Handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// CreateOrgRequest holds data for creating an organization.
type CreateOrgRequest struct {
	Name string `json:"name"`
}

// AddMemberRequest holds data for adding an organization member.
type AddMemberRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"`
}

// CreateOrganization handles POST /api/v1/organizations
func (h *Handler) CreateOrganization(c *fiber.Ctx) error {
	userID, err := apperror.GetAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	var req CreateOrgRequest
	if err := c.BodyParser(&req); err != nil {
		return apperror.NewAppError(
			fiber.StatusBadRequest,
			apperror.ErrCodeInvalidRequest,
			"Malformed JSON request body",
			err,
		)
	}

	org, err := h.service.CreateOrg(c.Context(), req.Name, userID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": fiber.Map{
			"id":   org.ID,
			"name": org.Name,
			"role": RoleOwner,
		},
	})
}

// ListOrganizations handles GET /api/v1/organizations
func (h *Handler) ListOrganizations(c *fiber.Ctx) error {
	userID, err := apperror.GetAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	orgs, err := h.service.ListUserOrgs(c.Context(), userID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": orgs,
	})
}

// GetOrganization handles GET /api/v1/organizations/:orgID
func (h *Handler) GetOrganization(c *fiber.Ctx) error {
	orgID, err := GetValidatedOrgID(c)
	if err != nil {
		return err
	}

	org, err := h.service.GetOrg(c.Context(), orgID)
	if err != nil {
		return err
	}

	role, _ := c.Locals("user_role").(string)

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": fiber.Map{
			"id":         org.ID,
			"name":       org.Name,
			"role":       role,
			"created_by": org.CreatedBy,
			"created_at": org.CreatedAt,
		},
	})
}

// AddMember handles POST /api/v1/organizations/:orgID/members
func (h *Handler) AddMember(c *fiber.Ctx) error {
	orgID, err := GetValidatedOrgID(c)
	if err != nil {
		return err
	}

	operatorID, err := apperror.GetAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	var req AddMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return apperror.NewAppError(
			fiber.StatusBadRequest,
			apperror.ErrCodeInvalidRequest,
			"Malformed JSON request body",
			err,
		)
	}

	membership, err := h.service.AddMember(c.Context(), orgID, operatorID, req.Email, req.Role)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": membership,
	})
}

// ListMembers handles GET /api/v1/organizations/:orgID/members
func (h *Handler) ListMembers(c *fiber.Ctx) error {
	orgID, err := GetValidatedOrgID(c)
	if err != nil {
		return err
	}

	members, err := h.service.ListMembers(c.Context(), orgID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": members,
	})
}

// RemoveMember handles DELETE /api/v1/organizations/:orgID/members/:userID
func (h *Handler) RemoveMember(c *fiber.Ctx) error {
	orgID, err := GetValidatedOrgID(c)
	if err != nil {
		return err
	}

	operatorID, err := apperror.GetAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	targetUserIDParam := c.Params("userID")
	targetUserID, err := uuid.Parse(targetUserIDParam)
	if err != nil {
		return apperror.NewAppError(
			fiber.StatusBadRequest,
			apperror.ErrCodeInvalidRequest,
			"Invalid user ID format in path",
			err,
		)
	}

	if err := h.service.RemoveMember(c.Context(), orgID, operatorID, targetUserID); err != nil {
		return err
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// RegisterRoutes mounts organization endpoints onto the provided Fiber router.
func (h *Handler) RegisterRoutes(
	router fiber.Router,
	jwtAuthMiddleware fiber.Handler,
	repo Repository,
) {
	orgs := router.Group("/organizations", jwtAuthMiddleware)

	// User org management
	orgs.Post("/", h.CreateOrganization)
	orgs.Get("/", h.ListOrganizations)

	// Scoped tenant endpoints (strictly protected by RequireOrgMembership)
	orgScoped := orgs.Group("/:orgID", RequireOrgMembership(repo))
	orgScoped.Get("/", h.GetOrganization)
	orgScoped.Get("/members", h.ListMembers)

	// Admin/Owner-only membership actions
	adminScoped := orgs.Group("/:orgID/members", RequireOrgMembership(repo, RoleAdmin))
	adminScoped.Post("/", h.AddMember)
	adminScoped.Delete("/:userID", h.RemoveMember)
}
