# FakeNumber DNO

A full-stack clone of [Somos RealNumber DNO](https://www.somos.com/realnumber) (Do Not Originate), the telecom industry's authoritative database for preventing illegal robocalls and caller ID spoofing.

This system maintains a database of phone numbers that should never appear as the originating caller ID on outbound calls. Carriers query the DNO database in real-time to block spoofed calls before they reach end users.

## Table of Contents

- [Architecture](#architecture)
- [Quick Start](#quick-start)
- [Roles & Access Model](#roles--access-model)
- [Environment Configuration](#environment-configuration)
- [API Reference](#api-reference)
- [Frontend](#frontend)
- [Database](#database)
- [Scaling & Performance](#scaling--performance)
- [Observability](#observability)
- [Project Structure](#project-structure)
- [Makefile Targets](#makefile-targets)

---

## Architecture

The system is split into four microservices sharing a common database, with an API gateway routing traffic to the appropriate service.

```
┌─────────────────┐
│  React Client   │
│  Vite + TS +    │
│  Tailwind       │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│    Gateway      │ :8080
│  (reverse proxy)│
└───┬─────────┬───┘
    │         │
    │ /api/dno/query    everything else
    │ /api/dno/query/   ──────────┐
    │   bulk                      │
    ▼                             ▼
┌──────────────┐         ┌───────────────┐
│ Query Service│ :8081   │ Portal Service│ :8082
│              │         │               │
│ DNO lookups  │         │ Auth & login  │
│ LRU cache    │         │ Number CRUD   │
│ Async query  │         │ Analytics     │
│   log writer │         │ Compliance    │
│ Rate limiting│         │ Webhooks      │
│              │         │ DNO Analyzer  │
│ API key +    │         │ ROI calculator│
│   JWT auth   │         │ Admin tools   │
└──────┬───────┘         └───────┬───────┘
       │                         │
       └────────────┬────────────┘
                    │
            ┌───────▼───────┐
            │ Worker Service│ (no HTTP)
            │               │
            │ Bulk job      │
            │   processing  │
            │ TSS sync      │
            │ NPAC events   │
            └───────┬───────┘
                    │
            ┌───────▼───────┐
            │  PostgreSQL   │ (default)
            │  SQLite       │ (local dev)
            └───────────────┘
```

**Backend:** Go, chi router, JWT + API key auth, bcrypt, SQLite/PostgreSQL
**Frontend:** React 19, TypeScript, Vite, Tailwind CSS 4, TanStack Query, Recharts
**Observability:** Structured JSON logging (slog), Prometheus metrics, request IDs

### Services

| Service | Port | Responsibility |
|---------|------|----------------|
| **Gateway** | 8080 | Reverse proxy, CORS, routes queries to query-service and everything else to portal-service |
| **Query Service** | 8081 | Hot-path DNO lookups (single + bulk), LRU cache, async query log writer. Independently scalable. |
| **Portal Service** | 8082 | Auth, number management, analytics, compliance, webhooks, DNO analyzer, ROI calculator, admin |
| **Worker Service** | -- | Background job processor for bulk uploads, TSS registry sync, NPAC porting events. No HTTP server. |

---

## Quick Start

### Prerequisites

- Go 1.23+ (CGO enabled for SQLite)
- Node.js 18+
- (Optional) Docker for PostgreSQL and full microservices deployment

### First Run

```bash
# Install frontend dependencies
make install

# Seed database + start all microservices + frontend
make dev-seed
```

This starts:
- **Gateway** on `http://localhost:8080` (routes to query + portal)
- **Query Service** on `http://localhost:8081`
- **Portal Service** on `http://localhost:8082`
- **Worker Service** (background, no port)
- **Frontend** on `http://localhost:5173` (Vite dev server with hot reload)

The frontend proxies API calls through Vite to the gateway on `:8080`.

### Default Login

| Email | Password |
|-------|----------|
| `admin@realnumber.local` | `admin123` |

Seed data also creates 10 additional users with password `password123`:

| Email | Role | Organization |
|-------|------|--------------|
| `jsmith@acmetelecom.com` | org_admin | Acme Telecom |
| `alee@securegate.com` | operator | SecureGate Systems |
| `tgarcia@pacificbell.com` | org_admin | Pacific Bell Services |
| `viewer@realnumber.local` | viewer | System Admin |
| `operator@realnumber.local` | operator | System Admin |

### Test API Keys (seeded)

| Organization | API Key |
|-------------|---------|
| Acme Telecom | `dno_test_acme_carrier_key_12345` |
| SecureGate Systems | `dno_test_securegate_gw_key_67890` |

### Using Docker (full stack with PostgreSQL)

```bash
docker compose up --build
```

This runs all four services + PostgreSQL in containers.

### Using Local PostgreSQL (without Docker services)

```bash
make pg-up          # Start PostgreSQL container
make seed           # Seed the database
make dev            # Start all services + frontend
make pg-down        # Stop PostgreSQL
```

---

## Roles & Access Model

FakeNumber DNO serves two distinct audiences: the **platform operator** (the company running the app) and **customer organizations** (telecom carriers, gateway providers, and responsible organizations that subscribe to the DNO service).

### User Roles

| Role | Who | Portal Access | API Key Access |
|------|-----|---------------|----------------|
| **admin** | FakeNumber platform staff | Full system access -- manage all orgs, users, API keys, view all data across every org | N/A (uses JWT) |
| **org_admin** | Customer org lead (e.g., Acme Telecom's fraud ops manager) | Manage their org's subscriber DNO numbers and view org-scoped analytics | N/A (uses JWT) |
| **operator** | Customer org staff | Add/remove/query DNO numbers for their org | N/A (uses JWT) |
| **viewer** | Customer org read-only user | Query and view data, cannot modify | N/A (uses JWT) |

### API Key Access (Machine-to-Machine)

For automated real-time integrations (e.g., a carrier's call routing infrastructure checking every inbound call against the DNO list), organizations use **API keys** instead of user accounts:

| Auth Method | Access | Use Case |
|-------------|--------|----------|
| `X-API-Key` header | Query endpoints only (`/api/dno/query`, `/api/dno/query/bulk`) | Carrier SBC, gateway provider call routing, automated systems |
| `Bearer` JWT token | Full portal API (management, analytics, audit, admin) | Human users via the web UI |

API keys are:
- Generated by admins via `POST /api/admin/api-keys?orgId=N`
- SHA-256 hashed before storage (raw key shown once on creation)
- Scoped to an organization (rate limited per-org, query logs attributed to org)
- Revocable via `DELETE /api/admin/api-keys?orgId=N`

### Organization Types

| Org Type | Industry Role | Typical Usage |
|----------|---------------|---------------|
| **carrier** | US telecom operators (AT&T, Verizon, T-Mobile, etc.) | Query DNO list in real-time to block spoofed calls on their network |
| **gateway_provider** | International gateway operators | Block foreign-originated illegal calls entering the US network |
| **resp_org** | Responsible Organizations (manage toll-free numbers in TFNRegistry) | Manually flag their toll-free numbers as inbound-only (subscriber set) |
| **admin** | FakeNumber platform operator | System administration |

### Data Scoping

- **Admins** see all data across all organizations
- **Non-admin users** see only their own organization's data (numbers, analytics, audit logs)
- **API key callers** see query results (hit/miss) but cannot access management endpoints
- Query logs are always attributed to the calling organization regardless of auth method

---

## Environment Configuration

Six environments with tailored defaults. Set via `--env` flag on each service:

| Environment | DB Driver | JWT Secret | Rate Limit | Cache TTL | Seed |
|-------------|-----------|------------|------------|-----------|------|
| `local` | SQLite | hardcoded | disabled | 30s | yes |
| `dev` | SQLite | hardcoded | 100 rps | 30s | yes |
| `testing` | SQLite | hardcoded | disabled | disabled | yes |
| `staging` | **PostgreSQL** | required | 500 rps | 60s | no |
| `pre-prod` | **PostgreSQL** | required | 1000 rps | 60s | no |
| `production` | **PostgreSQL** | required | 2000 rps | 45s | no |

### Environment Variables

| Variable | Description | Example |
|----------|-------------|---------|
| `GATEWAY_PORT` | Gateway listen port | `8080` |
| `QUERY_PORT` | Query service listen port | `8081` |
| `PORTAL_PORT` | Portal service listen port | `8082` |
| `QUERY_SERVICE_URL` | Gateway -> query service URL | `http://localhost:8081` |
| `PORTAL_SERVICE_URL` | Gateway -> portal service URL | `http://localhost:8082` |
| `DB_DRIVER` | `sqlite` or `postgres` | `postgres` |
| `DB_PATH` | SQLite file path | `realnumber_local.db` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:pass@host:5432/db?sslmode=disable` |
| `JWT_SECRET` | HMAC signing key (required staging+) | `your-secret-here` |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` |
| `CORS_ORIGIN` | Allowed origin | `https://app.example.com` |
| `ADMIN_PASSWORD` | Override default admin password | `secure-password` |
| `RATE_LIMIT_RPS` | Requests per second per org | `1000` |

---

## API Reference

Two authentication methods are supported:

- **JWT Bearer token** (portal users): `Authorization: Bearer <token>` -- full API access via portal-service
- **API key** (external integrations): `X-API-Key: <key>` -- query endpoints only via query-service

All endpoints are accessible through the gateway on `:8080`.

### Authentication

| Method | Endpoint | Service | Description |
|--------|----------|---------|-------------|
| `POST` | `/api/auth/login` | portal | Login, returns JWT (rate limited: 5/min per IP) |
| `GET` | `/api/auth/me` | portal | Get current authenticated user |

### DNO Queries (query-service)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/dno/query?phoneNumber=5551234567&channel=voice` | Single number DNO lookup |
| `POST` | `/api/dno/query/bulk` | Bulk lookup (up to 1000 numbers) |

### DNO Number Management (portal-service)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/dno/numbers` | Add number to subscriber DNO list |
| `DELETE` | `/api/dno/numbers?phoneNumber=...&channel=voice` | Remove number (subscriber set only, own org) |
| `GET` | `/api/dno/numbers?page=1&pageSize=25&dataset=...&status=...&channel=...&search=...` | List with filtering/pagination |
| `GET` | `/api/dno/validate-ownership?phoneNumber=...` | Check number ownership against mock registry |

### Bulk Operations (portal + worker)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/dno/bulk-upload` | Upload CSV for async background processing (returns 202 + jobId) |
| `GET` | `/api/dno/bulk-job?jobId=1` | Check bulk job status/progress |
| `GET` | `/api/dno/export` | Download full DNO database as CSV flat file |

### Analytics & Compliance

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/analytics` | Dashboard analytics (cached, org-scoped for non-admins) |
| `GET` | `/api/audit-log?page=1&pageSize=25` | Paginated audit trail |
| `GET` | `/api/compliance-report` | FCC compliance assessment for RMD filings |
| `GET` | `/api/roi-calculator?dailyCallVolume=50000` | Estimate blocked calls and annual savings |
| `POST` | `/api/analyzer` | DNO Analyzer: upload CDR data, get fraud exposure report |

### Webhooks

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/webhooks` | Create webhook subscription (HMAC-SHA256 signed) |
| `GET` | `/api/webhooks` | List webhook subscriptions |
| `DELETE` | `/api/webhooks?id=N` | Delete webhook subscription |

### Admin

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/admin/users` | Create user (min 8-char password, validated role) |
| `POST` | `/api/admin/api-keys?orgId=N` | Generate API key for an org |
| `DELETE` | `/api/admin/api-keys?orgId=N` | Revoke API key |
| `POST` | `/api/admin/itg-ingest` | Add number to ITG traceback set with investigation metadata |
| `POST` | `/api/admin/npac-event` | Simulate NPAC porting event (mock) |
| `POST` | `/api/admin/tss-sync` | Sync non-text-enabled TFNs to text DNO (mock) |

### Infrastructure

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check with DB ping (available on each service) |
| `GET` | `/metrics` | Prometheus metrics scrape endpoint (each service) |

---

## Frontend

### Pages

| Page | Path | Description |
|------|------|-------------|
| Login | `/login` | JWT authentication |
| Dashboard | `/` | Analytics with charts (queries/hour, dataset distribution, hit rate) |
| Query Numbers | `/query` | Single and bulk DNO lookups with color-coded hit/miss |
| DNO List | `/numbers` | Full CRUD with filtering, pagination, search, ownership validation on add |
| Bulk Operations | `/bulk` | CSV upload (async) and flat file export |
| DNO Analyzer | `/analyzer` | Upload CDR traffic data, get fraud exposure report with charts |
| Compliance | `/compliance` | FCC compliance assessment, dataset coverage, recommendations, JSON download |
| Webhooks | `/webhooks` | Create/list/delete webhook subscriptions with payload docs |
| ROI Calculator | `/roi` | Volume slider, projected blocked calls and annual savings |
| Audit Log | `/audit` | Paginated activity trail |
| Admin | `/admin` | User creation, API key management, ITG ingest, NPAC/TSS mock integrations |

### Tech Stack

- **Vite** for dev server with HMR and production builds
- **React Router** for client-side routing
- **TanStack Query** for server state management with automatic refetching
- **Tailwind CSS 4** via `@tailwindcss/vite` plugin
- **Recharts** for dashboard bar charts and pie charts
- **Lucide React** for icons
- **Axios** with JWT interceptor and 401 auto-redirect
- **Micro-transitions** throughout: staggered card entrances, button press effects, skeleton loading, cache-animated stat values

---

## Database

### DNO Data Model

| Dataset | Source | Description |
|---------|--------|-------------|
| **Auto Set** | System-generated / NPAC | Unassigned, disconnected, and spare numbers |
| **Subscriber Set** | Manually flagged by org owners | Inbound-only numbers (hotlines, IVRs, conference bridges) |
| **ITG Set** | Industry Traceback Group | Numbers identified via traceback as spoofed (with investigation ID + threat category) |
| **TSS Registry Set** | TSS Registry sync | Non-text-enabled toll-free numbers (text DNO only) |

### Dual-Driver Support

All service SQL is written in **PostgreSQL dialect** (`$1, $2, ...`). For SQLite (local dev), a transparent adapter rewrites:
- `$N` placeholders to `?`
- `date_trunc('hour', col)` to `strftime('%Y-%m-%d %H:00', col)`
- `NOW()` to `CURRENT_TIMESTAMP`

### SQLite Reader/Writer Split

- **Writer** (`MaxOpenConns=1`): Serializes all writes
- **Reader** (`MaxOpenConns=10`): Concurrent reads via WAL mode

In PostgreSQL mode, both point to the same pool (`MaxOpenConns=25`).

---

## Scaling & Performance

| Feature | Description |
|---------|-------------|
| **Microservice architecture** | Query service scales independently from portal and worker |
| **Async query logging** | Buffered batch-flush, no synchronous INSERT on the hot path |
| **In-process LRU cache** | DNO lookups cached with TTL, invalidated on add/remove |
| **Analytics caching** | 30-60s TTL to avoid repeated full-table aggregations |
| **Batch bulk queries** | Single `WHERE IN (...)` instead of N+1 queries |
| **Background job worker** | Async bulk uploads via separate worker process |
| **Per-org rate limiting** | Configurable RPS via `httprate`, login rate limited per IP |
| **HTTP server timeouts** | Read: 10s, Write: 30s, Idle: 120s |
| **Streaming CSV export** | Rows streamed directly from DB, not buffered in memory |
| **Composite indexes** | `(phone_number, channel, status)` covers the hot-path query |
| **Reader/Writer split** | Concurrent reads on SQLite; shared pool on PostgreSQL |

### Seed Data

| Data | Count |
|------|-------|
| Organizations | 8 (carriers, gateway providers, resp orgs) |
| Users | 10 (various roles) |
| DNO Numbers | ~1,330 (all 4 datasets) |
| Query Logs | 2,000 (48h, ~17% hit rate) |
| Audit Logs | 200 (30 days) |
| Number Registry | 550 (mock TFNRegistry/NPAC) |
| API Keys | 2 (test keys for Acme + SecureGate) |

---

## Observability

### Structured Logging

All services emit JSON logs via `slog`:

```json
{
  "time": "2026-03-28T16:42:33.088Z",
  "level": "INFO",
  "msg": "request",
  "method": "GET",
  "path": "/api/analytics",
  "status": 200,
  "bytes": 1298,
  "duration_ms": 6,
  "request_id": "MacBook-Pro-2.local/rxADltAp5B-000002"
}
```

### Prometheus Metrics

Each service exposes `GET /metrics`:

| Metric | Type | Labels |
|--------|------|--------|
| `http_requests_total` | counter | method, path, status |
| `http_request_duration_seconds` | histogram | method, path |
| `dno_query_total` | counter | channel, result |
| `dno_query_duration_seconds` | histogram | |
| `dno_bulk_query_size` | histogram | |
| `cache_hits_total` / `cache_misses_total` | counter | cache |

### Health Check

Each service exposes `GET /health` with DB ping and service name:

```json
{ "status": "ok", "env": "local", "service": "query-service", "db": "ok" }
```

---

## Project Structure

```
.
├── cmd/
│   ├── gateway/main.go             # API gateway (reverse proxy)
│   ├── query-service/main.go       # DNO lookup service (hot path)
│   ├── portal-service/main.go      # Management API service
│   └── worker-service/main.go      # Background job processor + seed tool
├── internal/
│   ├── api/
│   │   ├── handlers_common.go      # Shared Handlers struct, JSON helpers
│   │   ├── handlers_query.go       # QueryNumber, BulkQuery
│   │   ├── handlers_portal.go      # Auth, number CRUD, bulk ops, analytics, audit
│   │   ├── handlers_admin.go       # User mgmt, API keys, ITG ingest, NPAC/TSS
│   │   ├── handlers_integrations.go # Webhooks, ownership, analyzer, compliance, ROI
│   │   ├── middleware.go           # JWT auth, API key auth, admin-only guard
│   │   ├── router_query.go        # Query service routes
│   │   ├── router_portal.go       # Portal service routes
│   │   └── router_common.go       # Shared CORS, health, slog middleware
│   ├── boot/
│   │   ├── boot.go                 # Shared App bootstrap (config + DB + logger)
│   │   └── serve.go               # Graceful HTTP server with signal handling
│   ├── service/
│   │   ├── dno.go                  # DNO business logic (query, add, remove, analytics)
│   │   ├── auth.go                 # Auth (login, JWT, user CRUD)
│   │   ├── apikey.go              # API key generation, hashing, validation
│   │   └── features.go            # ITG ingest, webhooks, analyzer, compliance, ROI, NPAC/TSS
│   ├── db/
│   │   ├── db.go                   # DB init, migrations (SQLite + PostgreSQL), Q() adapter
│   │   └── seed.go                 # Mock data seeder
│   ├── config/config.go            # Environment-based configuration
│   ├── models/models.go            # Domain types, request/response structs, validators
│   ├── cache/cache.go              # Generic TTL cache
│   ├── querylog/writer.go          # Async buffered query log writer
│   ├── jobs/worker.go              # Background job worker
│   └── metrics/metrics.go          # Prometheus metrics + HTTP middleware
├── client/
│   ├── src/
│   │   ├── App.tsx                 # Router and auth provider
│   │   ├── api.ts                  # Axios client (auth, dno, analytics, admin, analyzer, etc.)
│   │   ├── auth.tsx                # Auth context (React 19 use() hook)
│   │   ├── types.ts                # TypeScript types for all API shapes
│   │   ├── components/Layout.tsx   # Sidebar navigation (10 items)
│   │   └── pages/                  # Dashboard, Query, Numbers, Bulk, Analyzer,
│   │                               # Compliance, Webhooks, ROI, Audit, Admin, Login
│   ├── index.html / index.css      # Entry point + Tailwind + micro-transitions
│   ├── vite.config.ts              # Vite + Tailwind plugin + API proxy to gateway
│   └── eslint.config.js            # Type-aware + react-x + react-dom rules
├── Dockerfile                      # Multi-stage build with per-service targets
├── docker-compose.yml              # Full microservices stack with PostgreSQL
├── Makefile
├── go.mod / go.sum
└── .gitignore
```

---

## Makefile Targets

### Development

| Command | Description |
|---------|-------------|
| `make dev` | Start all 4 microservices + frontend |
| `make dev-seed` | Seed database then start everything |
| `make seed` | Seed database only |
| `make client` | Frontend only |
| `make install` | Install frontend npm dependencies |

### Individual Services

| Command | Description |
|---------|-------------|
| `make svc-gateway` | Gateway on :8080 |
| `make svc-query` | Query service on :8081 |
| `make svc-portal` | Portal service on :8082 |
| `make svc-worker` | Worker service (background) |

### Docker

| Command | Description |
|---------|-------------|
| `docker compose up --build` | Full stack (all services + PostgreSQL) |
| `make pg-up` | PostgreSQL container only |
| `make pg-down` | Stop containers |

### Build & Test

| Command | Description |
|---------|-------------|
| `make build` | Build all 4 Go binaries + frontend production bundle |
| `make test` | Run Go tests |
| `make clean` | Remove build artifacts and database files |
