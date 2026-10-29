package documents

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresRepository implements Repository using PostgreSQL and pgxpool.Pool.
type PostgresRepository struct {
	pool *pgxpool.Pool

	// In-memory idempotency cache for fast lookup and deduplication
	idempMu      sync.RWMutex
	idempRecords map[string]*IdempotencyRecord
}

// NewPostgresRepository initializes a new PostgresRepository.
func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		pool:         pool,
		idempRecords: make(map[string]*IdempotencyRecord),
	}
}

// CreateDocumentWithJobAndOutbox executes a single atomic transaction creating the document, ingestion job, and outbox event.
func (r *PostgresRepository) CreateDocumentWithJobAndOutbox(ctx context.Context, doc *Document, job *IngestionJob, event *OutboxEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// 1. Check for duplicate active document with identical checksum in the organization
	var existingID uuid.UUID
	checkQuery := `
		SELECT id FROM documents
		WHERE organization_id = $1 AND checksum_sha256 = $2 AND status NOT IN ('deleted', 'deleting')
		LIMIT 1
	`
	err = tx.QueryRow(ctx, checkQuery, doc.OrganizationID, doc.ChecksumSHA256).Scan(&existingID)
	if err == nil {
		return ErrDuplicateDocument
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to check existing document checksum: %w", err)
	}

	// 2. Insert Document
	docQuery := `
		INSERT INTO documents (
			id, organization_id, uploaded_by, filename, content_type,
			size_bytes, checksum_sha256, storage_key, status,
			chunk_count, version, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9,
			$10, $11, $12, $13
		)
	`
	_, err = tx.Exec(ctx, docQuery,
		doc.ID, doc.OrganizationID, doc.UploadedBy, doc.Filename, doc.ContentType,
		doc.SizeBytes, doc.ChecksumSHA256, doc.StorageKey, doc.Status,
		doc.ChunkCount, doc.Version, doc.CreatedAt, doc.UpdatedAt,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			return ErrDuplicateDocument
		}
		return fmt.Errorf("failed to insert document: %w", err)
	}

	// 3. Insert Ingestion Job
	jobQuery := `
		INSERT INTO ingestion_jobs (
			id, document_id, organization_id, task_id, status,
			attempts, available_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9
		)
	`
	_, err = tx.Exec(ctx, jobQuery,
		job.ID, job.DocumentID, job.OrganizationID, job.TaskID, job.Status,
		job.Attempts, job.AvailableAt, job.CreatedAt, job.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to insert ingestion job: %w", err)
	}

	// 4. Insert Outbox Event
	outboxQuery := `
		INSERT INTO outbox_events (
			id, event_type, aggregate_id, payload, created_at, attempts
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
	`
	_, err = tx.Exec(ctx, outboxQuery,
		event.ID, event.EventType, event.AggregateID, event.Payload, event.CreatedAt, event.Attempts,
	)
	if err != nil {
		return fmt.Errorf("failed to insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit document ingestion transaction: %w", err)
	}

	return nil
}

// GetDocumentByID retrieves a document by ID.
func (r *PostgresRepository) GetDocumentByID(ctx context.Context, docID uuid.UUID) (*Document, error) {
	query := `
		SELECT
			id, organization_id, uploaded_by, filename, content_type,
			size_bytes, checksum_sha256, storage_key, status,
			error_code, error_message, chunk_count, version,
			created_at, updated_at, deleted_at
		FROM documents
		WHERE id = $1
	`
	doc := &Document{}
	err := r.pool.QueryRow(ctx, query, docID).Scan(
		&doc.ID, &doc.OrganizationID, &doc.UploadedBy, &doc.Filename, &doc.ContentType,
		&doc.SizeBytes, &doc.ChecksumSHA256, &doc.StorageKey, &doc.Status,
		&doc.ErrorCode, &doc.ErrorMessage, &doc.ChunkCount, &doc.Version,
		&doc.CreatedAt, &doc.UpdatedAt, &doc.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, fmt.Errorf("failed to query document by id: %w", err)
	}

	return doc, nil
}

// ListDocumentsByOrg retrieves non-deleted documents for an organization.
func (r *PostgresRepository) ListDocumentsByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int, statusFilter string) ([]*Document, int, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	countQuery := `SELECT COUNT(*) FROM documents WHERE organization_id = $1 AND status != 'deleted'`
	var total int
	if statusFilter != "" {
		countQuery += ` AND status = $2`
		err := r.pool.QueryRow(ctx, countQuery, orgID, statusFilter).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to count documents: %w", err)
		}
	} else {
		err := r.pool.QueryRow(ctx, countQuery, orgID).Scan(&total)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to count documents: %w", err)
		}
	}

	listQuery := `
		SELECT
			id, organization_id, uploaded_by, filename, content_type,
			size_bytes, checksum_sha256, storage_key, status,
			error_code, error_message, chunk_count, version,
			created_at, updated_at, deleted_at
		FROM documents
		WHERE organization_id = $1 AND status != 'deleted'
	`
	var rows pgx.Rows
	var err error
	if statusFilter != "" {
		listQuery += ` AND status = $2 ORDER BY created_at DESC LIMIT $3 OFFSET $4`
		rows, err = r.pool.Query(ctx, listQuery, orgID, statusFilter, limit, offset)
	} else {
		listQuery += ` ORDER BY created_at DESC LIMIT $2 OFFSET $3`
		rows, err = r.pool.Query(ctx, listQuery, orgID, limit, offset)
	}
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list documents: %w", err)
	}
	defer rows.Close()

	var docs []*Document
	for rows.Next() {
		doc := &Document{}
		err := rows.Scan(
			&doc.ID, &doc.OrganizationID, &doc.UploadedBy, &doc.Filename, &doc.ContentType,
			&doc.SizeBytes, &doc.ChecksumSHA256, &doc.StorageKey, &doc.Status,
			&doc.ErrorCode, &doc.ErrorMessage, &doc.ChunkCount, &doc.Version,
			&doc.CreatedAt, &doc.UpdatedAt, &doc.DeletedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan document row: %w", err)
		}
		docs = append(docs, doc)
	}

	return docs, total, nil
}

