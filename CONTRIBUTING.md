# Contributing to MicroURL

Thank you for your interest in contributing to **MicroURL**. Please read this guide before opening a pull request.

---

## Development Setup

### Prerequisites

- [Go 1.24+](https://go.dev/)
- [Node.js 20+](https://nodejs.org/)
- [Docker & Docker Compose](https://docs.docker.com/compose/) (or local PostgreSQL 16 + Upstash Redis)

### Running Locally

```bash
# 1. Start PostgreSQL and local Redis + Upstash REST proxy
docker compose up -d

# 2. Backend (in one terminal)
cd backend
cp .env.example .env
# Update DATABASE_URL in .env to match docker-compose.yml if needed
go mod download
go run ./cmd/api

# 3. Frontend (in a separate terminal)
cd frontend
npm install
VITE_API_BASE_URL=http://localhost:8080 npm run dev
```

---

## Architecture

The backend follows **Hexagonal Architecture** (Ports & Adapters):

```text
backend/
├── cmd/api/main.go             # Composition root & HTTP server lifecycle
└── internal/
    ├── domain/                 # Pure entities (Link) and domain errors — zero external imports
    ├── application/            # Use cases (CreateLink, ResolveLink, SlugGenerator) & port interfaces
    ├── infrastructure/
    │   ├── http/               # net/http handlers, router, rate limiter, CORS, OpenAPI/Scalar
    │   ├── persistence/        # PostgreSQL LinkRepository implementation (pgx/v5)
    │   ├── cache/              # Upstash Redis Cache implementation (REST API)
    │   └── security/           # Cloudflare Turnstile verifier
    └── config/                 # Environment loading and database pool setup
```

The frontend is organized by feature:

```text
frontend/src/
├── app/                        # Root App router shell
├── features/
│   ├── shorten/                # API client, Zustand store, HomePage, ShortenForm, CaptchaField, ResultDisplay
│   └── errors/                 # 404, 410, and 500 diagnostic pages
├── shared/                     # Reusable components (TopNavBar, Footer, ShortcutsModal, LoadingSpinner), hooks, utils
├── styles/                     # Global stylesheet and dark/light CSS theme tokens
└── __tests__/                  # Vitest + React Testing Library tests
```

---

## Coding Principles

### General
- **Comments explain *why*, not *what*:** Document non-obvious invariants, edge cases, and trade-offs.
- **Keep code focused and linear:** Avoid over-engineering or duplicating logic across handlers or components.

### Backend (Go)
- Preserve Hexagonal Architecture boundaries: `internal/domain/` must stay dependency-free; `internal/application/` must depend only on `domain/` and standard library; infrastructure adapters implement application ports.
- Always wrap errors with context (`fmt.Errorf("...: %w", err)`) and match sentinel errors with `errors.Is` rather than `==`.
- Enforce request timeouts (`context.WithTimeout`) on database, cache, and external HTTP calls.
- Never log secrets (`DATABASE_URL`, `TURNSTILE_SECRET_KEY`) or commit compiled binaries (`api`, `api.exe`).

### Frontend (TypeScript / React)
- Enable TypeScript `strict` mode and avoid `any`.
- Keep Zustand focused on client/UI state for the shortening flow (`useShortenStore`).
- Handle loading, validation, and HTTP error states (`400`, `403`, `409`, `429`, `500`) explicitly in the UI.
- Maintain light/dark theme parity using the CSS variables in `src/styles/globals.css`.
- **npm Supply-Chain Policy:** Keep `min-release-age=30` and `save-exact=true` in `.npmrc` and `frontend/.npmrc` (requires npm 11.10.0+). Pin exact dependency versions in `frontend/package.json` (never use `^` or `~`), commit `frontend/package-lock.json`, and run `npm audit` and `npm audit signatures` whenever dependencies change. Prefer `npm ci --ignore-scripts` in CI environments.

---

## Branch Naming

| Type | Pattern | Example |
|------|---------|---------|
| Feature | `feat/<short-description>` | `feat/link-expiration-picker` |
| Bug fix | `fix/<short-description>` | `fix/wrapped-error-status-codes` |
| Chore / CI | `chore/<short-description>` | `chore/vite-migration` |
| Documentation | `docs/<short-description>` | `docs/api-reference` |

---

## Pull Request Expectations

- Keep pull requests focused on a single concern.
- Run backend checks before opening a PR:
  ```bash
  cd backend && go vet ./... && go build ./cmd/api
  ```
- Run frontend checks before opening a PR:
  ```bash
  cd frontend && npm run typecheck && npm run lint && npm test && npm run build
  ```
- Keep `docs/api.md`, `backend/internal/infrastructure/http/openapi.go`, and `backend/.env.example` aligned whenever endpoints or environment variables change.
- Never commit `.env` files, API keys, or compiled binaries.

---

## Commit Messages

Follow [Conventional Commits](https://www.conventionalcommits.org):

```text
feat(frontend): add expiration selector to shorten form
fix(backend): use errors.Is when checking wrapped domain errors
chore(infra): add docker-compose for postgres and redis
docs(api): document QR code and health check endpoints
```
