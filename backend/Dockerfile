# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

# Copy dependency files first (better layer caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Tidy modules (resolves new deps like chi, removes old ones like fiber)
RUN go mod tidy

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o bin/api ./cmd/api/main.go

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM alpine:3.19

# Install CA certs and create tmp directory for multipart file uploads
RUN apk add --no-cache ca-certificates tzdata

# Copy the binary
COPY --from=builder /app/bin/api /api

# Ensure /tmp exists and is writable (needed by Go's multipart.ReadForm for large files)
RUN mkdir -p /tmp && chmod 1777 /tmp

EXPOSE 8080

ENTRYPOINT ["/api"]
