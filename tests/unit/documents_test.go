package unit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"

	"knowledgebase-ai/internal/auth"
	"knowledgebase-ai/internal/documents"
	"knowledgebase-ai/internal/identity"
	"knowledgebase-ai/internal/middleware"
	"knowledgebase-ai/internal/observability"
	"knowledgebase-ai/internal/platform/storage"
)

func setupDocumentsTestApp() (
	*fiber.App,
	identity.Repository,
	*documents.MemoryRepository,
	*storage.MemoryStorage,
	*auth.JWTManager,
) {
	logger := observability.NewLogger("test", "error")
	idRepo := identity.NewMemoryRepository()
	docRepo := documents.NewMemoryRepository()
	memStore := storage.NewMemoryStorage()

	jwtManager := auth.NewJWTManager("test-super-secret-signing-key-32-chars!!", "kb-test", "kb-test-api", 15*time.Minute)
	identityService := identity.NewService(idRepo)
	identityHandler := identity.NewHandler(identityService)

	docService := documents.NewService(docRepo, memStore, 1024*1024) // 1MB limit for testing
	docHandler := documents.NewHandler(docService, idRepo)

	app := fiber.New(fiber.Config{
		ErrorHandler: middleware.FiberErrorHandler(logger),
	})
	app.Use(middleware.RequestID())

	v1 := app.Group("/api/v1")
	jwtAuthMiddleware := auth.JWTMiddleware(jwtManager)
	identityHandler.RegisterRoutes(v1, jwtAuthMiddleware, idRepo)
	docHandler.RegisterRoutes(v1, jwtAuthMiddleware)

	return app, idRepo, docRepo, memStore, jwtManager
}

