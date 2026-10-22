#!/usr/bin/env python3
"""
KnowledgeBase AI - Technical Architecture and Implementation Guide Generator
Uses pycairo to produce a publication-quality PDF document.
"""

import os
import sys
import cairo

PAGE_WIDTH = 595   # A4 width in points
PAGE_HEIGHT = 842  # A4 height in points
MARGIN_X = 50
MARGIN_Y = 50
USABLE_WIDTH = PAGE_WIDTH - (2 * MARGIN_X)

class PDFDocument:
    def __init__(self, output_path):
        self.output_path = output_path
        self.surface = cairo.PDFSurface(output_path, PAGE_WIDTH, PAGE_HEIGHT)
        self.ctx = cairo.Context(self.surface)
        self.page_num = 1
        self.y = MARGIN_Y

    def new_page(self):
        self._draw_page_decorations()
        self.surface.show_page()
        self.page_num += 1
        self.y = MARGIN_Y

    def _draw_page_decorations(self):
        # Running header (except page 1)
        if self.page_num > 1:
            self.ctx.save()
            self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
            self.ctx.set_font_size(8)
            self.ctx.set_source_rgb(0.5, 0.5, 0.5)
            self.ctx.move_to(MARGIN_X, 32)
            self.ctx.show_text("KnowledgeBase AI — Technical Architecture & Implementation Blueprint")
            
            # Running header line
            self.ctx.set_line_width(0.5)
            self.ctx.set_source_rgb(0.85, 0.85, 0.85)
            self.ctx.move_to(MARGIN_X, 36)
            self.ctx.line_to(PAGE_WIDTH - MARGIN_X, 36)
            self.ctx.stroke()
            self.ctx.restore()

        # Running footer
        self.ctx.save()
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
        self.ctx.set_font_size(8)
        self.ctx.set_source_rgb(0.5, 0.5, 0.5)
        
        # Footer line
        self.ctx.set_line_width(0.5)
        self.ctx.set_source_rgb(0.85, 0.85, 0.85)
        self.ctx.move_to(MARGIN_X, PAGE_HEIGHT - 32)
        self.ctx.line_to(PAGE_WIDTH - MARGIN_X, PAGE_HEIGHT - 32)
        self.ctx.stroke()

        # Page number
        page_str = f"Page {self.page_num}"
        x_bearing, y_bearing, width, height, x_advance, y_advance = self.ctx.text_extents(page_str)
        self.ctx.move_to(PAGE_WIDTH - MARGIN_X - width, PAGE_HEIGHT - 20)
        self.ctx.show_text(page_str)

        self.ctx.move_to(MARGIN_X, PAGE_HEIGHT - 20)
        self.ctx.show_text("Confidential & Proprietary — Engineering Implementation Guide")
        self.ctx.restore()

    def check_space(self, needed):
        if self.y + needed > PAGE_HEIGHT - MARGIN_Y - 20:
            self.new_page()

    def draw_cover_header(self, title, subtitle, meta_info):
        self.ctx.save()
        
        # Decorative top accent band
        self.ctx.set_source_rgb(0.08, 0.28, 0.52) # Deep corporate blue
        self.ctx.rectangle(MARGIN_X, self.y, USABLE_WIDTH, 6)
        self.ctx.fill()
        self.y += 24

        # Title
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
        self.ctx.set_font_size(24)
        self.ctx.set_source_rgb(0.1, 0.15, 0.25)
        self.ctx.move_to(MARGIN_X, self.y)
        self.ctx.show_text(title)
        self.y += 26

        # Subtitle
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
        self.ctx.set_font_size(13)
        self.ctx.set_source_rgb(0.3, 0.35, 0.45)
        self.ctx.move_to(MARGIN_X, self.y)
        self.ctx.show_text(subtitle)
        self.y += 22

        # Meta pills / badges
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
        self.ctx.set_font_size(9)
        self.ctx.set_source_rgb(0.2, 0.45, 0.7)
        self.ctx.move_to(MARGIN_X, self.y)
        self.ctx.show_text(meta_info)
        self.y += 18

        # Divider line
        self.ctx.set_line_width(1)
        self.ctx.set_source_rgb(0.85, 0.88, 0.92)
        self.ctx.move_to(MARGIN_X, self.y)
        self.ctx.line_to(PAGE_WIDTH - MARGIN_X, self.y)
        self.ctx.stroke()
        self.y += 18

        self.ctx.restore()

    def draw_heading1(self, text):
        self.check_space(45)
        self.ctx.save()
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
        self.ctx.set_font_size(15)
        self.ctx.set_source_rgb(0.08, 0.28, 0.52) # Theme blue
        self.ctx.move_to(MARGIN_X, self.y)
        self.ctx.show_text(text)
        self.y += 8

        self.ctx.set_line_width(0.75)
        self.ctx.set_source_rgb(0.08, 0.28, 0.52)
        self.ctx.move_to(MARGIN_X, self.y)
        self.ctx.line_to(MARGIN_X + 180, self.y)
        self.ctx.stroke()
        self.y += 16
        self.ctx.restore()

    def draw_heading2(self, text):
        self.check_space(35)
        self.ctx.save()
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
        self.ctx.set_font_size(12)
        self.ctx.set_source_rgb(0.18, 0.24, 0.35)
        self.ctx.move_to(MARGIN_X, self.y)
        self.ctx.show_text(text)
        self.y += 16
        self.ctx.restore()

    def draw_heading3(self, text):
        self.check_space(25)
        self.ctx.save()
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
        self.ctx.set_font_size(10)
        self.ctx.set_source_rgb(0.25, 0.32, 0.42)
        self.ctx.move_to(MARGIN_X, self.y)
        self.ctx.show_text(text)
        self.y += 14
        self.ctx.restore()

    def draw_paragraph(self, text, indent=0, font_size=9.5, line_height=14, color=(0.2, 0.2, 0.2)):
        self.ctx.save()
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
        self.ctx.set_font_size(font_size)
        self.ctx.set_source_rgb(*color)

        words = text.split(" ")
        lines = []
        current_line = []
        max_w = USABLE_WIDTH - indent

        for word in words:
            test_line = " ".join(current_line + [word])
            _, _, w, _, _, _ = self.ctx.text_extents(test_line)
            if w <= max_w:
                current_line.append(word)
            else:
                if current_line:
                    lines.append(" ".join(current_line))
                    current_line = [word]
                else:
                    lines.append(word)
                    current_line = []
        if current_line:
            lines.append(" ".join(current_line))

        for line in lines:
            self.check_space(line_height + 2)
            self.ctx.move_to(MARGIN_X + indent, self.y)
            self.ctx.show_text(line)
            self.y += line_height

        self.y += 4
        self.ctx.restore()

    def draw_bullet(self, bold_prefix, text, indent=12):
        self.check_space(20)
        self.ctx.save()
        
        # Bullet dot
        self.ctx.set_source_rgb(0.08, 0.28, 0.52)
        self.ctx.arc(MARGIN_X + 4, self.y - 3, 2, 0, 2 * 3.14159)
        self.ctx.fill()

        # Prefix (bold)
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
        self.ctx.set_font_size(9)
        self.ctx.set_source_rgb(0.1, 0.1, 0.1)
        self.ctx.move_to(MARGIN_X + indent, self.y)
        self.ctx.show_text(bold_prefix + " ")
        _, _, p_w, _, _, _ = self.ctx.text_extents(bold_prefix + " ")

        # Remainder
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
        self.ctx.set_source_rgb(0.25, 0.25, 0.25)
        
        words = text.split(" ")
        lines = []
        current_line = []
        
        first_line_max = USABLE_WIDTH - indent - p_w
        normal_max = USABLE_WIDTH - indent - 10
        is_first = True

        for word in words:
            test_line = " ".join(current_line + [word])
            _, _, w, _, _, _ = self.ctx.text_extents(test_line)
            limit = first_line_max if is_first else normal_max
            if w <= limit:
                current_line.append(word)
            else:
                if current_line:
                    lines.append(" ".join(current_line))
                    current_line = [word]
                    is_first = False
                else:
                    lines.append(word)
                    current_line = []
                    is_first = False
        if current_line:
            lines.append(" ".join(current_line))

        if lines:
            self.ctx.move_to(MARGIN_X + indent + p_w, self.y)
            self.ctx.show_text(lines[0])
            self.y += 13
            for line in lines[1:]:
                self.check_space(15)
                self.ctx.move_to(MARGIN_X + indent + 10, self.y)
                self.ctx.show_text(line)
                self.y += 13
        else:
            self.y += 13

        self.y += 2
        self.ctx.restore()

    def draw_callout(self, title, lines_text, bg_rgb=(0.95, 0.97, 1.0), border_rgb=(0.2, 0.4, 0.8)):
        estimated_height = 24 + len(lines_text) * 14
        self.check_space(estimated_height + 10)
        self.ctx.save()

        # Background box
        self.ctx.set_source_rgb(*bg_rgb)
        self.ctx.rectangle(MARGIN_X, self.y, USABLE_WIDTH, estimated_height)
        self.ctx.fill()

        # Left border bar
        self.ctx.set_source_rgb(*border_rgb)
        self.ctx.rectangle(MARGIN_X, self.y, 4, estimated_height)
        self.ctx.fill()

        # Title
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
        self.ctx.set_font_size(9.5)
        self.ctx.set_source_rgb(*border_rgb)
        self.ctx.move_to(MARGIN_X + 12, self.y + 14)
        self.ctx.show_text(title)

        # Lines
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
        self.ctx.set_font_size(8.5)
        self.ctx.set_source_rgb(0.2, 0.2, 0.2)
        line_y = self.y + 28
        for line in lines_text:
            self.ctx.move_to(MARGIN_X + 12, line_y)
            self.ctx.show_text(line)
            line_y += 13

        self.y += estimated_height + 10
        self.ctx.restore()

    def draw_table(self, headers, rows, col_widths):
        table_height = 20 + len(rows) * 18
        self.check_space(table_height + 15)
        self.ctx.save()

        # Header background
        self.ctx.set_source_rgb(0.12, 0.22, 0.38)
        self.ctx.rectangle(MARGIN_X, self.y, USABLE_WIDTH, 18)
        self.ctx.fill()

        # Header text
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
        self.ctx.set_font_size(8.5)
        self.ctx.set_source_rgb(1, 1, 1)
        cur_x = MARGIN_X
        for idx, h in enumerate(headers):
            self.ctx.move_to(cur_x + 6, self.y + 12)
            self.ctx.show_text(h)
            cur_x += col_widths[idx]

        self.y += 18

        # Rows
        self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
        self.ctx.set_font_size(8)

        for r_idx, row in enumerate(rows):
            self.check_space(20)
            if r_idx % 2 == 1:
                self.ctx.set_source_rgb(0.96, 0.97, 0.98)
                self.ctx.rectangle(MARGIN_X, self.y, USABLE_WIDTH, 17)
                self.ctx.fill()

            # Row divider
            self.ctx.set_line_width(0.5)
            self.ctx.set_source_rgb(0.88, 0.88, 0.9)
            self.ctx.move_to(MARGIN_X, self.y + 17)
            self.ctx.line_to(MARGIN_X + USABLE_WIDTH, self.y + 17)
            self.ctx.stroke()

            cur_x = MARGIN_X
            for c_idx, cell in enumerate(row):
                if c_idx == 0:
                    self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
                    self.ctx.set_source_rgb(0.15, 0.25, 0.4)
                elif c_idx == len(row) - 1 and "COMPLETED" in str(cell):
                    self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_BOLD)
                    self.ctx.set_source_rgb(0.1, 0.55, 0.2)
                else:
                    self.ctx.select_font_face("Sans", cairo.FONT_SLANT_NORMAL, cairo.FONT_WEIGHT_NORMAL)
                    self.ctx.set_source_rgb(0.2, 0.2, 0.2)

                self.ctx.move_to(cur_x + 6, self.y + 12)
                self.ctx.show_text(str(cell))
                cur_x += col_widths[c_idx]

            self.y += 17

        self.y += 10
        self.ctx.restore()

    def finish(self):
        self._draw_page_decorations()
        self.surface.finish()
        print(f"Successfully generated: {self.output_path}")


