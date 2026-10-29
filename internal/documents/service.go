package documents

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"

	"knowledgebase-ai/internal/apperror"
	"knowledgebase-ai/internal/platform/storage"
)

// Service encapsulates document ingestion, validation, and lifecycle management.
type Service struct {
	repo           Repository
	storage        storage.Storage
	maxUploadBytes int64
}

// NewService initializes a new document Service.
func NewService(repo Repository, store storage.Storage, maxUploadBytes int64) *Service {
	if maxUploadBytes <= 0 {
		maxUploadBytes = 10 * 1024 * 1024 // 10MB default
	}
	return &Service{
		repo:           repo,
		storage:        store,
		maxUploadBytes: maxUploadBytes,
	}
}

// UploadParams contains all arguments required for processing an uploaded document stream.
type UploadParams struct {
	OrgID          uuid.UUID
	UserID         uuid.UUID
	Filename       string
	ContentType    string
	Size           int64
	Body           io.Reader
	IdempotencyKey string
}

// Upload handles streaming upload, content validation, SHA-256 hashing, storage persistence, and transactional outbox creation.
func (s *Service) Upload(ctx context.Context, params UploadParams) (*UploadResponse, error) {
	// 1. Idempotency Check: Return previously accepted response if key was already processed
	if params.IdempotencyKey != "" {
		existing, err := s.repo.GetIdempotencyRecord(ctx, params.IdempotencyKey, params.UserID, params.OrgID)
		if err == nil && existing != nil {
			doc, err := s.repo.GetDocumentByID(ctx, existing.DocumentID)
			if err == nil && doc != nil {
				return &UploadResponse{
					DocumentID: doc.ID,
					Status:     doc.Status,
					CreatedAt:  doc.CreatedAt,
				}, nil
			}
		}
	}

	// 2. Initial size check
	if params.Size > s.maxUploadBytes {
		return nil, apperror.NewAppError(
			413,
			apperror.ErrCodeFileTooLarge,
			fmt.Sprintf("File size %d bytes exceeds maximum limit of %d bytes", params.Size, s.maxUploadBytes),
			nil,
		)
	}

	// 3. Inspect the first 512 bytes to validate MIME type and magic numbers
	headerBuffer := make([]byte, 512)
	n, err := io.ReadFull(params.Body, headerBuffer)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) && !errors.Is(err, io.EOF) {
		return nil, apperror.NewAppError(400, apperror.ErrCodeInvalidRequest, "Failed to read file content", err)
	}
	headerBuffer = headerBuffer[:n]

	detectedMime, err := s.validateFileContent(params.Filename, params.ContentType, headerBuffer)
	if err != nil {
		return nil, err
	}

	// 4. Stream rest of file, computing SHA-256 and enforcing byte limits
	hasher := sha256.New()
	hasher.Write(headerBuffer)

	// Combine header with rest of body
	multiReader := io.MultiReader(bytes.NewReader(headerBuffer), params.Body)

	// Pipe to capture sha256 while streaming to storage
	docID := uuid.New()
	storageKey := fmt.Sprintf("%s/%s/%s", params.OrgID, docID, filepath.Base(params.Filename))

	pr, pw := io.Pipe()
	var totalBytes int64
	streamErrChan := make(chan error, 1)

	go func() {
		defer pw.Close()
		buf := make([]byte, 32*1024)
		for {
			readBytes, readErr := multiReader.Read(buf)
			if readBytes > 0 {
				totalBytes += int64(readBytes)
				if totalBytes > s.maxUploadBytes {
					_ = pw.CloseWithError(apperror.NewAppError(
						413,
						apperror.ErrCodeFileTooLarge,
						fmt.Sprintf("Streamed file size exceeds limit of %d bytes", s.maxUploadBytes),
						nil,
					))
					return
				}
				hasher.Write(buf[:readBytes])
				if _, writeErr := pw.Write(buf[:readBytes]); writeErr != nil {
					return
				}
			}
			if readErr != nil {
				if !errors.Is(readErr, io.EOF) {
					_ = pw.CloseWithError(readErr)
				}
				return
			}
		}
	}()

	// 5. Durably persist to storage
	saveErr := s.storage.Save(ctx, storageKey, pr)
	if saveErr != nil {
		return nil, apperror.NewAppError(500, apperror.ErrCodeInternalError, "Failed to write file to storage", saveErr)
	}

	select {
	case streamErr := <-streamErrChan:
		if streamErr != nil {
			_ = s.storage.Delete(ctx, storageKey) // Clean up orphan file
			return nil, streamErr
		}
	default:
	}

	checksumHex := hex.EncodeToString(hasher.Sum(nil))
	now := time.Now().UTC()

	// 6. Build Document, IngestionJob, and OutboxEvent entities
	doc := &Document{
		ID:             docID,
		OrganizationID: params.OrgID,
		UploadedBy:     params.UserID,
		Filename:       params.Filename,
		ContentType:    detectedMime,
		SizeBytes:      totalBytes,
		ChecksumSHA256: checksumHex,
		StorageKey:     storageKey,
		Status:         StatusPending,
		ChunkCount:     0,
		Version:        1,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	jobID := uuid.New()
	job := &IngestionJob{
		ID:             jobID,
		DocumentID:     docID,
		OrganizationID: params.OrgID,
		TaskID:         fmt.Sprintf("ingest:%s", docID),
		Status:         StatusPending,
		Attempts:       0,
		AvailableAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	taskPayload, _ := json.Marshal(IngestionTaskPayload{
		Version:        1,
		JobID:          jobID,
		DocumentID:     docID,
		OrganizationID: params.OrgID,
	})

	outboxEvent := &OutboxEvent{
		ID:          uuid.New(),
		EventType:   EventDocumentIngestionRequested,
		AggregateID: docID,
		Payload:     taskPayload,
		CreatedAt:   now,
		Attempts:    0,
	}

	// 7. Commit transactionally to PostgreSQL
	err = s.repo.CreateDocumentWithJobAndOutbox(ctx, doc, job, outboxEvent)
	if err != nil {
		// Clean up uploaded file from storage if DB transaction fails
		_ = s.storage.Delete(ctx, storageKey)

		if errors.Is(err, ErrDuplicateDocument) {
			return nil, apperror.NewAppError(
				409,
				apperror.ErrCodeResourceConflict,
				"A document with identical content has already been uploaded to this organization",
				err,
			)
		}
		return nil, fmt.Errorf("failed to commit document records: %w", err)
	}

	// 8. Record idempotency if requested
	if params.IdempotencyKey != "" {
		_ = s.repo.SaveIdempotencyRecord(ctx, &IdempotencyRecord{
			Key:        params.IdempotencyKey,
			UserID:     params.UserID,
			OrgID:      params.OrgID,
			DocumentID: docID,
			CreatedAt:  now,
		})
	}

	return &UploadResponse{
		DocumentID: doc.ID,
		Status:     doc.Status,
		CreatedAt:  doc.CreatedAt,
	}, nil
}

// GetDocument retrieves the document and ensures tenant isolation.
func (s *Service) GetDocument(ctx context.Context, docID, requestOrgID uuid.UUID) (*DocumentStatusResponse, error) {
	doc, err := s.repo.GetDocumentByID(ctx, docID)
	if err != nil {
		if errors.Is(err, ErrDocumentNotFound) {
			return nil, apperror.NewAppError(404, apperror.ErrCodeResourceNotFound, "Document not found", nil)
		}
		return nil, fmt.Errorf("failed to retrieve document: %w", err)
	}

	// Tenant Isolation: strictly reject cross-tenant access!
	if doc.OrganizationID != requestOrgID {
		return nil, apperror.NewAppError(403, apperror.ErrCodeForbidden, "Access to this document is forbidden", nil)
	}

	// If document has been soft-deleted, return 404
	if doc.Status == StatusDeleted {
		return nil, apperror.NewAppError(404, apperror.ErrCodeResourceNotFound, "Document has been deleted", nil)
	}

	return &DocumentStatusResponse{
		ID:         doc.ID,
		Filename:   doc.Filename,
		Status:     doc.Status,
		ChunkCount: doc.ChunkCount,
		Error:      doc.ErrorMessage,
		CreatedAt:  doc.CreatedAt,
		UpdatedAt:  doc.UpdatedAt,
	}, nil
}

// ListDocuments lists documents scoped strictly to an organization.
func (s *Service) ListDocuments(ctx context.Context, orgID uuid.UUID, limit, offset int, statusFilter string) ([]*DocumentSummary, int, error) {
	docs, total, err := s.repo.ListDocumentsByOrg(ctx, orgID, limit, offset, statusFilter)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list documents: %w", err)
	}

	summaries := make([]*DocumentSummary, 0, len(docs))
	for _, d := range docs {
		summaries = append(summaries, &DocumentSummary{
			ID:          d.ID,
			Filename:    d.Filename,
			ContentType: d.ContentType,
			SizeBytes:   d.SizeBytes,
			Status:      d.Status,
			ChunkCount:  d.ChunkCount,
			CreatedAt:   d.CreatedAt,
			UpdatedAt:   d.UpdatedAt,
		})
	}

	return summaries, total, nil
}

