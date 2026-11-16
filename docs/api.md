# MicroURL — REST API Reference

The Go backend serves interactive OpenAPI 3.1.0 documentation via **Scalar** at `/docs` and the machine-readable schema at `/openapi.json`.

---

## Health & Documentation

### `GET /health`
Checks connectivity to PostgreSQL and Upstash Redis. Excluded from IP rate limiting.

- **Status:** `200 OK` when `healthy` or `degraded` (cache unavailable); `503 Service Unavailable` when PostgreSQL is `unhealthy`.

```json
{
  "status": "healthy",
  "message": "All systems operational",
  "database": {
    "status": "healthy",
    "message": "Database connection is active and responding"
  },
  "cache": {
    "status": "healthy",
    "message": "Cache connection is active and responding"
  },
  "timestamp": "2026-11-15T20:00:00Z"
}
```

### `GET /openapi.json`
Returns the OpenAPI 3.1.0 specification in JSON format. Excluded from IP rate limiting.

### `GET /docs`
Serves the interactive Scalar API Reference UI backed by `/openapi.json`. Excluded from IP rate limiting.

---

## Links

### `POST /api/links`
Creates a new short link for a validated URL, optionally with an expiration timestamp and Cloudflare Turnstile verification.

#### Request Body

| Field | Type | Required | Description |
|---|---|---|---|
| `url` | `string` | Yes | Public `http://` or `https://` destination URL (max 2048 characters, no embedded credentials or local/private hosts) |
| `custom_slug` | `string` | No | Optional custom alias (3–64 chars, `[a-z0-9_-]`, normalized to lowercase, reserved paths disallowed) |
| `expires_at` | `string` | No | Optional future RFC3339 expiration timestamp (`2006-01-02T15:04:05Z07:00`) |
| `captcha_token` | `string` | Conditional | Cloudflare Turnstile token (required unless `APP_ENV=development` with empty `TURNSTILE_SECRET_KEY`) |

```json
{
  "url": "https://example.com/very/long/url",
  "custom_slug": "portfolio-2026",
  "expires_at": "2026-12-31T23:59:59Z",
  "captcha_token": "0.mF_..."
}
```

#### Response (`201 Created`)

```json
{
  "id": "1735123456789-abc123xyz",
  "slug": "portfolio-2026",
  "original_url": "https://example.com/very/long/url",
  "created_at": "2026-11-15T20:00:00Z",
  "expires_at": "2026-12-31T23:59:59Z",
  "short_url": "http://localhost:8080/portfolio-2026",
  "qr_code_url": "http://localhost:8080/api/qr/portfolio-2026"
}
```

#### Error Responses
- `400 Bad Request` — Invalid JSON body, missing/unsafe/non-public `url`, invalid URL length (> 2048 chars), invalid `custom_slug`, or invalid/past `expires_at`
- `403 Forbidden` — Invalid or missing Cloudflare Turnstile `captcha_token`
- `409 Conflict` — Custom alias already taken or slug collision
- `413 Request Entity Too Large` — JSON body exceeds 16 KiB limit
- `429 Too Many Requests` — Rate limit exceeded
- `500 Internal Server Error` — Unexpected server or database failure

---

### `GET /{slug}`
Resolves a short slug (checking Upstash Redis first, then PostgreSQL), increments the link's total and daily UTC click counters, and redirects to the destination URL.

- **Headers:** `X-Cache: HIT` when served from Upstash Redis, or `X-Cache: MISS` when read from PostgreSQL
- **Status:**
  - `302 Found` — Redirects (`Location` header set to `original_url`)
  - `400 Bad Request` — Missing slug
  - `404 Not Found` — Slug does not exist
  - `410 Gone` — Link has expired
  - `429 Too Many Requests` — Rate limit exceeded

---

### `GET /api/qr/{slug}`
Verifies that the slug exists and has not expired, then returns a 256×256 PNG QR code encoding `{BASE_URL}/{slug}`. Does not increment click analytics.

- **Headers:** `Content-Type: image/png`, `Cache-Control: public, max-age=3600`
- **Status:**
  - `200 OK` — Binary PNG image
  - `400 Bad Request` — Missing slug
  - `404 Not Found` — Slug does not exist
  - `410 Gone` — Link has expired
  - `429 Too Many Requests` — Rate limit exceeded

---

### `GET /api/links/{slug}/stats`
Returns privacy-conscious aggregate click analytics for a short link over a UTC daily window (`range=7d` default or `range=30d`, with missing days zero-filled).

- **Query parameters:** `range` (`7d` | `30d`, defaults to `7d`)
- **Response (`200 OK`):**
  ```json
  {
    "slug": "my-docs",
    "original_url": "https://example.com/docs",
    "status": "active",
    "created_at": "2026-11-01T10:00:00Z",
    "expires_at": "2026-12-01T10:00:00Z",
    "total_clicks": 42,
    "timezone": "UTC",
    "range_days": 7,
    "daily": [
      { "date": "2026-11-09", "clicks": 0 },
      { "date": "2026-11-15", "clicks": 12 }
    ]
  }
  ```
- **Status:**
  - `200 OK` — Analytics payload returned
  - `400 Bad Request` — Invalid slug or unsupported `range`
  - `404 Not Found` — Slug does not exist
  - `429 Too Many Requests` — Rate limit exceeded

---

## Rate Limiting & Client IP Trust

All non-diagnostic routes (`/api/links`, `/api/qr/{slug}`, `/{slug}`) are protected by a per-IP token-bucket rate limiter (`RATE_LIMIT_RPS` and `RATE_LIMIT_BURST`). Forwarding headers (`X-Forwarded-For`, `X-Real-IP`) are only honored when the direct TCP peer matches `TRUSTED_PROXIES` (parsed right-to-left to skip trusted proxy hops); otherwise, the peer `RemoteAddr` IP is used identically for both rate limiting and Turnstile `remoteip`. Exceeded requests receive:

- **Status:** `429 Too Many Requests`
- **Header:** `Retry-After: 1`
- **Body:**

```json
{
  "error": "rate limit exceeded"
}
```
