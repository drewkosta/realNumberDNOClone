# FakeNumber DNO

A full-stack clone of [Somos RealNumber DNO](https://www.somos.com/realnumber) (Do Not Originate), the telecom industry's authoritative database for preventing illegal robocalls and caller ID spoofing.

This system maintains a database of phone numbers that should never appear as the originating caller ID on outbound calls. Carriers query the DNO database in real-time to block spoofed calls before they reach end users.

## Table of Contents

- [Architecture](#architecture)
- [Quick Start](#quick-start)
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

```
┌─────────────────┐     ┌──────────────────────────────────────────────────┐
│  React Client   │────▶│  Go HTTP Server (chi)                            │
│  Vite + TS +    │     │                                                  │
│  Tailwind       │     │  ┌──────────┐  ┌───────────┐  ┌──────────────┐  │
│  TanStack Query │     │  │ Auth     │  │ DNO       │  │ Analytics    │  │
│  Recharts       │     │  │ Service  │  │ Service   │  │ (cached)     │  │
└─────────────────┘     │  └────┬─────┘  └─────┬─────┘  └──────┬───────┘  │
                        │       │              │               │          │
                        │  ┌────▼──────────────▼───────────────▼───────┐  │
                        │  │  DB Abstraction Layer                     │  │
                        │  │  PostgreSQL (default) | SQLite (local)    │  │
                        │  │  Reader/Writer split  | $N → ? rewrite   │  │
                        │  └───────────────────────────────────────────┘  │
                        │                                                  │
                        │  ┌──────────────┐  ┌─────────────┐  ┌────────┐  │
                        │  │ Async Query  │  │ LRU Cache   │  │ Job    │  │
                        │  │ Log Writer   │  │ (DNO +      │  │ Worker │  │
                        │  │ (buffered)   │  │  Analytics) │  │ (bulk) │  │
                        │  └──────────────┘  └─────────────┘  └────────┘  │
                        └──────────────────────────────────────────────────┘
```

**Backend:** Go 1.25, chi router, JWT auth, bcrypt, SQLite/PostgreSQL
**Frontend:** React 19, TypeScript, Vite, Tailwind CSS 4, TanStack Query, Recharts
**Observability:** Structured JSON logging (slog), Prometheus metrics, request IDs

---

## Quick Start

### Prerequisites

- Go 1.23+ (CGO enabled for SQLite)
- Node.js 18+
- (Optional) Docker for local PostgreSQL

### First Run

```bash
# Install frontend dependencies
make install

# Start backend (with seed data) + frontend dev server
make dev-seed
```

This starts:
- **Backend** on `http://localhost:8080` (SQLite, seeded with 1300+ mock DNO numbers)
- **Frontend** on `http://localhost:5173` (Vite dev server with hot reload)

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

### Using Local PostgreSQL

```bash
# Start PostgreSQL container
make pg-up

# Run with PostgreSQL instead of SQLite
make run-local-pg-seed

# Stop PostgreSQL
make pg-down
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

These map to telecom industry roles:

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

Six environments with tailored defaults. Set via `--env` flag:

| Environment | DB Driver | DB File/DSN | JWT Secret | Rate Limit | Cache TTL | Seed |
|-------------|-----------|-------------|------------|------------|-----------|------|
| `local` | SQLite | `realnumber_local.db` | hardcoded | disabled | 30s | yes |
| `dev` | SQLite | `realnumber_dev.db` | hardcoded | 100 rps | 30s | yes |
| `testing` | SQLite | `realnumber_test.db` | hardcoded | disabled | disabled | yes |
| `staging` | **PostgreSQL** | `DATABASE_URL` required | `JWT_SECRET` required | 500 rps | 60s | no |
| `pre-prod` | **PostgreSQL** | `DATABASE_URL` required | `JWT_SECRET` required | 1000 rps | 60s | no |
| `production` | **PostgreSQL** | `DATABASE_URL` required | `JWT_SECRET` required | 2000 rps | 45s | no |

### Environment Variables

All config values can be overridden via environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `PORT` | Server port | `8080` |
| `DB_DRIVER` | `sqlite` or `postgres` | `postgres` |
| `DB_PATH` | SQLite file path | `realnumber_local.db` |
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:pass@host:5432/db?sslmode=disable` |
| `JWT_SECRET` | HMAC signing key (required staging+) | `your-secret-here` |
| `LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` |
| `CORS_ORIGIN` | Allowed origin | `https://app.example.com` |
| `ADMIN_PASSWORD` | Override default admin password | `secure-password` |
| `RATE_LIMIT_RPS` | Requests per second per org | `1000` |
| `ALLOW_SEED` | Override seed permission | `true` |

---

## API Reference

Two authentication methods are supported:

- **JWT Bearer token** (portal users): `Authorization: Bearer <token>` -- full API access
- **API key** (external integrations): `X-API-Key: <key>` -- query endpoints only

Public endpoints: `/health`, `/metrics`, `/api/auth/login`

### Authentication

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/auth/login` | Login with email/password, returns JWT (rate limited: 5/min per IP) |
| `GET` | `/api/auth/me` | Get current authenticated user |

**Login request:**
```json
{ "email": "admin@realnumber.local", "password": "admin123" }
```

**Login response:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "user": { "id": 1, "email": "admin@realnumber.local", "firstName": "System", "lastName": "Admin", "role": "admin", "orgId": 1 }
}
```

### DNO Queries

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/dno/query?phoneNumber=5551234567&channel=voice` | Single number DNO lookup |
| `POST` | `/api/dno/query/bulk` | Bulk lookup (up to 1000 numbers) |

**Single query response:**
```json
{
  "phoneNumber": "5551234567",
  "isDno": true,
  "dataset": "subscriber",
  "channel": "voice",
  "status": "active",
  "lastUpdated": "2026-01-15T10:30:00Z"
}
```

**Bulk query request:**
```json
{ "phoneNumbers": ["5551234567", "8001234567"], "channel": "voice" }
```

**Bulk query response:**
```json
{
  "results": [ ... ],
  "total": 2,
  "hits": 1,
  "misses": 1
}
```

### DNO Number Management

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/dno/numbers` | Add number to subscriber DNO list |
| `DELETE` | `/api/dno/numbers?phoneNumber=...&channel=voice` | Remove number (subscriber set only, own org) |
| `GET` | `/api/dno/numbers?page=1&pageSize=25&dataset=subscriber&status=active&channel=voice&search=555` | List numbers with filtering/pagination |

**Add number request:**
```json
{
  "phoneNumber": "5551234567",
  "numberType": "local",
  "channel": "voice",
  "reason": "Customer service inbound only"
}
```

### Bulk Operations

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/dno/bulk-upload` | Upload CSV for async background processing |
| `GET` | `/api/dno/bulk-job?jobId=1` | Check bulk job status/progress |
| `GET` | `/api/dno/export` | Download full DNO database as CSV flat file |

Bulk uploads are processed asynchronously by a background worker. The upload endpoint returns `202 Accepted` with a `jobId` immediately:

```json
{
  "jobId": 1,
  "status": "pending",
  "totalRecords": 500,
  "message": "Bulk upload queued for background processing"
}
```

**CSV format:**
```
phone_number,reason
5551234567,Customer service inbound only
8001234567,Toll-free advertising number
```

**Export flat file format** (compatible with RealNumber DNO):
```
phone_number,last_update_date,status_flag,dataset,channel,number_type
5551234567,2026-01-15T10:30:00Z,1,subscriber,voice,local
8001234567,2026-01-14T08:00:00Z,0,auto,voice,toll_free
```

Status flag: `0` = Auto Set (system-determined), `1` = Subscriber Set (manually flagged)

### Analytics

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/analytics` | Dashboard analytics (cached, org-scoped for non-admins) |

**Response:**
```json
{
  "totalDnoNumbers": 1300,
  "activeNumbers": 1300,
  "byDataset": { "auto": 800, "subscriber": 300, "itg": 50, "tss_registry": 150 },
  "byChannel": { "voice": 1100, "text": 150, "both": 50 },
  "byNumberType": { "local": 750, "toll_free": 550 },
  "totalQueries24h": 1005,
  "hitRate24h": 15.2,
  "queriesByHour": [ { "hour": "2026-03-28 14:00", "count": 42 }, ... ],
  "recentAdditions": 12,
  "recentRemovals": 3
}
```

### Audit Log

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/audit-log?page=1&pageSize=25` | Paginated audit trail (org-scoped for non-admins) |

### Admin

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/admin/users` | Create user (admin only, min 8-char password, validated role) |
| `POST` | `/api/admin/api-keys?orgId=N` | Generate API key for an org (returns raw key once) |
| `DELETE` | `/api/admin/api-keys?orgId=N` | Revoke API key for an org |

**Generate API key response:**
```json
{
  "orgId": 3,
  "apiKey": "dno_9ff41df336e9eda0...",
  "note": "Store this key securely. It cannot be retrieved again."
}
```

### Infrastructure

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/health` | Health check with DB ping (returns `ok` or `degraded`) |
| `GET` | `/metrics` | Prometheus metrics scrape endpoint |

---

## Frontend

### Pages

| Page | Path | Description |
|------|------|-------------|
| Login | `/login` | JWT authentication |
| Dashboard | `/` | Analytics overview with charts (queries/hour, dataset distribution, hit rate) |
| Query Numbers | `/query` | Single and bulk DNO lookups with color-coded hit/miss results |
| DNO List | `/numbers` | Full CRUD with filtering by dataset/channel/status, pagination, search |
| Bulk Operations | `/bulk` | CSV upload (async) and flat file export |
| Audit Log | `/audit` | Paginated activity trail |
| Admin | `/admin` | User creation with role selection (admin only) |

### Tech Stack

- **Vite** for dev server with HMR and production builds
- **React Router** for client-side routing
- **TanStack Query** for server state management with automatic refetching
- **Tailwind CSS 4** for styling (via `@tailwindcss/vite` plugin)
- **Recharts** for dashboard bar charts and pie charts
- **Lucide React** for icons
- **Axios** for HTTP client with JWT interceptor and 401 auto-redirect

---

## Database

### DNO Data Model

The system mirrors the four datasets from the real Somos RealNumber DNO product:

| Dataset | Source | Description |
|---------|--------|-------------|
| **Auto Set** | System-generated | Unassigned, disconnected, and spare numbers from NANP/TFNRegistry/NPAC |
| **Subscriber Set** | Manually flagged by org owners | Inbound-only numbers (hotlines, IVRs, conference bridges, vanity numbers) |
| **ITG Set** | Industry Traceback Group | Numbers identified through traceback as spoofed for illegal/fraudulent calls |
| **TSS Registry Set** | TSS Registry | Non-text-enabled toll-free numbers (text DNO only) |

### Schema

```
organizations    users           dno_numbers       query_log
─────────────    ──────          ────────────       ──────────
id               id              id                 id
name             email           phone_number       org_id
org_type         password_hash   dataset            phone_number
spid             first_name      number_type        result (hit/miss)
resp_org_id      last_name       channel            channel
api_key          role            status             queried_at
                 org_id          reason
                 active          added_by_org_id

audit_log        bulk_jobs
──────────       ──────────
id               id
user_id          org_id
org_id           user_id
action           job_type
entity_type      status
entity_id        total_records
details          processed_records
created_at       success_count
                 error_count
                 file_name
                 result_summary
                 completed_at
```

### Dual-Driver Support

All service SQL is written in **PostgreSQL dialect** (`$1, $2, ...` placeholders, `date_trunc`, `NOW()`). For SQLite (local dev), a transparent adapter rewrites:

- `$N` placeholders to `?`
- `date_trunc('hour', col)` to `strftime('%Y-%m-%d %H:00', col)`
- `NOW()` to `CURRENT_TIMESTAMP`

This is handled by `DB.Q()`, `DB.QTimeTrunc()`, and `DB.QNow()` in `internal/db/db.go`.

### SQLite Reader/Writer Split

In SQLite mode, the DB layer opens two separate connection pools:

- **Writer** (`MaxOpenConns=1`): Serializes all writes to avoid `SQLITE_BUSY`
- **Reader** (`MaxOpenConns=10`): Concurrent reads via WAL mode

In PostgreSQL mode, both point to the same connection pool (`MaxOpenConns=25`).

### Indexes

```sql
-- Hot-path composite index for DNO lookups
CREATE INDEX idx_dno_lookup ON dno_numbers(phone_number, channel, status);

-- Filtering indexes
CREATE INDEX idx_dno_dataset ON dno_numbers(dataset);
CREATE INDEX idx_dno_org ON dno_numbers(added_by_org_id);

-- Query log time-series
CREATE INDEX idx_query_log_time ON query_log(queried_at);
CREATE INDEX idx_query_log_org ON query_log(org_id);

-- Audit log
CREATE INDEX idx_audit_time ON audit_log(created_at);
CREATE INDEX idx_audit_org ON audit_log(org_id);
```

---

## Scaling & Performance

### What's Implemented

| Feature | Description |
|---------|-------------|
| **Async query logging** | DNO lookups no longer do synchronous INSERTs. Query log entries are buffered in memory and batch-flushed to DB on a timer/threshold (configurable per environment). |
| **In-process LRU cache** | DNO lookup results are cached with configurable TTL (30-60s). Cache is invalidated on add/remove. Eliminates DB reads for repeated queries. |
| **Analytics caching** | Analytics summary is cached (30-60s TTL) to avoid repeated full-table aggregations. |
| **Batch bulk queries** | Bulk lookups use a single `WHERE phone_number IN (...)` query instead of N+1 individual queries. |
| **Background job worker** | CSV bulk uploads are processed asynchronously by a polling worker, not inline in the HTTP request. |
| **Rate limiting** | Per-org rate limiting on API endpoints (configurable RPS). Login endpoint rate limited to 5/min per IP. |
| **HTTP server timeouts** | `ReadHeaderTimeout: 5s`, `ReadTimeout: 10s`, `WriteTimeout: 30s`, `IdleTimeout: 120s` |
| **Streaming CSV export** | Export streams rows directly from DB to response writer, not buffered in memory. |
| **Composite indexes** | `(phone_number, channel, status)` covers the exact hot-path query. |
| **Reader/Writer split** | SQLite concurrent reads via WAL; PostgreSQL shared pool. |

### Seed Data

The `--seed` flag generates realistic mock data:

| Data | Count | Details |
|------|-------|---------|
| Organizations | 8 | Carriers, gateway providers, resp orgs |
| Users | 10 | Various roles across organizations |
| DNO Numbers | ~1,330 | Across all 4 datasets, voice/text/both channels, local and toll-free |
| Query Logs | 2,000 | Spread over 48 hours with ~17% hit rate (matching real-world) |
| Audit Logs | 200 | Add/remove actions over 30 days |

---

## Observability

### Structured Logging

All logs are JSON via Go's `slog` package:

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

Log level is configurable per environment (`debug` for local, `error` for production).

### Prometheus Metrics

Available at `GET /metrics`:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | counter | method, path, status | Total HTTP requests |
| `http_request_duration_seconds` | histogram | method, path | Request latency |
| `dno_query_total` | counter | channel, result | DNO lookups (hit/miss) |
| `dno_query_duration_seconds` | histogram | | DNO lookup latency |
| `dno_bulk_query_size` | histogram | | Batch sizes for bulk queries |
| `cache_hits_total` | counter | cache (dno, analytics) | Cache hit count |
| `cache_misses_total` | counter | cache (dno, analytics) | Cache miss count |

### Health Check

`GET /health` pings the database and returns:

```json
{ "status": "ok", "env": "local", "db": "ok" }
```

Returns `"status": "degraded"` with error details if the DB is unreachable.

---

## Project Structure

```
.
├── cmd/server/main.go              # Entrypoint: config, DB, cache, workers, server
├── internal/
│   ├── api/
│   │   ├── router.go               # Chi router, middleware chain, route definitions
│   │   ├── handlers.go             # HTTP handlers (request parsing, response writing)
│   │   └── middleware.go           # JWT auth middleware, admin-only guard
│   ├── service/
│   │   ├── dno.go                  # DNO business logic (query, add, remove, analytics)
│   │   └── auth.go                 # Auth business logic (login, JWT, user CRUD)
│   ├── db/
│   │   ├── db.go                   # DB init, migrations, reader/writer split, Q() adapter
│   │   └── seed.go                 # Mock data seeder for local development
│   ├── config/config.go            # Environment-based configuration
│   ├── models/models.go            # Domain types, request/response structs, validators
│   ├── cache/cache.go              # Generic TTL cache with concurrent-safe eviction
│   ├── querylog/writer.go          # Async buffered query log writer
│   ├── jobs/worker.go              # Background job worker for bulk uploads
│   └── metrics/metrics.go          # Prometheus metric definitions and HTTP middleware
├── client/
│   ├── src/
│   │   ├── App.tsx                 # Router and auth provider setup
│   │   ├── api.ts                  # Axios client with JWT interceptor
│   │   ├── auth.tsx                # Auth context (login/logout/token storage)
│   │   ├── types.ts                # TypeScript type definitions
│   │   ├── components/Layout.tsx   # Sidebar navigation shell
│   │   └── pages/                  # Dashboard, Query, Numbers, Bulk, Audit, Admin, Login
│   ├── index.html
│   ├── vite.config.ts              # Vite config with Tailwind plugin and API proxy
│   └── package.json
├── docker-compose.yml              # Local PostgreSQL for development
├── Makefile                        # Dev, build, and per-environment targets
├── go.mod / go.sum
└── .gitignore
```

---

## Makefile Targets

### Development

| Command | Description |
|---------|-------------|
| `make dev` | Start backend + frontend (no seed) |
| `make dev-seed` | Start backend + frontend (seed mock data on first run) |
| `make server` | Backend only |
| `make client` | Frontend only |
| `make install` | Install frontend npm dependencies |

### Per-Environment

| Command | Description |
|---------|-------------|
| `make run-local` | Local with SQLite |
| `make run-local-seed` | Local with SQLite + seed |
| `make run-local-pg` | Local with PostgreSQL (requires `make pg-up` first) |
| `make run-local-pg-seed` | Local with PostgreSQL + seed |
| `make run-dev` | Dev environment |
| `make run-testing` | Testing environment |
| `make run-staging` | Staging (requires `DATABASE_URL` + `JWT_SECRET`) |
| `make run-pre-prod` | Pre-prod (requires `DATABASE_URL` + `JWT_SECRET`) |
| `make run-production` | Production (requires `DATABASE_URL` + `JWT_SECRET`, seed blocked) |

### Docker

| Command | Description |
|---------|-------------|
| `make pg-up` | Start local PostgreSQL container |
| `make pg-down` | Stop and remove containers |

### Build & Test

| Command | Description |
|---------|-------------|
| `make build` | Build Go binary + frontend production bundle |
| `make test` | Run Go tests |
| `make clean` | Remove build artifacts and database files |
