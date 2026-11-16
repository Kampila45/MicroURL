# MicroURL Backend

Go 1.24 Hexagonal Architecture API service for **MicroURL**, built with standard-library `net/http`, **PostgreSQL** (`pgx/v5`), **Upstash Redis** (REST API), and **Cloudflare Turnstile**.

### 1. Domain Layer & Input Validation (`internal/domain/`)

- **Zero-dependency entities:** `Link` (`ID`, `Slug`, `OriginalURL`, `CreatedAt`, `ExpiresAt`, `ClickCount`) with `IsExpired()` / `EffectiveStatus()`, `LinkStats`, `DailyClickCount`, and sentinel domain errors (`ErrLinkNotFound`, `ErrLinkExpired`, `ErrInvalidSlug`, `ErrInvalidAlias`, `ErrAliasConflict`).
- **Destination URL hardening (`ValidateDestinationURL`):** Enforces `http`/`https` schemes, caps length at `MaxURLLength` (`2048` chars), validates port bounds, rejects control characters and embedded credentials, and blocks `localhost`, `.local`, `.internal`, loopback, private RFC-1918, and link-local IP targets.
- **Custom alias validation (`ValidateCustomAlias`):** Normalizes caller-chosen endings to lowercase ASCII (`3–64` chars, `[a-z0-9_-]`) and blocks `reservedAliases` (`api`, `health`, `docs`, `openapi.json`, `stats`, error routes).

### 2. Application Use Cases & Ports (`internal/application/`)

- **Port boundaries (`LinkRepository`, `Cache`):** Isolate business workflows from PostgreSQL, Redis REST, and HTTP types (`ErrCacheMiss` separates misses from connectivity failures).
- **Slug generation (`SlugGenerator`):** Generates collision-checked 8-character alphanumeric slugs via `crypto/rand` bounded to `maxRetries = 10` checks against `LinkRepository.Exists`.
- **Use cases:**
  - `CreateLinkUseCase`: Re-enforces destination/alias validation, accepts custom aliases (`409 Conflict` if taken) or retries random slugs, persists to PostgreSQL, and warms cache non-fatally.
  - `ResolveLinkUseCase`: Checks `Cache` (`link:slug:{slug}`) before PostgreSQL, evicts expired entries on detection (`ErrLinkExpired`), falls back cleanly on cache errors, and records clicks (`CountClick: true`, skipped on QR generation).
  - `GetLinkStatsUseCase`: Returns zero-filled `7d` or `30d` continuous UTC daily aggregates alongside link metadata.

### 3. Persistence & Upstash Redis Cache (`internal/infrastructure/persistence/`, `internal/infrastructure/cache/`)

- **PostgreSQL adapter (`PostgresRepository`):** Uses `pgx/v5/stdlib` (`simple_protocol`, pooled in `internal/config/database.go`), idempotent startup provisioning via `EnsureSchema`, `pgconn.PgError` (`23505`) duplicate translation, `sql.ErrNoRows` → `ErrLinkNotFound`, and an atomic `RecordClick` CTE updating `links.click_count` and `link_click_daily`.
- **Upstash Redis adapter (`UpstashRedisAdapter`):** Standard-library HTTP REST client (`net/http` + `encoding/json`, 2s timeout) using `UPSTASH_REDIS_REST_URL` and Bearer `UPSTASH_REDIS_REST_TOKEN` (`GET`, `SET ... EX <CACHE_TTL>`, `DEL`). Translates empty credentials cleanly to `ErrCacheNotConfigured` so redirects continue operating in degraded-cache mode.

### 4. HTTP Presentation, Security & Composition (`internal/infrastructure/http/`, `internal/infrastructure/security/`, `cmd/api/`)

- **Router & middleware stack (`NewRouter`):** Chains `newRateLimitMiddleware` (`golang.org/x/time/rate` token bucket per IP, returning `429` + `Retry-After: 1` while excluding `/health`, `/docs`, `/openapi.json`) → `loggingMiddleware` → `corsMiddleware` → `ServeMux`.
- **Trusted-proxy client IP resolution (`ClientIPResolver`):** Shared by rate limiting and Turnstile verification; ignores `X-Forwarded-For` / `X-Real-IP` unless the direct peer matches `TRUSTED_PROXIES` IPs/CIDRs and walks forwarding headers right-to-left.
- **CAPTCHA & Handlers (`TurnstileVerifier`, `Handlers`):** Verifies Turnstile tokens against Cloudflare `siteverify` (5s timeout, mandatory whenever `APP_ENV != "development"`), caps request bodies at 16 KiB, generates 256×256 PNG QR codes via `skip2/go-qrcode`, and serves `/health`, `/openapi.json`, and Scalar UI (`/docs`).
- **Configuration & DI (`internal/config/`, `cmd/api/main.go`):** Validates startup env config via `config.Load()` (see [.env.example](./.env.example)), injects concrete adapters via constructor wiring, and handles graceful `SIGINT`/`SIGTERM` shutdown.

## Local Commands

```bash
# Run vet and full race-enabled unit test suite
go vet ./...
go test -race ./...

# Optional: include live PostgreSQL integration tests in persistence/
TEST_DATABASE_URL="postgres://microurl:microurl_dev_password@localhost:5432/microurl?sslmode=disable" go test -v -race ./internal/infrastructure/persistence/...

# Build binary (ignored by git)
go build -o api ./cmd/api

# Start development server (requires backend/.env; start Postgres + local Redis via `docker compose up -d`)
go run ./cmd/api
```
