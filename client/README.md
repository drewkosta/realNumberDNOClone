# FakeNumber DNO - Frontend

React 19 + TypeScript + Vite frontend for the FakeNumber DNO management portal.

## Tech Stack

- **React 19** with `use()` hook for context
- **TypeScript** with strict type checking
- **Vite** for dev server (HMR) and production builds
- **Tailwind CSS 4** via `@tailwindcss/vite` plugin
- **TanStack Query** for server state management
- **React Router** for client-side routing
- **Recharts** for dashboard charts
- **Axios** with JWT interceptor and auto-refresh on 401
- **Lucide React** for icons

## Pages

| Page | Path | Description |
|------|------|-------------|
| Login | `/login` | JWT auth with demo credential display |
| Dashboard | `/` | Analytics charts and stat cards |
| Query | `/query` | Single and bulk DNO lookups |
| DNO List | `/numbers` | Browse, search, add/remove numbers |
| Bulk Ops | `/bulk` | CSV upload and flat file export |
| Analyzer | `/analyzer` | Upload CDR data for fraud analysis |
| Compliance | `/compliance` | FCC compliance report with recommendations |
| Webhooks | `/webhooks` | Manage webhook subscriptions |
| ROI | `/roi` | Calculate DNO enforcement value |
| Audit | `/audit` | Activity trail |
| Admin | `/admin` | User/API key management, ITG ingest, mock integrations |

## Development

```bash
npm install       # Install dependencies
npm run dev       # Start dev server on :5173
npm run build     # Production build to dist/
npm test          # Run Vitest tests (36 tests)
npm run test:watch # Watch mode
npm run lint      # ESLint with type-aware rules
```

The dev server proxies `/api` and `/health` to `http://localhost:8080` (the gateway).

## Testing

36 tests using Vitest + Testing Library:

- **auth.test.tsx** -- AuthContext login/logout/restore
- **types.test.ts** -- TypeScript type shape validation
- **components.test.tsx** -- ErrorBoundary, Layout nav by role, ServiceStatus
- **pages.test.tsx** -- All 11 pages render correctly, role-based UI access
- **app.test.tsx** -- Route guards and redirects

## Role-Based UI

| Feature | admin | org_admin | operator | viewer |
|---------|-------|-----------|----------|--------|
| Dashboard, Query, Analyzer, Compliance, ROI, Audit | yes | yes | yes | yes |
| DNO List (view) | yes | yes | yes | yes |
| DNO List (add/remove) | yes | yes | yes | no |
| Bulk Operations | yes | yes | yes | no |
| Webhooks | yes | yes | no | no |
| Admin | yes | no | no | no |
