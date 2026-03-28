.PHONY: dev dev-seed seed client build clean install test \
       svc-gateway svc-query svc-portal svc-worker

# ── Development (microservices) ──────────────────────────────────────────────

dev:
	@echo "Starting: gateway(:8080) query(:8081) portal(:8082) worker + frontend(:5173)..."
	@make -j5 svc-gateway svc-query svc-portal svc-worker client

dev-seed:
	@make seed
	@make dev

seed:
	@echo "Seeding database..."
	@go run ./cmd/worker-service/ --env=local --seed &
	@sleep 4
	@pkill -f "worker-service.*--seed" 2>/dev/null || true
	@echo "Seed complete."

client:
	cd client && npm run dev

svc-gateway:
	GATEWAY_PORT=8080 QUERY_SERVICE_URL=http://localhost:8081 PORTAL_SERVICE_URL=http://localhost:8082 \
		go run ./cmd/gateway/

svc-query:
	QUERY_PORT=8081 go run ./cmd/query-service/ --env=local

svc-portal:
	PORTAL_PORT=8082 go run ./cmd/portal-service/ --env=local

svc-worker:
	go run ./cmd/worker-service/ --env=local

# ── Per-environment ──────────────────────────────────────────────────────────

run-staging:
	@echo "Start each service with DATABASE_URL and JWT_SECRET set"
	@echo "  QUERY_PORT=8081 go run ./cmd/query-service/ --env=staging"
	@echo "  PORTAL_PORT=8082 go run ./cmd/portal-service/ --env=staging"
	@echo "  go run ./cmd/worker-service/ --env=staging"
	@echo "  GATEWAY_PORT=8080 go run ./cmd/gateway/"

# ── Local PostgreSQL (via Docker) ────────────────────────────────────────────

pg-up:
	docker compose up -d postgres

pg-down:
	docker compose down

# ── Build & utilities ────────────────────────────────────────────────────────

build:
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
