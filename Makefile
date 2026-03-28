.PHONY: dev dev-seed server client build clean install test \
       run-local run-dev run-staging run-testing run-pre-prod run-production \
       microservices micro-seed

# ── Monolith (default, backward compatible) ──────────────────────────────────

dev:
	@echo "Starting monolith backend (:8080) + frontend (:5173)..."
	@make -j2 server client

dev-seed:
	@echo "Starting monolith backend (:8080) with seed + frontend (:5173)..."
	@make -j2 server-seed client

server:
	go run ./cmd/server/ --env=local

server-seed:
	go run ./cmd/server/ --env=local --seed

client:
	cd client && npm run dev

# ── Microservices ────────────────────────────────────────────────────────────

microservices:
	@echo "Starting microservices: gateway(:8080) query(:8081) portal(:8082) worker + frontend(:5173)..."
	@make -j5 svc-gateway svc-query svc-portal svc-worker client

micro-seed:
	@echo "Seeding database via worker-service..."
	go run ./cmd/worker-service/ --env=local --seed &
	@sleep 3
	@kill %1 2>/dev/null || true
	@echo "Seed complete. Run 'make microservices' to start all services."

svc-gateway:
	GATEWAY_PORT=8080 QUERY_SERVICE_URL=http://localhost:8081 PORTAL_SERVICE_URL=http://localhost:8082 \
		go run ./cmd/gateway/

svc-query:
	QUERY_PORT=8081 go run ./cmd/query-service/ --env=local

svc-portal:
	PORTAL_PORT=8082 go run ./cmd/portal-service/ --env=local

svc-worker:
	go run ./cmd/worker-service/ --env=local

# ── Per-environment (monolith) ───────────────────────────────────────────────

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

# ── Local PostgreSQL (via Docker) ────────────────────────────────────────────

pg-up:
	docker compose up -d postgres

pg-down:
	docker compose down

run-local-pg:
	DB_DRIVER=postgres DATABASE_URL="postgres://realnumber:realnumber@localhost:5432/realnumber?sslmode=disable" \
		go run ./cmd/server/ --env=local

run-local-pg-seed:
	DB_DRIVER=postgres DATABASE_URL="postgres://realnumber:realnumber@localhost:5432/realnumber?sslmode=disable" \
		go run ./cmd/server/ --env=local --seed

# ── Build & utilities ────────────────────────────────────────────────────────

build:
	go build -o bin/server ./cmd/server/
	go build -o bin/query-service ./cmd/query-service/
	go build -o bin/portal-service ./cmd/portal-service/
	go build -o bin/worker-service ./cmd/worker-service/
	go build -o bin/gateway ./cmd/gateway/
	cd client && npm run build

clean:
	rm -rf bin/ client/dist/ realnumber_*.db realnumber.db

install:
	cd client && npm install

test:
	go test ./...
