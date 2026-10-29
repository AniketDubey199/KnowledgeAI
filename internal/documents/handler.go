package documents

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"knowledgebase-ai/internal/apperror"
	"knowledgebase-ai/internal/identity"
)

// Handler exposes HTTP endpoints for document ingestion and lifecycle management.
type Handler struct {
	service      *Service
	identityRepo identity.Repository
}

// NewHandler initializes a new document Handler.
func NewHandler(service *Service, identityRepo identity.Repository) *Handler {
	return &Handler{
		service:      service,
		identityRepo: identityRepo,
	}
}

// Upload handles multipart document uploads: POST /api/v1/organizations/:orgID/documents
func (h *Handler) Upload(c *fiber.Ctx) error {
	orgID, err := identity.GetValidatedOrgID(c)
	if err != nil {
		return err
	}

	userID, err := apperror.GetAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return apperror.NewAppError(
			fiber.StatusBadRequest,
			apperror.ErrCodeInvalidRequest,
			"Missing multipart 'file' field in form data",
			err,
		)
	}

	file, err := fileHeader.Open()
	if err != nil {
		return apperror.NewAppError(
			fiber.StatusBadRequest,
			apperror.ErrCodeInvalidRequest,
			"Failed to open uploaded file",
			err,
		)
	}
	defer file.Close()

	idempotencyKey := c.Get("Idempotency-Key")

	params := UploadParams{
		OrgID:          orgID,
		UserID:         userID,
		Filename:       fileHeader.Filename,
		ContentType:    fileHeader.Header.Get("Content-Type"),
		Size:           fileHeader.Size,
		Body:           file,
		IdempotencyKey: idempotencyKey,
	}

	resp, err := h.service.Upload(c.Context(), params)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"data": resp,
	})
}

// GetStatus retrieves client-safe document status: GET /api/v1/documents/:documentID
func (h *Handler) GetStatus(c *fiber.Ctx) error {
	docIDParam := c.Params("documentID")
	docID, err := uuid.Parse(docIDParam)
	if err != nil {
		return apperror.NewAppError(
			fiber.StatusBadRequest,
			apperror.ErrCodeInvalidRequest,
			"Invalid document ID parameter (must be UUID)",
			err,
		)
	}

	userID, err := apperror.GetAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	// Fetch raw document to determine organization ownership
	rawDoc, err := h.service.repo.GetDocumentByID(c.Context(), docID)
	if err != nil {
		return apperror.NewAppError(fiber.StatusNotFound, apperror.ErrCodeResourceNotFound, "Document not found", nil)
	}

	// Verify authenticated user is a member of the document's organization
	_, err = h.identityRepo.GetMembership(c.Context(), rawDoc.OrganizationID, userID)
	if err != nil {
		return apperror.NewAppError(fiber.StatusForbidden, apperror.ErrCodeForbidden, "Access to this document is forbidden", nil)
	}

	resp, err := h.service.GetDocument(c.Context(), docID, rawDoc.OrganizationID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": resp,
	})
}

// List retrieves a paginated list of documents: GET /api/v1/organizations/:orgID/documents
func (h *Handler) List(c *fiber.Ctx) error {
	orgID, err := identity.GetValidatedOrgID(c)
	if err != nil {
		return err
	}

	limit := c.QueryInt("limit", 20)
	offset := c.QueryInt("offset", 0)
	statusFilter := c.Query("status", "")

	summaries, total, err := h.service.ListDocuments(c.Context(), orgID, limit, offset, statusFilter)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"data": summaries,
		"meta": fiber.Map{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// Delete marks a document as deleting: DELETE /api/v1/documents/:documentID
func (h *Handler) Delete(c *fiber.Ctx) error {
	docIDParam := c.Params("documentID")
	docID, err := uuid.Parse(docIDParam)
	if err != nil {
		return apperror.NewAppError(
			fiber.StatusBadRequest,
			apperror.ErrCodeInvalidRequest,
			"Invalid document ID parameter (must be UUID)",
			err,
		)
	}

	userID, err := apperror.GetAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	rawDoc, err := h.service.repo.GetDocumentByID(c.Context(), docID)
	if err != nil {
		return apperror.NewAppError(fiber.StatusNotFound, apperror.ErrCodeResourceNotFound, "Document not found", nil)
	}

	// Require membership in organization
	mem, err := h.identityRepo.GetMembership(c.Context(), rawDoc.OrganizationID, userID)
	if err != nil {
		return apperror.NewAppError(fiber.StatusForbidden, apperror.ErrCodeForbidden, "Access to this document is forbidden", nil)
	}

	// Only admin, owner, or the original uploader can delete
	if mem.Role != identity.RoleOwner && mem.Role != identity.RoleAdmin && rawDoc.UploadedBy != userID {
		return apperror.NewAppError(fiber.StatusForbidden, apperror.ErrCodeForbidden, "Only owners, admins, or the uploader can delete this document", nil)
	}

	resp, err := h.service.DeleteDocument(c.Context(), docID, rawDoc.OrganizationID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"data": resp,
	})
}

// Retry retries a failed document ingestion job: POST /api/v1/documents/:documentID/retry
func (h *Handler) Retry(c *fiber.Ctx) error {
	docIDParam := c.Params("documentID")
	docID, err := uuid.Parse(docIDParam)
	if err != nil {
		return apperror.NewAppError(
			fiber.StatusBadRequest,
			apperror.ErrCodeInvalidRequest,
			"Invalid document ID parameter (must be UUID)",
			err,
		)
	}

	userID, err := apperror.GetAuthenticatedUserID(c)
	if err != nil {
		return err
	}

	rawDoc, err := h.service.repo.GetDocumentByID(c.Context(), docID)
	if err != nil {
		return apperror.NewAppError(fiber.StatusNotFound, apperror.ErrCodeResourceNotFound, "Document not found", nil)
	}

	// Verify membership in organization
	_, err = h.identityRepo.GetMembership(c.Context(), rawDoc.OrganizationID, userID)
	if err != nil {
		return apperror.NewAppError(fiber.StatusForbidden, apperror.ErrCodeForbidden, "Access to this document is forbidden", nil)
	}

	resp, err := h.service.RetryIngestion(c.Context(), docID, rawDoc.OrganizationID)
	if err != nil {
		return err
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
		"data": resp,
	})
}

// RegisterRoutes mounts document endpoints onto the Fiber router.
func (h *Handler) RegisterRoutes(
	router fiber.Router,
	jwtAuthMiddleware fiber.Handler,
) {
	// Organization-scoped endpoints
	orgs := router.Group("/organizations/:orgID/documents", jwtAuthMiddleware, identity.RequireOrgMembership(h.identityRepo))
	orgs.Post("/", h.Upload)
	orgs.Get("/", h.List)

	// Document-scoped endpoints
	docs := router.Group("/documents/:documentID", jwtAuthMiddleware)
	docs.Get("/", h.GetStatus)
	docs.Delete("/", h.Delete)
	docs.Post("/retry", h.Retry)
}