// DeleteDocument marks a document as deleting and dispatches a deletion event.
func (s *Service) DeleteDocument(ctx context.Context, docID, requestOrgID uuid.UUID) (*DeleteResponse, error) {
	doc, err := s.repo.GetDocumentByID(ctx, docID)
	if err != nil {
		if errors.Is(err, ErrDocumentNotFound) {
			return nil, apperror.NewAppError(404, apperror.ErrCodeResourceNotFound, "Document not found", nil)
		}
		return nil, err
	}

	// Tenant isolation check
	if doc.OrganizationID != requestOrgID {
		return nil, apperror.NewAppError(403, apperror.ErrCodeForbidden, "Access to this document is forbidden", nil)
	}

	if doc.Status == StatusDeleted {
		return &DeleteResponse{DocumentID: docID, Status: StatusDeleted}, nil
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"version":         1,
		"document_id":     docID,
		"organization_id": requestOrgID,
		"storage_key":     doc.StorageKey,
	})

	outboxEvent := &OutboxEvent{
		ID:          uuid.New(),
		EventType:   EventDocumentDeletionRequested,
		AggregateID: docID,
		Payload:     payload,
		CreatedAt:   time.Now().UTC(),
		Attempts:    0,
	}

	if err := s.repo.MarkDocumentDeleting(ctx, docID, outboxEvent); err != nil {
		return nil, fmt.Errorf("failed to mark document deleting: %w", err)
	}

	return &DeleteResponse{
		DocumentID: docID,
		Status:     StatusDeleting,
	}, nil
}

