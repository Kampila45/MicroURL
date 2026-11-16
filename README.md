# MicroURL

> **Shorten links, cleanly.**

A fast, production-ready URL shortener monorepo built with a **Go** backend following **Hexagonal Architecture** (Ports & Adapters), backed by **PostgreSQL** and **Upstash Redis**, protected with **Cloudflare Turnstile** and per-IP rate limiting, and paired with a **TypeScript + React** web frontend.

---

## Demo

https://github.com/user-attachments/assets/75243624-c2a9-4e87-8e42-2b852fe1d78a

---

## Features

- **Instant URL Shortening & Custom Aliases** — Generates collision-checked 8-character alphanumeric slugs via `crypto/rand` (retrying on database conflicts) or validates lowercase custom aliases (`[a-z0-9_-]`), with optional RFC3339 link expiration (`expires_at`).
- **Cache-First Redirects & Privacy-Conscious Analytics** — `GET /{slug}` checks Upstash Redis before falling back to PostgreSQL, automatically evicting expired links, while atomically recording UTC daily click aggregates (`GET /api/links/{slug}/stats`) without storing visitor IPs.
- **QR Code Generation** — Generates downloadable 256×256 QR codes as PNG and SVG in the UI, plus a PNG endpoint (`GET /api/qr/{slug}`) for direct embedding — without inflating click analytics.
- **Bot & Abuse Protection** — Cloudflare Turnstile CAPTCHA verification on link creation (with explicit development-mode bypass when no secret key is set) and per-IP token-bucket rate limiting.
- **Interactive API Docs** — Built-in OpenAPI 3.1.0 specification (`/openapi.json`) and Scalar API Reference (`/docs`), plus deep `/health` checks for both PostgreSQL and Upstash Redis.
- **Light & Dark Theme** — System-aware theme toggle persisted in `localStorage`.

---

## Tech Stack

| Layer | Technology |
|---|---|
| Backend API | Go 1.24 (`net/http`, Hexagonal Architecture) |
| Database | PostgreSQL (`pgx/v5`) |
| Cache | Upstash Redis (REST API) |
| Rate Limiting | Token bucket per IP (`golang.org/x/time/rate`) |
| CAPTCHA | Cloudflare Turnstile (`siteverify` + `@marsidev/react-turnstile`) |
| QR Codes | `skip2/go-qrcode` (backend PNG) + `qrcode` (frontend) |
| API Docs | OpenAPI 3.1.0 + Scalar |
| Frontend | React 19 + TypeScript + Vite + Tailwind CSS v4 + Zustand + React Router |
| Architecture | Monorepo (`backend/` + `frontend/`) |

---

## Repository Structure

```text
MicroURL/
├── backend/
│   ├── cmd/api/main.go                 # Application entrypoint & graceful shutdown
│   ├── internal/
│   │   ├── domain/                     # Link entity & domain errors
│   │   ├── application/                # Use cases (CreateLink, ResolveLink, SlugGenerator) & ports
│   │   ├── infrastructure/
│   │   │   ├── http/                   # Router, handlers, rate limiter, OpenAPI & Scalar UI
│   │   │   ├── persistence/            # PostgreSQL repository adapter
│   │   │   ├── cache/                  # Upstash Redis cache adapter
│   │   │   └── security/               # Cloudflare Turnstile verifier
│   │   └── config/                     # Env config & Postgres connection pool
│   ├── .env.example
│   └── go.mod
├── frontend/
│   ├── src/
│   │   ├── app/                        # Root App router shell
│   │   ├── features/
│   │   │   ├── shorten/                # HomePage, ShortenForm, CaptchaField, ResultDisplay, API client, Zustand store
│   │   │   └── errors/                 # 404, 410, and 500 diagnostic pages
│   │   ├── shared/                     # TopNavBar, Footer, ShortcutsModal, LoadingSpinner, hooks, QR/CSS utils
│   │   ├── styles/                     # Tailwind v4 theme tokens (dark/light)
│   │   ├── __tests__/                  # Vitest + React Testing Library integration tests
│   │   └── main.tsx
│   ├── index.html
│   └── package.json
├── docker/
│   └── init-db.sql                     # PostgreSQL schema initialization (links table + index)
├── docs/
│   ├── api.md                          # REST API reference
│   └── architecture.md                 # Hexagonal architecture & request flows
├── docker-compose.yml                  # Local PostgreSQL + Redis & Upstash REST proxy stack
├── render.yaml                         # Render deployment blueprint
└── README.md
```

