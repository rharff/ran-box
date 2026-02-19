# Naratel Box — Project Initialization Guide

> A self-hosted file storage system with block-level deduplication, streaming upload/download, and JWT-based access control.  
> Stack: **Go (Backend API)** · **PostgreSQL (Metadata)** · **QNAP S3 (Block Storage)** · **React + Vite (Frontend MVP)**

---

## Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Monorepo Structure](#monorepo-structure)
3. [Block Size Decision](#block-size-decision)
4. [Prerequisites](#prerequisites)
5. [Environment Variables](#environment-variables)
6. [Database Schema](#database-schema)
7. [Backend Setup (Go)](#backend-setup-go)
8. [Frontend Setup (React + Vite)](#frontend-setup-react--vite)
9. [API Contract](#api-contract)
10. [Running the Project](#running-the-project)
11. [Docker Compose](#docker-compose)

---

## Architecture Overview

```
┌─────────────┐        ┌──────────────────┐        ┌─────────────────┐        ┌──────────────┐
│   Frontend  │──JWT──▶│  Backend (Go)    │──SQL──▶│  PostgreSQL     │        │  QNAP S3     │
│  React+Vite │◀───────│  Fiber / Chi     │        │  (Metadata +    │        │  (Block      │
│             │        │                  │──S3───▶│   Block Index)  │        │   Objects)   │
└─────────────┘        └──────────────────┘        └─────────────────┘        └──────────────┘
```

### Upload Flow
1. Client streams file → Backend splits into **8MB blocks**
2. Per block: compute **SHA-256** hash
3. Check PostgreSQL: does this hash already exist?
   - **Hash not found** → `PutObject` to S3 (key = hash hex), register block in DB
   - **Hash found** → skip upload (deduplication)
4. Link all block IDs to a new `file_version` record in DB
5. Return `201 Created` with file metadata

### Download Flow
1. Client sends `GET /api/v1/files/:id` with `Authorization: Bearer <JWT>`
2. Backend decodes JWT → extracts `user_id`
3. Query DB: does file `id` belong to `user_id`?
   - **Authorized** → fetch block list, stream blocks from S3 in order → respond with file stream
   - **Unauthorized** → `403 Forbidden`

---

## Monorepo Structure

```
naratel-box/
├── PROJECT_INIT.md          ← this file
├── docker-compose.yml
├── .env.example
├── Makefile
│
├── backend/                 ← Go API
│   ├── cmd/
│   │   └── api/
│   │       └── main.go
│   ├── internal/
│   │   ├── auth/            ← JWT middleware
│   │   │   ├── jwt.go
│   │   │   └── middleware.go
│   │   ├── block/           ← Block split, hash, dedup logic
│   │   │   ├── splitter.go
│   │   │   └── hasher.go
│   │   ├── storage/         ← S3 client wrapper
│   │   │   └── s3.go
│   │   ├── repository/      ← DB queries (sqlc or raw)
│   │   │   ├── files.go
│   │   │   ├── blocks.go
│   │   │   └── users.go
│   │   ├── handler/         ← HTTP handlers
│   │   │   ├── auth.go
│   │   │   ├── file_upload.go
│   │   │   └── file_download.go
│   │   └── model/           ← Domain structs
│   │       ├── file.go
│   │       ├── block.go
│   │       └── user.go
│   ├── migrations/          ← SQL migration files
│   │   ├── 001_create_users.sql
│   │   ├── 002_create_blocks.sql
│   │   └── 003_create_files.sql
│   ├── go.mod
│   └── go.sum
│
└── frontend/                ← React + Vite MVP
    ├── src/
    │   ├── api/             ← Axios client + API calls
    │   │   └── client.ts
    │   ├── components/
    │   │   ├── FileUpload.tsx
    │   │   ├── FileList.tsx
    │   │   └── DownloadButton.tsx
    │   ├── pages/
    │   │   ├── LoginPage.tsx
    │   │   └── DashboardPage.tsx
    │   ├── store/           ← Zustand state
    │   │   └── authStore.ts
    │   ├── App.tsx
    │   └── main.tsx
    ├── index.html
    ├── package.json
    ├── tsconfig.json
    └── vite.config.ts
```

---

## Block Size Decision

| Block Size | Pros | Cons |
|---|---|---|
| 4 MB | Fine dedup granularity | More DB rows, more S3 requests |
| **8 MB** ✅ | Balanced: good dedup + fewer S3 ops | — |
| 16 MB | Fewer S3 requests | Coarse dedup, higher memory per worker |
| 64 MB | Best throughput | Near-zero dedup benefit |

**Decision: `BLOCK_SIZE = 8MB` (8 × 1024 × 1024 bytes)**

Rationale:
- S3 multipart upload minimum part size is **5MB** — 8MB comfortably exceeds it
- SHA-256 deduplication is meaningful at 8MB granularity for typical office/media files
- Fits in RAM even with 10 concurrent upload workers (10 × 8MB = 80MB buffer)
- Configurable via `BLOCK_SIZE_MB` env var

---

## Prerequisites

| Tool | Version | Purpose |
|---|---|---|
| Go | ≥ 1.22 | Backend |
| Node.js | ≥ 20 | Frontend |
| PostgreSQL | ≥ 15 | Metadata DB |
| Docker + Compose | latest | Local dev |
| `migrate` CLI | latest | DB migrations |
| `golang-migrate` | — | Migration runner |

Install `migrate` CLI:
```bash
go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
```

---

## Environment Variables

Copy `.env.example` to `.env` and fill in values:

```bash
cp .env.example .env
```

### `.env.example`

```env
# ── App ───────────────────────────────────────────
APP_PORT=8080
APP_ENV=development

# ── JWT ───────────────────────────────────────────
JWT_SECRET=your-super-secret-key-change-in-production
JWT_EXPIRY_HOURS=24

# ── PostgreSQL ────────────────────────────────────
DB_HOST=localhost
DB_PORT=5432
DB_NAME=naratel_box
DB_USER=postgres
DB_PASSWORD=postgres
DB_SSLMODE=disable

# ── QNAP S3 ───────────────────────────────────────
S3_ENDPOINT=https://your-qnap-host:port
S3_BUCKET=naratel-blocks
S3_ACCESS_KEY=your-access-key
S3_SECRET_KEY=your-secret-key
S3_REGION=us-east-1
S3_FORCE_PATH_STYLE=true        # required for QNAP / MinIO S3

# ── Block ─────────────────────────────────────────
BLOCK_SIZE_MB=8
```

---

## Database Schema

Run migrations in order:

```bash
migrate -path backend/migrations -database "postgres://postgres:postgres@localhost:5432/naratel_box?sslmode=disable" up
```

### Migration 001 — Users

```sql
-- backend/migrations/001_create_users.sql
CREATE TABLE users (
    id         BIGSERIAL PRIMARY KEY,
    email      TEXT        NOT NULL UNIQUE,
    password   TEXT        NOT NULL,       -- bcrypt hash
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

### Migration 002 — Blocks

```sql
-- backend/migrations/002_create_blocks.sql
CREATE TABLE blocks (
    id          BIGSERIAL PRIMARY KEY,
    sha256_hash CHAR(64)   NOT NULL UNIQUE,  -- hex-encoded SHA-256
    s3_key      TEXT       NOT NULL,          -- same as sha256_hash
    size_bytes  BIGINT     NOT NULL,
    ref_count   INT        NOT NULL DEFAULT 0, -- dedup reference count
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_blocks_sha256 ON blocks(sha256_hash);
```

### Migration 003 — Files & Versions

```sql
-- backend/migrations/003_create_files.sql
CREATE TABLE files (
    id           BIGSERIAL PRIMARY KEY,
    user_id      BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT        NOT NULL,
    mime_type    TEXT,
    total_size   BIGINT      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE file_blocks (
    id           BIGSERIAL PRIMARY KEY,
    file_id      BIGINT NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    block_id     BIGINT NOT NULL REFERENCES blocks(id),
    block_index  INT    NOT NULL,   -- order of block in file (0-based)
    UNIQUE (file_id, block_index)
);

CREATE INDEX idx_files_user_id     ON files(user_id);
CREATE INDEX idx_file_blocks_file  ON file_blocks(file_id);
```

---

## Backend Setup (Go)

### 1. Initialize module

```bash
cd backend
go mod init github.com/naratel/naratel-box/backend
```

### 2. Install dependencies

```bash
go get github.com/gofiber/fiber/v2
go get github.com/gofiber/fiber/v2/middleware/logger
go get github.com/gofiber/fiber/v2/middleware/cors
go get github.com/golang-jwt/jwt/v5
go get github.com/aws/aws-sdk-go-v2
go get github.com/aws/aws-sdk-go-v2/config
go get github.com/aws/aws-sdk-go-v2/service/s3
go get github.com/aws/aws-sdk-go-v2/credentials
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/pgxpool
go get github.com/joho/godotenv
go get golang.org/x/crypto
```

### 3. Key implementation notes

#### `internal/block/splitter.go`
- Read incoming `io.Reader` in `BLOCK_SIZE_MB` chunks
- For each chunk: compute SHA-256, check DB, conditionally upload to S3
- Use `io.LimitReader` for memory-safe chunking

#### `internal/auth/jwt.go`
- Sign tokens with `HS256` using `JWT_SECRET`
- Claims: `user_id`, `email`, `exp`
- Middleware extracts Bearer token, validates, injects `user_id` into `fiber.Ctx.Locals`

#### `internal/storage/s3.go`
- Use `aws-sdk-go-v2` with custom `EndpointResolverV2` for QNAP S3
- Set `UsePathStyle: true` (required for QNAP/MinIO)
- `PutObject` for upload, `GetObject` for streaming download

#### `internal/handler/file_download.go`
- Fetch `file_blocks` ordered by `block_index`
- For each block: `GetObject` from S3 → `io.Copy` to response writer
- Set `Content-Disposition: attachment; filename="<name>"`
- Set `Content-Length` from `total_size`

---

## Frontend Setup (React + Vite)

### 1. Initialize project

```bash
cd frontend
npm create vite@latest . -- --template react-ts
npm install
```

### 2. Install dependencies

```bash
npm install axios zustand react-router-dom
npm install -D tailwindcss postcss autoprefixer
npx tailwindcss init -p
```

### 3. Key pages

| Page | Route | Description |
|---|---|---|
| `LoginPage` | `/login` | Email + password form → calls `POST /api/v1/auth/login` → stores JWT in Zustand + localStorage |
| `DashboardPage` | `/` | Lists user's files, upload button, download button |

### 4. Upload strategy (frontend)
- Use `FormData` with `Content-Type: multipart/form-data`
- Show upload progress via `onUploadProgress` in Axios
- The **backend handles all block splitting** — frontend sends the raw file as a single stream

---

## API Contract

### Auth

| Method | Endpoint | Body | Response |
|---|---|---|---|
| `POST` | `/api/v1/auth/register` | `{email, password}` | `{user_id, email}` |
| `POST` | `/api/v1/auth/login` | `{email, password}` | `{token, expires_at}` |

### Files (all require `Authorization: Bearer <token>`)

| Method | Endpoint | Body | Response |
|---|---|---|---|
| `POST` | `/api/v1/files` | `multipart/form-data` (field: `file`) | `{file_id, name, size, blocks_count}` |
| `GET` | `/api/v1/files` | — | `[{file_id, name, size, created_at}]` |
| `GET` | `/api/v1/files/:id` | — | File stream (binary) |
| `DELETE` | `/api/v1/files/:id` | — | `204 No Content` |

### Error responses

```json
{
  "error": "unauthorized",
  "message": "token is expired"
}
```

HTTP status codes: `400` bad request · `401` unauthorized · `403` forbidden · `404` not found · `500` internal error

---

## Running the Project

### Local (without Docker)

```bash
# Terminal 1 — Backend
cd backend
cp ../.env.example ../.env   # fill in values
go run cmd/api/main.go

# Terminal 2 — Frontend
cd frontend
npm run dev
```

### With Make

```bash
make dev-backend    # go run
make dev-frontend   # npm run dev
make migrate-up     # run DB migrations
make migrate-down   # rollback
```

---

## Docker Compose

```yaml
# docker-compose.yml
version: "3.9"

services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_DB: naratel_box
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: postgres
    ports:
      - "5432:5432"
    volumes:
      - pg_data:/var/lib/postgresql/data

  backend:
    build:
      context: ./backend
      dockerfile: Dockerfile
    ports:
      - "8080:8080"
    env_file: .env
    depends_on:
      - postgres

  frontend:
    build:
      context: ./frontend
      dockerfile: Dockerfile
    ports:
      - "5173:80"
    depends_on:
      - backend

volumes:
  pg_data:
```

---

## Makefile

```makefile
.PHONY: dev-backend dev-frontend migrate-up migrate-down

dev-backend:
	cd backend && go run cmd/api/main.go

dev-frontend:
	cd frontend && npm run dev

migrate-up:
	migrate -path backend/migrations \
	  -database "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)" up

migrate-down:
	migrate -path backend/migrations \
	  -database "postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)" down
```

---

## Next Steps (Post-MVP)

- [ ] File versioning (multiple versions per file name)
- [ ] Block garbage collection (decrement `ref_count`, delete orphan blocks from S3)
- [ ] Chunked resumable upload (track which blocks were already uploaded)
- [ ] Block encryption at rest (AES-256 before PutObject)
- [ ] Admin dashboard (storage stats, dedup ratio)
- [ ] Rate limiting per user
