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
    │ /api/v1/dno/query  everything else
    │ /api/v1/dno/query/ ─────────┐
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

### Demo Accounts

The seed creates accounts for every role so you can test each access level:

| Role | Email | Password | Organization |
|------|-------|----------|--------------|
| **admin** | `admin@realnumber.local` | `admin123` | System Admin (platform operator) |
| **org_admin** | `jsmith@acmetelecom.com` | `password123` | Acme Telecom (carrier) |
| **org_admin** | `bwilson@nationalvoice.com` | `password123` | National Voice Corp (carrier) |
| **org_admin** | `tgarcia@pacificbell.com` | `password123` | Pacific Bell Services (resp org) |
| **org_admin** | `kpatel@atlantictf.com` | `password123` | Atlantic TF Management (resp org) |
| **operator** | `mjones@acmetelecom.com` | `password123` | Acme Telecom (carrier) |
| **operator** | `alee@securegate.com` | `password123` | SecureGate Systems (gateway provider) |
| **operator** | `dkim@midwestcarrier.com` | `password123` | Midwest Carrier Group (carrier) |
| **operator** | `lchen@coastalgateway.com` | `password123` | Coastal Gateway Inc (gateway provider) |
| **operator** | `operator@realnumber.local` | `password123` | System Admin |
| **viewer** | `viewer@realnumber.local` | `password123` | System Admin (read-only) |

These are also displayed on the login page in local dev.

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
| `X-API-Key` header | Query endpoints only (`/api/v1/dno/query`, `/api/v1/dno/query/bulk`) | Carrier SBC, gateway provider call routing, automated systems |
| `Bearer` JWT token | Full portal API (management, analytics, audit, admin) | Human users via the web UI |

API keys are:
- Generated by admins via `POST /api/v1/admin/api-keys?orgId=N`
- SHA-256 hashed before storage (raw key shown once on creation)
- Scoped to an organization (rate limited per-org, query logs attributed to org)
- Revocable via `DELETE /api/v1/admin/api-keys?orgId=N`

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
| `POST` | `/api/v1/auth/login` | portal | Login, returns JWT (rate limited: 5/min per IP) |
| `GET` | `/api/v1/auth/me` | portal | Get current authenticated user |

### DNO Queries (query-service)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/dno/query?phoneNumber=5551234567&channel=voice` | Single number DNO lookup |
| `POST` | `/api/v1/dno/query/bulk` | Bulk lookup (up to 1000 numbers) |

### DNO Number Management (portal-service)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/dno/numbers` | Add number to subscriber DNO list |
| `DELETE` | `/api/v1/dno/numbers?phoneNumber=...&channel=voice` | Remove number (subscriber set only, own org) |
| `GET` | `/api/v1/dno/numbers?page=1&pageSize=25&dataset=...&status=...&channel=...&search=...` | List with filtering/pagination |
| `GET` | `/api/v1/dno/validate-ownership?phoneNumber=...` | Check number ownership against mock registry |

### Bulk Operations (portal + worker)

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/dno/bulk-upload` | Upload CSV for async background processing (returns 202 + jobId) |
| `GET` | `/api/v1/dno/bulk-job?jobId=1` | Check bulk job status/progress |
| `GET` | `/api/v1/dno/export` | Download full DNO database as CSV flat file |

### Analytics & Compliance

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/analytics` | Dashboard analytics (cached, org-scoped for non-admins) |
| `GET` | `/api/v1/audit-log?page=1&pageSize=25` | Paginated audit trail |
| `GET` | `/api/v1/compliance-report` | FCC compliance assessment for RMD filings |
| `GET` | `/api/v1/roi-calculator?dailyCallVolume=50000` | Estimate blocked calls and annual savings |
| `POST` | `/api/v1/analyzer` | DNO Analyzer: upload CDR data, get fraud exposure report |

### Webhooks

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/webhooks` | Create webhook subscription (HMAC-SHA256 signed) |
| `GET` | `/api/v1/webhooks` | List webhook subscriptions |
| `DELETE` | `/api/v1/webhooks?id=N` | Delete webhook subscription |

### Admin

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/v1/admin/users` | Create user (min 8-char password, validated role) |
| `POST` | `/api/v1/admin/api-keys?orgId=N` | Generate API key for an org |
| `DELETE` | `/api/v1/admin/api-keys?orgId=N` | Revoke API key |
| `POST` | `/api/v1/admin/itg-ingest` | Add number to ITG traceback set with investigation metadata |
| `POST` | `/api/v1/admin/npac-event` | Simulate NPAC porting event (mock) |
| `POST` | `/api/v1/admin/tss-sync` | Sync non-text-enabled TFNs to text DNO (mock) |

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
| `make swagger` | Regenerate Swagger docs from annotations |
| `make clean` | Remove build artifacts and database files |

### Swagger UI

Swagger UI is available at `http://localhost:8080/swagger/index.html` when services are running. All 27 API endpoints are documented with request/response schemas.

---

## Security

### Authentication

- **JWT access tokens** expire in 15 minutes. Refresh tokens expire in 7 days.
- **Refresh flow**: `POST /api/v1/auth/refresh` exchanges a refresh token for a new access + refresh pair. The frontend auto-refreshes on 401 before redirecting to login.
- **bcrypt** with default cost (10) for password hashing. Minimum 8-character passwords enforced.
- **API keys** are SHA-256 hashed before storage. Raw key shown once on generation.

### CSRF