---

## Local Development

### Prerequisites

- [Go 1.24+](https://go.dev/)
- [Node.js 20+](https://nodejs.org/)
- [Docker & Docker Compose](https://docs.docker.com/compose/) (or local PostgreSQL 16 + Upstash Redis)

### 1. Start PostgreSQL & Upstash Redis (Local Emulator)

```bash
docker compose up -d
```

This starts PostgreSQL (automatically creating the `links` table via `docker/init-db.sql`) and a local Redis + Upstash REST proxy on `http://localhost:8079`.

### 2. Configure & Run the Backend

```bash
cd backend
cp .env.example .env
```

Update `backend/.env` if needed (default `docker-compose.yml` credentials):

```env
DATABASE_URL=postgres://microurl:microurl_dev_password@localhost:5432/microurl?sslmode=disable
BASE_URL=http://localhost:8080
PORT=8080
APP_ENV=development
RATE_LIMIT_RPS=10.0
RATE_LIMIT_BURST=20
TRUSTED_PROXIES=
UPSTASH_REDIS_REST_URL=http://localhost:8079
UPSTASH_REDIS_REST_TOKEN=microurl_dev_redis_token
CACHE_TTL=3600
```

Run the API server:

```bash
go mod download
go run ./cmd/api
```

- Interactive API docs (Scalar): `http://localhost:8080/docs`
- OpenAPI spec: `http://localhost:8080/openapi.json`
- Health check: `http://localhost:8080/health`

### 3. Configure & Run the Frontend

```bash
cd frontend
npm install
```

Create `frontend/.env.local`:

```env
VITE_API_BASE_URL=http://localhost:8080
# Optional in development (enables Turnstile widget when set):
# VITE_TURNSTILE_SITE_KEY=your_site_key
```

Start the dev server:

```bash
npm run dev
```

### Quality & Verification

**Backend:**
```bash
cd backend
go vet ./...
go test -race ./...
go build ./cmd/api
```

**Frontend:**
```bash
cd frontend
npm run typecheck
npm run lint
npm test
npm run build
```

---

## API Overview

See [docs/api.md](./docs/api.md) for full request/response schemas and [docs/architecture.md](./docs/architecture.md) for system design.

```text
GET  /health                      Check API, PostgreSQL, and Upstash Redis health
GET  /docs                        Interactive Scalar API documentation
GET  /openapi.json                OpenAPI 3.1.0 JSON specification
POST /api/links                   Create a short link (optional custom_slug, expires_at + Turnstile)
GET  /api/links/{slug}/stats      Aggregate UTC click totals and daily trend (7d | 30d)
GET  /api/qr/{slug}               Generate a 256x256 PNG QR code for a short link
GET  /{slug}                      Resolve slug, record click, and redirect (302 Found)
```

---

## Security Summary

- **CAPTCHA & IP Trust:** Link creation verifies Cloudflare Turnstile tokens server-side (`TURNSTILE_SECRET_KEY` is mandatory when `APP_ENV=production`). Both Turnstile and the per-IP rate limiter (`RATE_LIMIT_RPS`, `RATE_LIMIT_BURST`, returning `429 Too Many Requests` with `Retry-After: 1`) share a single `ClientIPResolver` that ignores `X-Forwarded-For`/`X-Real-IP` unless the peer IP matches `TRUSTED_PROXIES`.
- **URL Validation:** `POST /api/links` caps bodies at 16 KiB, allows only public `http://`/`https://` URLs up to 2048 characters (rejecting embedded credentials and `localhost`/private hosts), and requires optional `expires_at` RFC3339 timestamps to be in the future.
- **Secret Hygiene:** Database credentials and `TURNSTILE_SECRET_KEY` live exclusively in backend environment variables.

See [SECURITY.md](./SECURITY.md) for the full security policy.

---

## License

[MIT](./LICENSE)