func createMultipartRequest(url, fieldName, filename string, content []byte, headers map[string]string) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(content); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	req := httptest.NewRequest(http.MethodPost, url, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

func TestDocumentUploadSuccess(t *testing.T) {
	app, idRepo, docRepo, memStore, jwtManager := setupDocumentsTestApp()
	ctx := context.Background()

	// Provision user and org
	user, _ := idRepo.CreateUser(ctx, "uploader@example.com", "hash", "Uploader")
	token, _, _ := jwtManager.GenerateAccessToken(user.ID, user.Email)
	org, _ := idRepo.CreateOrganization(ctx, "Engineering Org", user.ID)

	pdfContent := []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n<<>>\n%%EOF")

	req, err := createMultipartRequest(
		fmt.Sprintf("/api/v1/organizations/%s/documents/", org.ID),
		"file",
		"handbook.pdf",
		pdfContent,
		map[string]string{
			"Authorization": "Bearer " + token,
		},
	)
	if err != nil {
		t.Fatalf("failed to create upload request: %v", err)
	}

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("upload request error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202 Accepted, got %d: %s", resp.StatusCode, string(body))
	}

	var uploadResp struct {
		Data struct {
			DocumentID string `json:"document_id"`
			Status     string `json:"status"`
			CreatedAt  string `json:"created_at"`
		} `json:"data"`
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(bodyBytes, &uploadResp); err != nil {
		t.Fatalf("failed to decode upload response: %v", err)
	}

	docID, err := uuid.Parse(uploadResp.Data.DocumentID)
	if err != nil {
		t.Fatalf("invalid returned document UUID: %v", err)
	}
	if uploadResp.Data.Status != documents.StatusPending {
		t.Errorf("expected status 'pending', got %s", uploadResp.Data.Status)
	}

	// Verify persistence in repository
	doc, err := docRepo.GetDocumentByID(ctx, docID)
	if err != nil {
		t.Fatalf("document not persisted in repo: %v", err)
	}
	if doc.Filename != "handbook.pdf" {
		t.Errorf("expected filename 'handbook.pdf', got %s", doc.Filename)
	}
	if doc.ContentType != documents.MimePDF {
		t.Errorf("expected content type %s, got %s", documents.MimePDF, doc.ContentType)
	}

	// Verify file saved in storage
	exists, err := memStore.Exists(ctx, doc.StorageKey)
	if err != nil || !exists {
		t.Errorf("storage key %s was not found in storage driver", doc.StorageKey)
	}

	// Verify outbox event created
	events := docRepo.GetOutboxEvents()
	if len(events) == 0 {
		t.Fatalf("expected outbox event to be created transactionally")
	}
	if events[0].EventType != documents.EventDocumentIngestionRequested {
		t.Errorf("expected event type %s, got %s", documents.EventDocumentIngestionRequested, events[0].EventType)
	}
}

func TestDocumentUploadValidation(t *testing.T) {
	app, idRepo, _, _, jwtManager := setupDocumentsTestApp()
	ctx := context.Background()

	user, _ := idRepo.CreateUser(ctx, "uploader2@example.com", "hash", "Uploader 2")
	token, _, _ := jwtManager.GenerateAccessToken(user.ID, user.Email)
	org, _ := idRepo.CreateOrganization(ctx, "Tech Org", user.ID)

	t.Run("Rejects invalid file extension (.exe)", func(t *testing.T) {
		req, _ := createMultipartRequest(
			fmt.Sprintf("/api/v1/organizations/%s/documents/", org.ID),
			"file",
			"malicious.exe",
			[]byte("MZ executable binary data"),
			map[string]string{"Authorization": "Bearer " + token},
		)
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for unsupported file format, got %d", resp.StatusCode)
		}
	})

	t.Run("Rejects fake PDF with missing magic header", func(t *testing.T) {
		req, _ := createMultipartRequest(
			fmt.Sprintf("/api/v1/organizations/%s/documents/", org.ID),
			"file",
			"fake.pdf",
			[]byte("NOT A VALID PDF FILE SIGNATURE"),
			map[string]string{"Authorization": "Bearer " + token},
		)
		resp, _ := app.Test(req)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("expected 400 Bad Request for spoofed PDF, got %d", resp.StatusCode)
		}
	})

	t.Run("Accepts valid Markdown and Text files", func(t *testing.T) {
		reqMD, _ := createMultipartRequest(
			fmt.Sprintf("/api/v1/organizations/%s/documents/", org.ID),
			"file",
			"guide.md",
			[]byte("# System Documentation\n\nThis is valid markdown."),
			map[string]string{"Authorization": "Bearer " + token},
		)
		respMD, _ := app.Test(reqMD)
		defer respMD.Body.Close()

		if respMD.StatusCode != http.StatusAccepted {
			t.Errorf("expected 202 Accepted for markdown file, got %d", respMD.StatusCode)
		}

		reqTXT, _ := createMultipartRequest(
			fmt.Sprintf("/api/v1/organizations/%s/documents/", org.ID),
			"file",
			"notes.txt",
			[]byte("Plain text note content."),
			map[string]string{"Authorization": "Bearer " + token},
		)
		respTXT, _ := app.Test(reqTXT)
		defer respTXT.Body.Close()

		if respTXT.StatusCode != http.StatusAccepted {
			t.Errorf("expected 202 Accepted for plain text file, got %d", respTXT.StatusCode)
		}
	})
}