CSRF is **not a concern** for this application because:
- All state-changing requests require a `Bearer` token in the `Authorization` header or an `X-API-Key` header
- Tokens are stored in `localStorage`, not cookies
- Browsers do not automatically attach `Authorization` or `X-API-Key` headers on cross-origin requests
- CORS is configured to only allow specific origins

If cookies are ever introduced for auth, `SameSite=Strict` and a CSRF token would be required.

### Rate Limiting

- Login: 5 requests/minute per IP
- API endpoints: configurable per-org RPS (0 = disabled for local, 2000 for production)
- Rate limit headers exposed: `X-RateLimit-Limit`, `X-RateLimit-Remaining`, `X-RateLimit-Reset`, `Retry-After`

### Request Limits

- Query service: 1MB body limit
- Portal service: 10MB body limit (bulk uploads)
- DNO Analyzer: max 100,000 records per request with 30s timeout
- CSV export: 60s timeout

### Role-Based Access

Backend enforces role checks on all endpoints. See [Roles & Access Model](#roles--access-model).

---

## Operations

### Deployment

**Recommended production topology:**

```
Internet
  │
  ▼
[TLS Termination / Load Balancer]  (e.g., AWS ALB, nginx, Cloudflare)
  │
  ├── Gateway (N instances, stateless)
  │     ├── Query Service (N instances, stateless, scale independently)
  │     └── Portal Service (N instances, stateless)
  │
  ├── Worker Service (1-2 instances, polls for jobs)
  │
  └── PostgreSQL (primary + read replica for analytics)
```

All services are stateless (JWT auth, no sessions) and can be horizontally scaled behind a load balancer. Use the `/ready` endpoint for load balancer health checks (returns 503 if DB unreachable).

**Build and deploy:**

```bash
make build                    # Builds all 4 binaries + frontend
STATIC_DIR=./client/dist \    # Gateway serves frontend in production
  ./bin/gateway
```

**Required environment variables for production:**

```bash
DATABASE_URL=postgres://user:pass@host:5432/dbname?sslmode=require
JWT_SECRET=<random-64-char-string>
GATEWAY_PORT=8080
QUERY_PORT=8081
PORTAL_PORT=8082
QUERY_SERVICE_URL=http://query-service:8081
PORTAL_SERVICE_URL=http://portal-service:8082
```

### Database Migrations

Migrations run automatically on service startup. PostgreSQL uses `pg_advisory_lock` to prevent race conditions when multiple instances start simultaneously. For manual control, start a single instance first, then scale up.

### Backup & Restore

**PostgreSQL:**

```bash
# Backup
pg_dump -Fc $DATABASE_URL > backup_$(date +%Y%m%d).dump

# Restore
pg_restore -d $DATABASE_URL backup_20260328.dump

# Automated daily backup (cron)
0 2 * * * pg_dump -Fc $DATABASE_URL > /backups/daily_$(date +\%Y\%m\%d).dump
```

**SQLite (local dev):**

```bash
cp realnumber_local.db realnumber_local.db.backup
```

### Secrets Management

Environment variables are fine for development. In production, use a secrets manager:

- **AWS**: Secrets Manager or SSM Parameter Store
- **GCP**: Secret Manager
- **Kubernetes**: External Secrets Operator syncing from Vault/AWS/GCP
- **HashiCorp Vault**: Dynamic database credentials, JWT signing keys

At minimum, these must not be in source control or container images:
- `JWT_SECRET`
- `DATABASE_URL` (contains password)
- `ADMIN_PASSWORD`

### Monitoring & Alerting

Each service exposes Prometheus metrics at `/metrics`. Recommended alerts:

| Alert | Condition | Severity |
|-------|-----------|----------|
| Service down | `/ready` returns non-200 for >30s | Critical |
| High error rate | `http_requests_total{status=~"5.."}` > 1% of total | High |
| High latency | `http_request_duration_seconds` p99 > 2s | High |
| DNO cache miss rate | `cache_misses_total{cache="dno"}` > 50% | Medium |
| Query log buffer backing up | Worker not flushing | Medium |
| Bulk job failures | `bulk_jobs.status = 'failed'` accumulating | Medium |
| Disk space (SQLite) | Database file > 1GB | Low |

### Incident Runbook

**Service won't start:**
1. Check logs: `docker logs <container>` or service stdout
2. Common causes: `DATABASE_URL` not set, DB unreachable, port already in use
3. Verify DB connectivity: `psql $DATABASE_URL -c "SELECT 1"`

**High latency on queries:**
1. Check `/metrics` -- is `dno_query_duration_seconds` elevated?
2. Check cache hit rate -- is `cache_hits_total` much lower than `cache_misses_total`?
3. Check DB: `SELECT count(*) FROM dno_numbers` -- is the table unexpectedly large?
4. Check if the composite index exists: `\d dno_numbers` in psql

**Bulk jobs stuck in "processing":**
1. Check worker logs for errors
2. Worker retries failed jobs up to 3 times automatically
3. Manual fix: `UPDATE bulk_jobs SET status = 'pending' WHERE status = 'processing' AND created_at < NOW() - INTERVAL '10 minutes'`

**Query log table too large:**
1. Worker cleans up entries >90 days automatically (hourly)
2. Manual cleanup: `DELETE FROM query_log WHERE queried_at < NOW() - INTERVAL '90 days'`
3. For PostgreSQL, consider partitioning by month if >100M rows

**Rollback a deployment:**
1. Deploy the previous container image / binary
2. Migrations are additive (`CREATE IF NOT EXISTS`, `ADD COLUMN IF NOT EXISTS`) -- no rollback DDL needed
3. If a migration added a column that the old code doesn't know about, it's harmless (extra column is ignored)