// RetryIngestion reschedules a failed ingestion job.
func (s *Service) RetryIngestion(ctx context.Context, docID, requestOrgID uuid.UUID) (*UploadResponse, error) {
	doc, err := s.repo.GetDocumentByID(ctx, docID)
	if err != nil {
		if errors.Is(err, ErrDocumentNotFound) {
			return nil, apperror.NewAppError(404, apperror.ErrCodeResourceNotFound, "Document not found", nil)
		}
		return nil, err
	}

	if doc.OrganizationID != requestOrgID {
		return nil, apperror.NewAppError(403, apperror.ErrCodeForbidden, "Access to this document is forbidden", nil)
	}

	if doc.Status != StatusFailed {
		return nil, apperror.NewAppError(
			400,
			apperror.ErrCodeInvalidRequest,
			fmt.Sprintf("Only failed documents can be retried; current status is '%s'", doc.Status),
			nil,
		)
	}

	now := time.Now().UTC()
	jobID := uuid.New()
	job := &IngestionJob{
		ID:             jobID,
		DocumentID:     docID,
		OrganizationID: requestOrgID,
		TaskID:         fmt.Sprintf("ingest:%s", docID),
		Status:         StatusPending,
		Attempts:       0,
		AvailableAt:    now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	taskPayload, _ := json.Marshal(IngestionTaskPayload{
		Version:        1,
		JobID:          jobID,
		DocumentID:     docID,
		OrganizationID: requestOrgID,
	})

	outboxEvent := &OutboxEvent{
		ID:          uuid.New(),
		EventType:   EventDocumentIngestionRequested,
		AggregateID: docID,
		Payload:     taskPayload,
		CreatedAt:   now,
		Attempts:    0,
	}

	if err := s.repo.RetryIngestion(ctx, docID, job, outboxEvent); err != nil {
		return nil, fmt.Errorf("failed to retry ingestion: %w", err)
	}

	return &UploadResponse{
		DocumentID: docID,
		Status:     StatusPending,
		CreatedAt:  now,
	}, nil
}

// validateFileContent validates MIME types and actual magic bytes.
func (s *Service) validateFileContent(filename, contentType string, header []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))

	// Validate PDF
	if ext == ".pdf" || contentType == MimePDF {
		if len(header) >= 4 && string(header[:4]) == "%PDF" {
			return MimePDF, nil
		}
		return "", apperror.NewAppError(400, apperror.ErrCodeInvalidFileType, "Invalid PDF: missing '%PDF' file signature", nil)
	}

	// Validate Markdown
	if ext == ".md" || ext == ".markdown" || contentType == MimeMarkdown || contentType == "text/x-markdown" {
		if utf8.Valid(header) {
			return MimeMarkdown, nil
		}
		return "", apperror.NewAppError(400, apperror.ErrCodeInvalidFileType, "Invalid Markdown file: content is not valid UTF-8 text", nil)
	}

	// Validate Plain Text
	if ext == ".txt" || contentType == MimePlainText {
		if utf8.Valid(header) {
			return MimePlainText, nil
		}
		return "", apperror.NewAppError(400, apperror.ErrCodeInvalidFileType, "Invalid plain text file: content is not valid UTF-8 text", nil)
	}

	return "", apperror.NewAppError(
		400,
		apperror.ErrCodeInvalidFileType,
		fmt.Sprintf("Unsupported file format '%s'. Only PDF, TXT, and Markdown files are supported.", ext),
		nil,
	)
}
