# Security Policy

## Supported Versions

| Version | Supported |
|---------|-----------|
| `main` (current) | ✅ Yes |

Only the latest release on `main` receives security updates.

---

## Security Model & Controls

MicroURL is a Go + PostgreSQL + Upstash Redis URL shortener with a TypeScript/React frontend. Its security controls cover the following areas:

- **Bot & Spam Prevention (Cloudflare Turnstile):**
  - `POST /api/links` verifies the client's `captcha_token` against Cloudflare's `siteverify` API (`https://challenges.cloudflare.com/turnstile/v0/siteverify`) with a 5-second HTTP timeout.
  - `TURNSTILE_SECRET_KEY` is strictly required whenever `APP_ENV` is anything other than `development`; startup fails fast if the secret key is missing in non-development environments.
  - Development bypass mode only activates when `APP_ENV=development` and `TURNSTILE_SECRET_KEY` is empty, and logs explicit warnings to `stderr`.
- **Rate Limiting & Trusted Proxy IP Resolution:**
  - An in-memory per-IP token-bucket rate limiter (`golang.org/x/time/rate`) protects link creation, QR generation, and slug resolution (default `10.0` RPS, burst `20`, configurable via `RATE_LIMIT_RPS` and `RATE_LIMIT_BURST`), returning `429 Too Many Requests` with `Retry-After: 1`.
  - A single shared `ClientIPResolver` derives client identity for both rate limiting and Turnstile verification. By default (`TRUSTED_PROXIES` empty), `X-Forwarded-For` and `X-Real-IP` headers are ignored and the peer IP from `RemoteAddr` is used; when `TRUSTED_PROXIES` specifies trusted proxy IPs/CIDRs, forwarding chains are walked right-to-left across trusted hops.
- **Input & Redirect Validation:**
  - Request bodies on `POST /api/links` are capped at 16 KiB (`http.MaxBytesReader`).
  - `domain.ValidateDestinationURL` (enforced in both `Handlers.CreateLink` and `CreateLinkUseCase`) restricts links to public `http://` and `https://` URLs up to 2048 characters, rejecting `javascript:`/`data:`/`file:` schemes, embedded `user:pass@` credentials, and `localhost`/`.local`/`.internal`/loopback/private/link-local destinations.
  - Optional `expires_at` timestamps must parse as RFC3339 and lie strictly in the future; expired links return `410 Gone` and are evicted from Upstash Redis upon detection.
- **Secret & Configuration Hygiene:**
  - `DATABASE_URL`, `UPSTASH_REDIS_REST_TOKEN`, and `TURNSTILE_SECRET_KEY` live exclusively in backend environment variables and are never exposed to the frontend or returned in API responses.
  - Only public client values (`VITE_API_BASE_URL` and `VITE_TURNSTILE_SITE_KEY`) are exposed to browser builds.
- **Supply-Chain & Dependency Security (npm & Go):**
  - `backend/go.mod` + `backend/go.sum` pin Go module checksums.
  - Project-level `.npmrc` files (`.npmrc` and `frontend/.npmrc`) enforce `min-release-age=30` (requiring npm 11.10.0+ so only package versions published at least 30 days ago are resolved) and `save-exact=true` (strict exact version pinning with no `^` or `~` ranges).
  - Both `frontend/package.json` and `frontend/package-lock.json` are committed to Git and checked in CI via `npm ci --ignore-scripts`, `npm audit signatures`, and `npm audit --audit-level=high`.

---

## Reporting a Vulnerability

If you discover a security vulnerability in MicroURL, please report it responsibly:

1. **Do not open a public GitHub issue** for security vulnerabilities.
2. Open a [GitHub Security Advisory](https://github.com/Kampila45/MicroURL/security/advisories/new) using GitHub's private reporting feature.
3. Include a clear description of the vulnerability, reproduction steps, affected component (`backend`, `frontend`, `captcha`, `rate-limiter`), and potential impact.

Reported vulnerabilities will be reviewed and acknowledged within **7 days**, with a fix or remediation timeline targeted within **30 days**.