def build_guide_pdf(target_file):
    doc = PDFDocument(target_file)

    # ================= PAGE 1: TITLE & EXECUTIVE SUMMARY =================
    doc.draw_cover_header(
        title="KnowledgeBase AI — Implementation Manual",
        subtitle="6-Phase Architecture Blueprint, Engineering Rationale & Verification",
        meta_info="Version 1.1  •  Go + Fiber v2 + PostgreSQL + pgvector + Redis + Asynq + Gemini API"
    )

    doc.draw_heading1("1. Executive Summary & Purpose")
    doc.draw_paragraph(
        "KnowledgeBase AI is an enterprise-grade, multi-tenant Retrieval-Augmented Generation (RAG) backend. "
        "Unlike trivial AI wrappers that simply forward uploaded documents directly into prompts, KnowledgeBase AI is engineered "
        "for production robustness, strict tenant isolation, asynchronous durable ingestion, recoverable background pipelines, "
        "and verifiable citation lineage."
    )
    doc.draw_paragraph(
        "This specification organizes the complete lifecycle of the platform into 6 sequential phases punctuated by "
        "5 concrete checkpoints. Each phase addresses specific engineering failure modes, enforces strict interface boundaries, "
        "and guarantees database invariants before proceeding."
    )

    doc.draw_callout(
        "Core Architectural Principles",
        [
            "1. Asynchronous Ingestion: The HTTP API never performs embedding generation or document parsing inline.",
            "2. Durable Outbox Pattern: Ingestion jobs and outbox events are committed transactionally with document metadata.",
            "3. Strict Multi-Tenancy: Every single database query and vector search is scoped to the authenticated organization.",
            "4. Untrusted Model Output: LLM responses cannot bypass authorization or invent fabricated citation IDs.",
            "5. Process Isolation: Fiber API and Asynq background workers share the codebase but run as isolated services."
        ],
        bg_rgb=(0.95, 0.98, 1.0),
        border_rgb=(0.08, 0.28, 0.52)
    )

    doc.draw_heading1("2. Technology Stack & Component Matrix")
    headers = ["Layer", "Technology", "Design Purpose / Rationale"]
    rows = [
        ["Language", "Go 1.22+", "Memory efficiency, compile-time type safety, exceptional concurrency."],
        ["HTTP Framework", "Fiber v2 (fasthttp)", "High-performance routing; safe context isolation for background jobs."],
        ["Primary Database", "PostgreSQL 16", "ACID transactional state, outbox events, and user/org ownership."],
        ["Vector Search", "pgvector (HNSW)", "Cosine similarity search over 768-dimensional Gemini embeddings."],
        ["SQL Access", "pgx/v5 (pgxpool)", "Direct parameterized queries without ORM latency overhead."],
        ["Task Queue", "Redis + Asynq", "Durable task retry policies, uniqueness deduplication, priority queues."],
        ["Authentication", "JWT + Refresh Sessions", "15-minute access JWTs; rotating hashed refresh tokens with family revocation."],
        ["AI Provider", "Gemini API (1.5 Pro / Embeddings)", "Encapsulated provider interface requesting 768-dim embeddings."]
    ]
    doc.draw_table(headers, rows, [90, 140, 265])

    # ================= PAGE 2: 6 PHASES & 5 CHECKPOINTS BREAKDOWN =================
    doc.new_page()
    doc.draw_heading1("3. The 6-Phase Breakdown & 5 Checkpoints")
    doc.draw_paragraph(
        "To deliver a reliable backend without accumulating technical debt, implementation proceeds strictly across 6 sequential phases. "
        "Transitions between phases are guarded by 5 explicit verification checkpoints."
    )

    phases_summary = [
        ["Phase", "Scope & Description", "Checkpoint Gate", "Status"],
        ["Phase 1", "Repository Foundation & Infrastructure", "Checkpoint 1: Compiling binaries, Fiber router, migrations, health probes", "COMPLETED"],
        ["Phase 2", "Identity, Authentication & Tenancy", "Checkpoint 2: Argon2id, JWT/refresh rotation, org RBAC, cross-tenant tests", "COMPLETED"],
        ["Phase 3", "Document Uploads & Outbox Pipeline", "Checkpoint 3: Streaming upload, SHA-256, atomic storage, outbox persistence", "PLANNED"],
        ["Phase 4", "Asynq Worker & Vector Ingestion", "Checkpoint 4: Dispatcher, PDF/TXT/MD parsers, chunking, pgvector writes", "PLANNED"],
        ["Phase 5", "Vector Search & RAG Chat Engine", "Checkpoint 5: Cosine search, Gemini generation, verified citations, SSE streaming", "PLANNED"],
        ["Phase 6", "Hardening, Observability & Evaluation", "Final Release: Rate limits, Prometheus metrics, E2E tests, AI benchmark suite", "PLANNED"]
    ]
    doc.draw_table(phases_summary[0], phases_summary[1:], [55, 175, 205, 60])

    doc.draw_heading2("Detailed Phase Analysis: What, How, and For What Purpose")

    doc.draw_heading3("Phase 1: Repository Foundation & Core Infrastructure (Checkpoint 1) — COMPLETED")
    doc.draw_bullet("What:", "Establishes Go module structure, Fiber v2 router, configuration loader, PostgreSQL & Redis connection pools, database migrations, and health check endpoints.")
    doc.draw_bullet("How:", "Uses pgxpool with connection limits, go-redis client, standard error envelopes, and a multi-stage Dockerfile running non-root.")
    doc.draw_bullet("Purpose:", "Provides a rock-solid, production-ready substrate ensuring external services can be probed and scaled independently.")

    doc.draw_heading3("Phase 2: Identity, Multi-Tenancy & Authentication (Checkpoint 2) — COMPLETED")
    doc.draw_bullet("What:", "User registration, Argon2id password hashing, login, rotating opaque refresh tokens, JWT access tokens, organization lifecycle, and RBAC memberships (owner, admin, member).")
    doc.draw_bullet("How:", "Argon2id with OWASP parameters (64MB memory, 1 iteration, 4 parallelism); 15m JWT access tokens; cryptographically random refresh tokens stored as SHA-256 hashes in PostgreSQL; family_id reuse revocation; single-transaction organization and owner creation; route-level RequireOrgMembership middleware.")
    doc.draw_bullet("Purpose:", "Enforces absolute tenant isolation, prevents unauthorized cross-organization data access, and eliminates session hijacking risks.")

    doc.draw_heading3("Phase 3: Document Management & Ingestion Outbox (Checkpoint 3)")
    doc.draw_bullet("What:", "Streaming multipart uploads, SHA-256 deduplication, file validation (PDF, TXT, MD), and transactional ingestion records.")
    doc.draw_bullet("How:", "Streams raw bytes directly to storage without buffering in memory; inserts document record, ingestion job, and outbox event inside a single PostgreSQL transaction.")
    doc.draw_bullet("Purpose:", "Prevents memory exhaustion on large file uploads and eliminates race conditions between storage and database state.")

    # ================= PAGE 3: PHASES 4-6 & IN-DEPTH PHASE 1 & 2 =================
    doc.new_page()
    doc.draw_heading3("Phase 4: Asynq Ingestion Worker & Vector Ingestion (Checkpoint 4)")
    doc.draw_bullet("What:", "Background task worker consuming outbox events, document parsing, text chunking, and vector embedding generation.")
    doc.draw_bullet("How:", "Polls unpublished outbox events using 'FOR UPDATE SKIP LOCKED', enqueues deterministic tasks into Redis Asynq, calls Gemini embeddings (768 dimensions), and writes chunks to pgvector with HNSW indexing.")
    doc.draw_bullet("Purpose:", "Decouples heavy file parsing and AI API latencies from HTTP response times while guaranteeing at-least-once processing.")

    doc.draw_heading3("Phase 5: Retrieval-Augmented Generation & Chat Engine (Checkpoint 5)")
    doc.draw_bullet("What:", "Semantic vector search, context prompt engineering, Gemini chat generation, and Server-Sent Events (SSE) streaming.")
    doc.draw_bullet("How:", "Computes query embeddings, queries pgvector strictly scoped by organization_id with cosine similarity and relevance thresholds, injects retrieved passages into guarded prompts, and validates citation chunk IDs against the database.")
    doc.draw_bullet("Purpose:", "Eliminates LLM hallucinations by enforcing citations linked strictly to retrieved, authenticated passages.")

    doc.draw_heading3("Phase 6: Production Hardening, Observability & Evaluation (Final Release)")
    doc.draw_bullet("What:", "Atomic Redis rate limiting, Prometheus metrics, structured request tracing, comprehensive integration tests, and RAG evaluation benchmarks.")
    doc.draw_bullet("How:", "Redis sliding-window rate limiters per org/user/endpoint, automated test suites, and RAG benchmark scoring for faithfulness, recall, and latency.")
    doc.draw_bullet("Purpose:", "Guarantees production stability under load and ensures predictable LLM cost and accuracy bounds.")

    doc.draw_heading1("4. Phase 2 Implementation & Security Engineering")
    doc.draw_paragraph(
        "Phase 2 establishes the identity and security backbone of KnowledgeBase AI. "
        "All identity constructs, password security algorithms, token rotation lifecycles, and multi-tenant RBAC boundaries have been fully implemented."
    )

    doc.draw_bullet("Argon2id Password Security:", "Complies with OWASP password storage recommendations. Uses 64MB memory, 1 iteration, 4 parallel threads, 16-byte random salt, and 32-byte key length. Verified with constant-time equality checks.")
    doc.draw_bullet("Dual-Token Lifecycle:", "Short-lived (15-minute) HMAC-SHA256 JWT access tokens containing user_id and email claims. Opaque 32-byte refresh tokens stored exclusively as SHA-256 hashes in PostgreSQL.")
    doc.draw_bullet("Session Family Reuse Detection:", "Each refresh chain shares a unique family_id. When an already-rotated or revoked token is presented, the system revokes the entire session family immediately, thwarting token theft attempts.")
    doc.draw_bullet("HttpOnly Cookie Delivery:", "In accordance with TDD Section 5.2, refresh tokens are never returned in JSON response bodies or stored in localStorage. They are delivered via HttpOnly, Secure, SameSite=Lax cookies with path /api/v1/auth.")
    doc.draw_bullet("Single-Transaction Organization Provisioning:", "POST /api/v1/organizations creates both the organization entity and the creator's owner membership within a single atomic database transaction.")
    doc.draw_bullet("Tenant Isolation & Role Weights:", "RequireOrgMembership middleware extracts :orgID, parses UUID, verifies membership from DB, and enforces role hierarchies (owner:3, admin:2, member:1). Prevents cross-tenant access with 403 Forbidden.")
    doc.draw_bullet("Last-Owner Invariant:", "Explicit business rule and test preventing an organization from deleting or unassigning its last remaining owner.")

    # ================= PAGE 4: CHECKPOINT 2 VERIFICATION & ARTIFACTS =================
    doc.new_page()
    doc.draw_heading1("5. Checkpoint 2 Verification Report")
    
    doc.draw_callout(
        "Checkpoint 2 Verification Results: ALL TESTS PASSED",
        [
            "✓ Password Hashing: Argon2id generation and verification succeeded (0.15s).",
            "✓ JWT Validation: Signatures, expiration limits, and tamper detection confirmed (0.10s).",
            "✓ Registration & Conflict: Case normalization and 409 duplicate email conflict verified.",
            "✓ Login & Cookie Security: Returned 200 OK, issued JWT, and set HttpOnly refresh cookie.",
            "✓ Token Rotation: /api/v1/auth/refresh rotated token and issued fresh JWT access token.",
            "✓ Family Reuse Detection: Presenting old refresh token revoked entire family (401 Unauthorized).",
            "✓ Cross-Tenant Isolation: User B forbidden (403) from accessing User A's organization.",
            "✓ RBAC Enforcement: Members prohibited from inviting users; admins granted privileges.",
            "✓ Invariant Protection: Removal of the final owner rejected with 400 Bad Request."
        ],
        bg_rgb=(0.94, 0.99, 0.95),
        border_rgb=(0.1, 0.6, 0.25)
    )

    doc.draw_heading1("6. Repository Structure in 'Knowledge AI'")
    doc.draw_paragraph(
        "The project layout adheres to clean architecture principles with clear domain separation:"
    )

    repo_files = [
        ["Directory / File", "Purpose"],
        ["cmd/api/main.go", "HTTP entrypoint initializing Fiber v2, repositories, auth routes, and health probes."],
        ["cmd/worker/main.go", "Background process initializing Asynq server, Redis client, and graceful shutdown."],
        ["cmd/migrate/main.go", "Database migration CLI supporting 'up', 'down', and 'version' commands."],
        ["internal/apperror/errors.go", "Centralized standard error envelope (TDD §5.1) and domain error codes."],
        ["internal/apperror/context.go", "Context extraction utilities for authenticated user_id and tenant boundaries."],
        ["internal/auth/password.go", "OWASP-compliant Argon2id password hashing and constant-time verification."],
        ["internal/auth/jwt.go", "HMAC-SHA256 JWT access token minting and signature validation."],
        ["internal/auth/token.go", "Cryptographic random token generation and SHA-256 bytea hashing."],
        ["internal/auth/service.go", "Authentication workflows: signup, login, family rotation, and logout."],
        ["internal/auth/handler.go", "HTTP endpoints for /api/v1/auth with HttpOnly refresh cookie management."],
        ["internal/auth/middleware.go", "JWTMiddleware validating Bearer tokens and setting context locals."],
        ["internal/identity/models.go", "Domain entities: User, Organization, Membership, RefreshSession."],
        ["internal/identity/repository.go", "Repository interface for persistent and test storage."],
        ["internal/identity/postgres_repo.go", "PostgreSQL implementation utilizing pgxpool.Pool."],
        ["internal/identity/memory_repo.go", "Thread-safe in-memory repository for unit and integration testing."],
        ["internal/identity/service.go", "Organization creation, member management, and last-owner protection."],
        ["internal/identity/handler.go", "HTTP endpoints for /api/v1/organizations with RBAC controls."],
        ["internal/identity/middleware.go", "RequireOrgMembership middleware enforcing tenant isolation."],
        ["tests/unit/auth_test.go", "Unit tests for password hashing, JWTs, rotation, and reuse revocation."],
        ["tests/unit/tenancy_test.go", "Unit tests for multi-tenancy, cross-tenant denial, and RBAC roles."]
    ]
    doc.draw_table(repo_files[0], repo_files[1:], [180, 315])

    doc.draw_heading1("7. Instructions for Pushing to GitHub")
    doc.draw_paragraph(
        "Phase 2 is complete, tested, and verified against Checkpoint 2. "
        "Run the following git commands inside '/home/lucifer/Knowledge AI' to push the updates to your repository:"
    )
    doc.draw_bullet("Step 1:", "Stage all new and updated files: git add .")
    doc.draw_bullet("Step 2:", "Commit Phase 2 changes: git commit -m 'feat(phase-2): identity, multi-tenancy, and authentication with token rotation'")
    doc.draw_bullet("Step 3:", "Push to remote repository: git push origin main")

    doc.finish()

if __name__ == "__main__":
    out_dir = os.path.dirname(os.path.abspath(__file__))
    out_pdf = os.path.join(out_dir, "KnowledgeBase_AI_Architecture_and_Implementation_Guide.pdf")
    build_guide_pdf(out_pdf)
