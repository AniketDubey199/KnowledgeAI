# Multi-stage Docker build for KnowledgeBase AI
# Stage 1: Build binary
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install build tools and certificates
RUN apk add --no-cache git ca-certificates tzdata

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source tree
COPY . .

# Build statically linked binaries
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/worker ./cmd/worker
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /app/bin/migrate ./cmd/migrate

# Stage 2: Minimal non-root runtime image
FROM alpine:3.20

WORKDIR /app

# Add security updates, certificates and non-root user
RUN apk --no-cache add ca-certificates tzdata curl \
    && addgroup -g 10001 -S appgroup \
    && adduser -u 10001 -S appuser -G appgroup \
    && mkdir -p /var/lib/knowledgebase/uploads \
    && chown -R appuser:appgroup /var/lib/knowledgebase/uploads /app

# Copy binaries and migrations from builder
COPY --from=builder /app/bin/ /app/bin/
COPY --from=builder /app/migrations/ /app/migrations/

USER 10001:10001

EXPOSE 8080

ENTRYPOINT ["/app/bin/api"]
