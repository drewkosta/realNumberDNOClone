.PHONY: dev dev-seed server client build clean install test \
       run-local run-dev run-staging run-testing run-pre-prod run-production

# ── Local development (default) ──────────────────────────────────────────────

# Run backend + frontend, no seed
dev:
	@echo "Starting local backend (:8080) and frontend (:5173)..."
	@make -j2 server client

# Run backend + frontend, seed mock data on first run
dev-seed:
	@echo "Starting local backend (:8080) with seed + frontend (:5173)..."
	@make -j2 server-seed client

server:
	go run ./cmd/server/ --env=local

server-seed:
	go run ./cmd/server/ --env=local --seed

client:
	cd client && npm run dev

# ── Per-environment server targets ───────────────────────────────────────────

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

run-staging:
	go run ./cmd/server/ --env=staging

run-pre-prod:
	go run ./cmd/server/ --env=pre-prod

run-production:
	go run ./cmd/server/ --env=production

# ── Build & utilities ──────���─────────────────────────────────────────────────

build:
	go build -o bin/server ./cmd/server/
	cd client && npm run build

clean:
	rm -rf bin/ client/dist/ realnumber_*.db realnumber.db

install:
	cd client && npm install

test:
	go test ./...
