.PHONY: dev dev-seed server client build clean install test \
       run-local run-dev run-staging run-testing run-pre-prod run-production

# ── Local development (default) ──────────────────────────────────────────────

dev:
	@echo "Starting local backend (:8080) and frontend (:5173)..."
	@make -j2 server client

dev-seed:
	@echo "Starting local backend (:8080) with seed + frontend (:5173)..."
	@make -j2 server-seed client

server:
	go run ./cmd/server/ --env=local

server-seed:
	go run ./cmd/server/ --env=local --seed

client:
	cd client && npm run dev

# ── Per-environment targets ──────────────────────────────────────────────────

run-local:
	go run ./cmd/server/ --env=local

run-local-seed:
	go run ./cmd/server/ --env=local --seed

run-dev:
	go run ./cmd/server/ --env=dev

run-dev-seed:
	go run ./cmd/server/ --env=dev --seed

run-testing:
	go run ./cmd/server/ --env=testing

run-testing-seed:
	go run ./cmd/server/ --env=testing --seed

# Staging+ require DATABASE_URL and JWT_SECRET env vars
run-staging:
	go run ./cmd/server/ --env=staging

run-pre-prod:
	go run ./cmd/server/ --env=pre-prod

run-production:
	go run ./cmd/server/ --env=production

# ── Local PostgreSQL (via Docker) ────────────────────────────────────────────

pg-up:
	docker compose up -d postgres

pg-down:
	docker compose down

# Run with local PostgreSQL instead of SQLite
run-local-pg:
	DB_DRIVER=postgres DATABASE_URL="postgres://realnumber:realnumber@localhost:5432/realnumber?sslmode=disable" \
		go run ./cmd/server/ --env=local

run-local-pg-seed:
	DB_DRIVER=postgres DATABASE_URL="postgres://realnumber:realnumber@localhost:5432/realnumber?sslmode=disable" \
		go run ./cmd/server/ --env=local --seed

# ── Build & utilities ────────────────────────────────────────────────────────

build:
	go build -o bin/server ./cmd/server/
	cd client && npm run build

clean:
	rm -rf bin/ client/dist/ realnumber_*.db realnumber.db

install:
	cd client && npm install

test:
	go test ./...
