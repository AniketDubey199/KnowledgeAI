# KnowledgeBase AI

A production-grade, multi-tenant KnowledgeBase AI platform built in **Go (Fiber v2)**, **PostgreSQL with pgvector**, **Redis**, and **Google Gemini AI**.

The system prioritizes backend correctness: strict tenant isolation, asynchronous durable ingestion, recoverable jobs, verifiable citations, and secure token rotation.

---

## 🏛 System Architecture Overview

```
                        +----------------------------+
                        |         API Client         |
                        +--------------+-------------+
                                       |
                   REST / JSON / Multipart Upload / SSE
                                       |
                                       v
                        +----------------------------+
                        |      Fiber API Process     |
                        | (Auth, Tenancy, Ingestion, |
                        |  Vector Search, RAG Chat)  |
                        +--------------+-------------+
                                       |
                   +-------------------+-------------------+
                   |                                       |
                   v                                       v
         +-------------------+                   +-------------------+
         |    PostgreSQL     |                   |       Redis       |
         | (pgvector, state, |                   |  (Asynq queues,   |
         |  outbox, tokens)  |                   |   rates, cache)   |
         +---------+---------+                   +---------+---------+
                   ^                                       ^
                   |                                       |
                   +-------------------+-------------------+
                                       |
                                       v
                        +----------------------------+
                        |    Asynq Worker Process    |
                        | (Outbox polling, parsers,  |
                        |  chunking, vector writes)  |
                        +--------------+-------------+
                                       |
                                       v
                        +----------------------------+
                        |     Google Gemini API      |
                        | (Embeddings & Generation)  |
                        +----------------------------+
```

---

## 📋 6-Phase Implementation Roadmap & 5 Checkpoints

| Phase | Description | Key Checkpoint | Status |
|---|---|---|:---:|
| **Phase 1** | **Repository Foundation & Core Infrastructure** | **Checkpoint 1**: Compiling Go binaries (`api`, `worker`, `migrate`), Fiber router with standard error envelope, health endpoints (`/api/v1/health/live`, `/api/v1/health/ready`), DB migrations, Docker Compose configuration. | ✅ **COMPLETED** |
| **Phase 2** | **Identity, Authentication & Multi-Tenancy** | **Checkpoint 2**: Argon2id hashing, short-lived JWT + rotating opaque refresh tokens with family revocation, org creation, RBAC membership (`owner`, `admin`, `member`), cross-tenant denial tests. | ⏳ *Next Phase* |
| **Phase 3** | **Document Management & Ingestion Outbox** | **Checkpoint 3**: Multipart streaming upload, SHA-256 calculation, local storage volume driver, transactional outbox record creation, 202 Accepted response, idempotent uploads. | ⏳ *Upcoming* |
| **Phase 4** | **Asynq Ingestion Pipeline & Embeddings** | **Checkpoint 4**: Redis Asynq task dispatcher (`FOR UPDATE SKIP LOCKED`), PDF/TXT/MD parsers, chunking with token boundaries, Gemini embedding adapter (768d), pgvector HNSW indexing, stale-job recovery. | ⏳ *Upcoming* |
| **Phase 5** | **Retrieval-Augmented Generation (RAG) & Chat Engine** | **Checkpoint 5**: Multi-tenant vector similarity search, query embedding cache, prompt engineering with untrusted context isolation, Gemini generation, citation verification, SSE streaming (`delta`, `citations`, `done`). | ⏳ *Upcoming* |
| **Phase 6** | **Production Hardening, Observability & Evaluation** | **Final Release**: Atomic Redis rate limiting, Prometheus metrics, structured request tracing, comprehensive integration and E2E test suites, AI evaluation dataset (faithfulness, recall). | ⏳ *Upcoming* |

---

## 📂 Repository Directory Layout

```
Knowledge AI/
├── cmd/
│   ├── api/main.go            # HTTP API server entrypoint
│   ├── worker/main.go         # Asynq background worker entrypoint
│   └── migrate/main.go        # Database migration CLI tool
├── internal/
│   ├── config/                # Strongly-typed environment configuration
│   ├── platform/
│   │   ├── postgres/          # pgxpool PostgreSQL client
│   │   ├── redis/             # go-redis client
│   │   └── storage/           # Local file storage driver
│   ├── middleware/            # Standard error envelope, request ID, logger
│   └── observability/         # Structured slog logger
├── migrations/                # Versioned SQL migrations (up/down)
│   ├── 000001_extensions.up.sql
│   ├── 000002_identity.up.sql
│   ├── 000003_documents.up.sql
│   └── 000004_conversations.up.sql
├── tests/
│   └── unit/                  # Fast unit tests
├── api/
│   └── openapi.yaml           # OpenAPI 3.1 specification
├── docs/
│   ├── KnowledgeBase_AI_Architecture_and_Implementation_Guide.pdf
│   └── generate_guide_pdf.py  # Script generating the documentation PDF
├── Dockerfile                 # Multi-stage non-root container definition
├── compose.yaml               # Docker Compose stack
├── Makefile                   # Developer CLI targets
├── .env.example               # Reference configuration
└── README.md
```

---

## 🚀 Quickstart & Local Development

### Prerequisites
- [Go 1.22+](https://go.dev/)
- [Docker & Docker Compose](https://www.docker.com/) (optional for containerized run)
- Python 3 with `pycairo` (for generating documentation PDF)

### 1. Build Binaries
```bash
make build
```
This builds `./bin/api`, `./bin/worker`, and `./bin/migrate`.

### 2. Run Tests
```bash
make test
```

### 3. Run with Docker Compose
```bash
docker compose up --build
```
This boots up:
- PostgreSQL 16 with pgvector on port `5432`
- Redis 7 on port `6379`
- Automated database migrations via `migrate`
- API server on port `8080`
- Asynq background worker

### 4. Health Checks
- **Liveness**:
  ```bash
  curl http://localhost:8080/api/v1/health/live
  ```
- **Readiness** (verifies PostgreSQL and Redis connectivity):
  ```bash
  curl http://localhost:8080/api/v1/health/ready
  ```

---

## 📄 Documentation PDF
A comprehensive technical manual is maintained at:
[`docs/KnowledgeBase_AI_Architecture_and_Implementation_Guide.pdf`](docs/KnowledgeBase_AI_Architecture_and_Implementation_Guide.pdf).

To regenerate the PDF at any time:
```bash
make pdf
```
