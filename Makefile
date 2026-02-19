.PHONY: dev-backend tidy migrate-up migrate-down migrate-create build \
        docker-up docker-down docker-logs docker-rebuild swag

include .env
export

# ── Dev ───────────────────────────────────────────
dev-backend:
	cd backend && go run ./cmd/api/main.go

# ── Go ────────────────────────────────────────────
tidy:
	cd backend && go mod tidy

build:
	cd backend && go build -o bin/api ./cmd/api/main.go

# ── Migrations ────────────────────────────────────
DB_URL=postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

migrate-up:
	migrate -path backend/migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path backend/migrations -database "$(DB_URL)" down

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir backend/migrations -seq $$name

# ── Docker ────────────────────────────────────────
docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f backend

docker-rebuild:
	docker compose up -d --build --force-recreate backend

# ── Swagger ───────────────────────────────────────
swag:
	cd backend && ~/go/bin/swag init -g cmd/api/main.go --output docs
