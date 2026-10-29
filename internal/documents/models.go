package documents

import (
	"time"

	"github.com/google/uuid"
)

// Document status lifecycle states defined in TDD §4 and §7
const (
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusCompleted  = "completed"
	StatusFailed     = "failed"
	StatusDeleting   = "deleting"
	StatusDeleted    = "deleted"
)

// Outbox event types
const (
	EventDocumentIngestionRequested = "document.ingestion.requested"
	EventDocumentDeletionRequested  = "document.deletion.requested"
)

// Allowed MIME types
const (
	MimePDF      = "application/pdf"
	MimePlainText = "text/plain"
	MimeMarkdown = "text/markdown"
)

// Document represents an uploaded document entity.
type Document struct {
	ID             uuid.UUID  `json:"id"`
	OrganizationID uuid.UUID  `json:"organization_id"`
	UploadedBy     uuid.UUID  `json:"uploaded_by"`
	Filename       string     `json:"filename"`
	ContentType    string     `json:"content_type"`
	SizeBytes      int64      `json:"size_bytes"`
	ChecksumSHA256 string     `json:"checksum_sha256"`
	StorageKey     string     `json:"-"` // Never exposed to clients
	Status         string     `json:"status"`
	ErrorCode      *string    `json:"error_code,omitempty"`
	ErrorMessage   *string    `json:"error_message,omitempty"`
	ChunkCount     int        `json:"chunk_count"`
	Version        int        `json:"version"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty"`
}

// IngestionJob tracks an asynchronous background parsing and embedding job.
type IngestionJob struct {
	ID             uuid.UUID `json:"id"`
	DocumentID     uuid.UUID `json:"document_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
	TaskID         string    `json:"task_id"`
	Status         string    `json:"status"`
	Attempts       int       `json:"attempts"`
	AvailableAt    time.Time `json:"available_at"`
	LastErrorCode  *string   `json:"last_error_code,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// OutboxEvent records events committed transactionally with state changes for guaranteed dispatch.
type OutboxEvent struct {
	ID          uuid.UUID  `json:"id"`
	EventType   string     `json:"event_type"`
	AggregateID uuid.UUID  `json:"aggregate_id"`
	Payload     []byte     `json:"payload"`
	CreatedAt   time.Time  `json:"created_at"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	Attempts    int        `json:"attempts"`
}

// IngestionTaskPayload specifies the versioned JSON payload stored in outbox events and Asynq tasks.
type IngestionTaskPayload struct {
	Version        int       `json:"version"`
	JobID          uuid.UUID `json:"job_id"`
	DocumentID     uuid.UUID `json:"document_id"`
	OrganizationID uuid.UUID `json:"organization_id"`
}

// UploadResponse returned on successful upload submission (202 Accepted).
type UploadResponse struct {
	DocumentID uuid.UUID `json:"document_id"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

// DocumentStatusResponse provides client-safe status details for GET /documents/{id}.
// Invariant: Storage keys, database internals, and raw provider credentials are never leaked.
type DocumentStatusResponse struct {
	ID         uuid.UUID  `json:"id"`
	Filename   string     `json:"filename"`
	Status     string     `json:"status"`
	ChunkCount int        `json:"chunk_count"`
	Error      *string    `json:"error"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// DocumentSummary provides metadata for lists of documents.
type DocumentSummary struct {
	ID          uuid.UUID `json:"id"`
	Filename    string    `json:"filename"`
	ContentType string    `json:"content_type"`
	SizeBytes   int64     `json:"size_bytes"`
	Status      string    `json:"status"`
	ChunkCount  int       `json:"chunk_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DeleteResponse returned when a document deletion is accepted (202 Accepted).
type DeleteResponse struct {
	DocumentID uuid.UUID `json:"document_id"`
	Status     string    `json:"status"`
}

// IdempotencyRecord stores cached responses for duplicate client requests.
type IdempotencyRecord struct {
	Key        string    `json:"key"`
	UserID     uuid.UUID `json:"user_id"`
	OrgID      uuid.UUID `json:"org_id"`
	DocumentID uuid.UUID `json:"document_id"`
	CreatedAt  time.Time `json:"created_at"`
}