func TestDuplicateUploadAndIdempotency(t *testing.T) {
	app, idRepo, _, _, jwtManager := setupDocumentsTestApp()
	ctx := context.Background()

	user, _ := idRepo.CreateUser(ctx, "idemp@example.com", "hash", "Idemp User")
	token, _, _ := jwtManager.GenerateAccessToken(user.ID, user.Email)
	org, _ := idRepo.CreateOrganization(ctx, "Deduplication Org", user.ID)

	content := []byte("%PDF-1.4\nUnique Content For Deduplication Test\n%%EOF")

	// 1. First upload with Idempotency-Key
	req1, _ := createMultipartRequest(
		fmt.Sprintf("/api/v1/organizations/%s/documents/", org.ID),
		"file",
		"doc1.pdf",
		content,
		map[string]string{
			"Authorization":   "Bearer " + token,
			"Idempotency-Key": "unique-client-key-12345",
		},
	)
	resp1, _ := app.Test(req1)
	defer resp1.Body.Close()

	if resp1.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 on initial upload, got %d", resp1.StatusCode)
	}

	var data1 struct {
		Data struct {
			DocumentID string `json:"document_id"`
		} `json:"data"`
	}
	b1, _ := io.ReadAll(resp1.Body)
	_ = json.Unmarshal(b1, &data1)

	// 2. Exact same upload with same Idempotency-Key -> returns same response idempotently
	req2, _ := createMultipartRequest(
		fmt.Sprintf("/api/v1/organizations/%s/documents/", org.ID),
		"file",
		"doc1.pdf",
		content,
		map[string]string{
			"Authorization":   "Bearer " + token,
			"Idempotency-Key": "unique-client-key-12345",
		},
	)
	resp2, _ := app.Test(req2)
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 on idempotent repeat, got %d", resp2.StatusCode)
	}
	var data2 struct {
		Data struct {
			DocumentID string `json:"document_id"`
		} `json:"data"`
	}
	b2, _ := io.ReadAll(resp2.Body)
	_ = json.Unmarshal(b2, &data2)

	if data1.Data.DocumentID != data2.Data.DocumentID {
		t.Errorf("expected idempotent repeat to return same document ID %s, got %s", data1.Data.DocumentID, data2.Data.DocumentID)
	}

	// 3. Different request WITHOUT idempotency key but IDENTICAL content -> 409 Conflict
	req3, _ := createMultipartRequest(
		fmt.Sprintf("/api/v1/organizations/%s/documents/", org.ID),
		"file",
		"doc_duplicate.pdf",
		content,
		map[string]string{
			"Authorization": "Bearer " + token,
		},
	)
	resp3, _ := app.Test(req3)
	defer resp3.Body.Close()

	if resp3.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 Conflict for duplicate document checksum, got %d", resp3.StatusCode)
	}
}

func TestDocumentStatusAndTenantIsolation(t *testing.T) {
	app, idRepo, _, _, jwtManager := setupDocumentsTestApp()
	ctx := context.Background()

	// Provision User A & Org A
	userA, _ := idRepo.CreateUser(ctx, "usera@example.com", "hash", "User A")
	tokenA, _, _ := jwtManager.GenerateAccessToken(userA.ID, userA.Email)
	orgA, _ := idRepo.CreateOrganization(ctx, "Org A", userA.ID)

	// Provision User B & Org B
	userB, _ := idRepo.CreateUser(ctx, "userb@example.com", "hash", "User B")
	tokenB, _, _ := jwtManager.GenerateAccessToken(userB.ID, userB.Email)
	orgB, _ := idRepo.CreateOrganization(ctx, "Org B", userB.ID)

	// Upload document to Org A
	reqUpload, _ := createMultipartRequest(
		fmt.Sprintf("/api/v1/organizations/%s/documents/", orgA.ID),
		"file",
		"secret-a.pdf",
		[]byte("%PDF-1.4\nConfidential Org A Plan\n%%EOF"),
		map[string]string{"Authorization": "Bearer " + tokenA},
	)
	respUpload, _ := app.Test(reqUpload)
	defer respUpload.Body.Close()

	var uploadData struct {
		Data struct {
			DocumentID string `json:"document_id"`
		} `json:"data"`
	}
	bUpload, _ := io.ReadAll(respUpload.Body)
	_ = json.Unmarshal(bUpload, &uploadData)
	docAID := uploadData.Data.DocumentID

	// 1. User A retrieves Document A -> 200 OK
	reqGetA := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/documents/%s", docAID), nil)
	reqGetA.Header.Set("Authorization", "Bearer "+tokenA)
	respGetA, _ := app.Test(reqGetA)
	defer respGetA.Body.Close()

	if respGetA.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for document owner, got %d", respGetA.StatusCode)
	}

	var statusResp struct {
		Data struct {
			ID         string `json:"id"`
			Filename   string `json:"filename"`
			Status     string `json:"status"`
			StorageKey string `json:"storage_key"` // Should NOT exist in JSON!
		} `json:"data"`
	}
	bGetA, _ := io.ReadAll(respGetA.Body)
	_ = json.Unmarshal(bGetA, &statusResp)

	if statusResp.Data.Filename != "secret-a.pdf" {
		t.Errorf("expected filename 'secret-a.pdf', got %s", statusResp.Data.Filename)
	}
	if statusResp.Data.StorageKey != "" {
		t.Errorf("SECURITY INVARIANT LEAK: storage_key must not be exposed to client!")
	}

	// 2. CROSS-TENANT ISOLATION: User B attempts to view Document A -> 403 Forbidden!
	reqGetB := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/documents/%s", docAID), nil)
	reqGetB.Header.Set("Authorization", "Bearer "+tokenB)
	respGetB, _ := app.Test(reqGetB)
	defer respGetB.Body.Close()

	if respGetB.StatusCode != http.StatusForbidden {
		t.Fatalf("SECURITY VIOLATION: expected 403 Forbidden when User B accesses Document A, got %d", respGetB.StatusCode)
	}

	// 3. User B cannot see Document A in Org B's document list
	reqListB := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/organizations/%s/documents/", orgB.ID), nil)
	reqListB.Header.Set("Authorization", "Bearer "+tokenB)
	respListB, _ := app.Test(reqListB)
	defer respListB.Body.Close()

	var listData struct {
		Data []interface{} `json:"data"`
	}
	bListB, _ := io.ReadAll(respListB.Body)
	_ = json.Unmarshal(bListB, &listData)

	if len(listData.Data) != 0 {
		t.Errorf("expected 0 documents in Org B, got %d", len(listData.Data))
	}
}

