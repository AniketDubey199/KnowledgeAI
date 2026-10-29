package documents

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

var (
	ErrDocumentNotFound   = errors.New("document not found")
	ErrDuplicateDocument  = errors.New("document with same checksum already exists in organization")
	ErrJobNotFound        = errors.New("ingestion job not found")
	ErrCannotRetry        = errors.New("only failed documents can be retried")
	ErrDocumentIsDeleting = errors.New("document is in deleting state")
)

// Repository defines the storage contract for documents, ingestion jobs, and transactional outbox events.
type Repository interface {
	// CreateDocumentWithJobAndOutbox executes a single atomic transaction creating:
	// 1. The document record
	// 2. The ingestion job record
	// 3. The outbox event record
	CreateDocumentWithJobAndOutbox(ctx context.Context, doc *Document, job *IngestionJob, event *OutboxEvent) error

	// GetDocumentByID retrieves a single document by its UUID.
	GetDocumentByID(ctx context.Context, docID uuid.UUID) (*Document, error)

	// ListDocumentsByOrg lists active documents for an organization, excluding soft-deleted ones.
	ListDocumentsByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int, statusFilter string) ([]*Document, int, error)

	// MarkDocumentDeleting flags the document as deleting and queues a deletion outbox event atomically.
	MarkDocumentDeleting(ctx context.Context, docID uuid.UUID, outboxEvent *OutboxEvent) error

	// UpdateDocumentStatus updates status, error information, and incremented version.
	UpdateDocumentStatus(ctx context.Context, docID uuid.UUID, status string, errorCode, errorMessage *string) error

	// RetryIngestion safely creates a new job attempt or resets the existing job for a failed document.
	RetryIngestion(ctx context.Context, docID uuid.UUID, job *IngestionJob, outboxEvent *OutboxEvent) error

	// Idempotency support
	GetIdempotencyRecord(ctx context.Context, key string, userID, orgID uuid.UUID) (*IdempotencyRecord, error)
	SaveIdempotencyRecord(ctx context.Context, record *IdempotencyRecord) error
}