// MarkDocumentDeleting atomically flags the document as deleting and inserts a deletion outbox event.
func (r *PostgresRepository) MarkDocumentDeleting(ctx context.Context, docID uuid.UUID, outboxEvent *OutboxEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	now := time.Now().UTC()
	cmd, err := tx.Exec(ctx, `
		UPDATE documents
		SET status = 'deleting', updated_at = $2, deleted_at = $2
		WHERE id = $1 AND status != 'deleted'
	`, docID, now)
	if err != nil {
		return fmt.Errorf("failed to mark document deleting: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrDocumentNotFound
	}

	// Insert outbox event for cleanup
	if outboxEvent != nil {
		_, err = tx.Exec(ctx, `
			INSERT INTO outbox_events (
				id, event_type, aggregate_id, payload, created_at, attempts
			) VALUES (
				$1, $2, $3, $4, $5, $6
			)
		`, outboxEvent.ID, outboxEvent.EventType, outboxEvent.AggregateID, outboxEvent.Payload, outboxEvent.CreatedAt, outboxEvent.Attempts)
		if err != nil {
			return fmt.Errorf("failed to insert deletion outbox event: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// UpdateDocumentStatus updates status, errors, and increments version.
func (r *PostgresRepository) UpdateDocumentStatus(ctx context.Context, docID uuid.UUID, status string, errorCode, errorMessage *string) error {
	now := time.Now().UTC()
	query := `
		UPDATE documents
		SET status = $2, error_code = $3, error_message = $4, version = version + 1, updated_at = $5
		WHERE id = $1
	`
	cmd, err := r.pool.Exec(ctx, query, docID, status, errorCode, errorMessage, now)
	if err != nil {
		return fmt.Errorf("failed to update document status: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrDocumentNotFound
	}
	return nil
}

// RetryIngestion resets a failed document and enqueues a new outbox event.
func (r *PostgresRepository) RetryIngestion(ctx context.Context, docID uuid.UUID, job *IngestionJob, outboxEvent *OutboxEvent) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Verify document is in failed state
	var currentStatus string
	err = tx.QueryRow(ctx, `SELECT status FROM documents WHERE id = $1 FOR UPDATE`, docID).Scan(&currentStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrDocumentNotFound
		}
		return fmt.Errorf("failed to check document status: %w", err)
	}

	if currentStatus != StatusFailed {
		return ErrCannotRetry
	}

	now := time.Now().UTC()
	_, err = tx.Exec(ctx, `
		UPDATE documents
		SET status = 'pending', error_code = NULL, error_message = NULL, version = version + 1, updated_at = $2
		WHERE id = $1
	`, docID, now)
	if err != nil {
		return fmt.Errorf("failed to reset document for retry: %w", err)
	}

	// Insert or update ingestion job
	_, err = tx.Exec(ctx, `
		INSERT INTO ingestion_jobs (
			id, document_id, organization_id, task_id, status,
			attempts, available_at, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5,
			$6, $7, $8, $9
		)
		ON CONFLICT (task_id) DO UPDATE
		SET status = EXCLUDED.status, attempts = 0, available_at = EXCLUDED.available_at, updated_at = EXCLUDED.updated_at
	`, job.ID, job.DocumentID, job.OrganizationID, job.TaskID, job.Status, job.Attempts, job.AvailableAt, job.CreatedAt, job.UpdatedAt)
	if err != nil {
		return fmt.Errorf("failed to upsert ingestion job: %w", err)
	}

	// Insert outbox event
	_, err = tx.Exec(ctx, `
		INSERT INTO outbox_events (
			id, event_type, aggregate_id, payload, created_at, attempts
		) VALUES (
			$1, $2, $3, $4, $5, $6
		)
	`, outboxEvent.ID, outboxEvent.EventType, outboxEvent.AggregateID, outboxEvent.Payload, outboxEvent.CreatedAt, outboxEvent.Attempts)
	if err != nil {
		return fmt.Errorf("failed to insert retry outbox event: %w", err)
	}

	return tx.Commit(ctx)
}

// GetIdempotencyRecord retrieves an idempotency record.
func (r *PostgresRepository) GetIdempotencyRecord(ctx context.Context, key string, userID, orgID uuid.UUID) (*IdempotencyRecord, error) {
	compositeKey := fmt.Sprintf("%s:%s:%s", orgID, userID, key)
	r.idempMu.RLock()
	defer r.idempMu.RUnlock()

	rec, ok := r.idempRecords[compositeKey]
	if !ok {
		return nil, nil
	}
	return rec, nil
}

// SaveIdempotencyRecord saves an idempotency record.
func (r *PostgresRepository) SaveIdempotencyRecord(ctx context.Context, record *IdempotencyRecord) error {
	compositeKey := fmt.Sprintf("%s:%s:%s", record.OrgID, record.UserID, record.Key)
	r.idempMu.Lock()
	defer r.idempMu.Unlock()

	r.idempRecords[compositeKey] = record
	return nil
}