func TestDocumentDeleteAndRetryLifecycle(t *testing.T) {
	app, idRepo, docRepo, _, jwtManager := setupDocumentsTestApp()
	ctx := context.Background()

	user, _ := idRepo.CreateUser(ctx, "lifecycle@example.com", "hash", "Lifecycle User")
	token, _, _ := jwtManager.GenerateAccessToken(user.ID, user.Email)
	org, _ := idRepo.CreateOrganization(ctx, "Lifecycle Org", user.ID)

	// Upload document
	reqUpload, _ := createMultipartRequest(
		fmt.Sprintf("/api/v1/organizations/%s/documents/", org.ID),
		"file",
		"lifecycle.pdf",
		[]byte("%PDF-1.4\nLifecycle Document\n%%EOF"),
		map[string]string{"Authorization": "Bearer " + token},
	)
	respUpload, _ := app.Test(reqUpload)
	defer respUpload.Body.Close()

	var uploadData struct {
		Data struct {
			DocumentID string `json:"document_id"`
		} `json:"data"`
	}
	b, _ := io.ReadAll(respUpload.Body)
	_ = json.Unmarshal(b, &uploadData)
	docID, _ := uuid.Parse(uploadData.Data.DocumentID)

	// 1. Retry on a pending document must fail (only failed docs can be retried)
	reqRetryPending := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/documents/%s/retry", docID), nil)
	reqRetryPending.Header.Set("Authorization", "Bearer "+token)
	respRetryPending, _ := app.Test(reqRetryPending)
	defer respRetryPending.Body.Close()

	if respRetryPending.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request when retrying pending document, got %d", respRetryPending.StatusCode)
	}

	// 2. Mark document as failed
	errCode := "CORRUPT_PAGES"
	errMsg := "Document contains corrupted page data"
	_ = docRepo.UpdateDocumentStatus(ctx, docID, documents.StatusFailed, &errCode, &errMsg)

	// 3. Retry on failed document succeeds
	reqRetryFailed := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/documents/%s/retry", docID), nil)
	reqRetryFailed.Header.Set("Authorization", "Bearer "+token)
	respRetryFailed, _ := app.Test(reqRetryFailed)
	defer respRetryFailed.Body.Close()

	if respRetryFailed.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted on retrying failed document, got %d", respRetryFailed.StatusCode)
	}

	// Status should be reset to 'pending'
	docAfterRetry, _ := docRepo.GetDocumentByID(ctx, docID)
	if docAfterRetry.Status != documents.StatusPending {
		t.Errorf("expected status 'pending' after retry, got %s", docAfterRetry.Status)
	}

	// 4. Delete document
	reqDelete := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/api/v1/documents/%s", docID), nil)
	reqDelete.Header.Set("Authorization", "Bearer "+token)
	respDelete, _ := app.Test(reqDelete)
	defer respDelete.Body.Close()

	if respDelete.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202 Accepted on delete, got %d", respDelete.StatusCode)
	}

	// Document status must now be 'deleting'
	docAfterDelete, _ := docRepo.GetDocumentByID(ctx, docID)
	if docAfterDelete.Status != documents.StatusDeleting {
		t.Errorf("expected status 'deleting' after delete call, got %s", docAfterDelete.Status)
	}
}
