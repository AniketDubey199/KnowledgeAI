package documents

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MemoryRepository implements Repository in memory for fast, deterministic unit testing.
type MemoryRepository struct {
	mu           sync.RWMutex
	documents    map[uuid.UUID]*Document
	jobs         map[uuid.UUID]*IngestionJob
	outboxEvents map[uuid.UUID]*OutboxEvent
	idempRecords map[string]*IdempotencyRecord
}

// NewMemoryRepository initializes a new MemoryRepository.
func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		documents:    make(map[uuid.UUID]*Document),
		jobs:         make(map[uuid.UUID]*IngestionJob),
		outboxEvents: make(map[uuid.UUID]*OutboxEvent),
		idempRecords: make(map[string]*IdempotencyRecord),
	}
}

func (m *MemoryRepository) CreateDocumentWithJobAndOutbox(ctx context.Context, doc *Document, job *IngestionJob, event *OutboxEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check checksum uniqueness within organization
	for _, existing := range m.documents {
		if existing.OrganizationID == doc.OrganizationID &&
			existing.ChecksumSHA256 == doc.ChecksumSHA256 &&
			existing.Status != StatusDeleted &&
			existing.Status != StatusDeleting {
			return ErrDuplicateDocument
		}
	}

	docCopy := *doc
	jobCopy := *job
	eventCopy := *event

	m.documents[doc.ID] = &docCopy
	m.jobs[job.ID] = &jobCopy
	m.outboxEvents[event.ID] = &eventCopy

	return nil
}

func (m *MemoryRepository) GetDocumentByID(ctx context.Context, docID uuid.UUID) (*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, ok := m.documents[docID]
	if !ok {
		return nil, ErrDocumentNotFound
	}
	docCopy := *doc
	return &docCopy, nil
}

func (m *MemoryRepository) ListDocumentsByOrg(ctx context.Context, orgID uuid.UUID, limit, offset int, statusFilter string) ([]*Document, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matching []*Document
	for _, doc := range m.documents {
		if doc.OrganizationID == orgID && doc.Status != StatusDeleted {
			if statusFilter == "" || doc.Status == statusFilter {
				docCopy := *doc
				matching = append(matching, &docCopy)
			}
		}
	}

	// Sort by created_at DESC
	sort.Slice(matching, func(i, j int) bool {
		return matching[i].CreatedAt.After(matching[j].CreatedAt)
	})

	total := len(matching)
	if offset >= total {
		return []*Document{}, total, nil
	}

	end := offset + limit
	if end > total {
		end = total
	}

	return matching[offset:end], total, nil
}

func (m *MemoryRepository) MarkDocumentDeleting(ctx context.Context, docID uuid.UUID, outboxEvent *OutboxEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, ok := m.documents[docID]
	if !ok || doc.Status == StatusDeleted {
		return ErrDocumentNotFound
	}

	now := time.Now().UTC()
	doc.Status = StatusDeleting
	doc.UpdatedAt = now
	doc.DeletedAt = &now

	if outboxEvent != nil {
		eventCopy := *outboxEvent
		m.outboxEvents[outboxEvent.ID] = &eventCopy
	}

	return nil
}

func (m *MemoryRepository) UpdateDocumentStatus(ctx context.Context, docID uuid.UUID, status string, errorCode, errorMessage *string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, ok := m.documents[docID]
	if !ok {
		return ErrDocumentNotFound
	}

	doc.Status = status
	doc.ErrorCode = errorCode
	doc.ErrorMessage = errorMessage
	doc.Version++
	doc.UpdatedAt = time.Now().UTC()

	return nil
}

func (m *MemoryRepository) RetryIngestion(ctx context.Context, docID uuid.UUID, job *IngestionJob, outboxEvent *OutboxEvent) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, ok := m.documents[docID]
	if !ok {
		return ErrDocumentNotFound
	}

	if doc.Status != StatusFailed {
		return ErrCannotRetry
	}

	doc.Status = StatusPending
	doc.ErrorCode = nil
	doc.ErrorMessage = nil
	doc.Version++
	doc.UpdatedAt = time.Now().UTC()

	jobCopy := *job
	m.jobs[job.ID] = &jobCopy

	eventCopy := *outboxEvent
	m.outboxEvents[outboxEvent.ID] = &eventCopy

	return nil
}

func (m *MemoryRepository) GetIdempotencyRecord(ctx context.Context, key string, userID, orgID uuid.UUID) (*IdempotencyRecord, error) {
	compositeKey := fmt.Sprintf("%s:%s:%s", orgID, userID, key)
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.idempRecords[compositeKey]
	if !ok {
		return nil, nil
	}
	recCopy := *rec
	return &recCopy, nil
}

func (m *MemoryRepository) SaveIdempotencyRecord(ctx context.Context, record *IdempotencyRecord) error {
	compositeKey := fmt.Sprintf("%s:%s:%s", record.OrgID, record.UserID, record.Key)
	m.mu.Lock()
	defer m.mu.Unlock()

	recCopy := *record
	m.idempRecords[compositeKey] = &recCopy
	return nil
}

// GetOutboxEvents returns all committed outbox events for assertions in tests.
func (m *MemoryRepository) GetOutboxEvents() []*OutboxEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var events []*OutboxEvent
	for _, e := range m.outboxEvents {
		eCopy := *e
		events = append(events, &eCopy)
	}
	return events
}
