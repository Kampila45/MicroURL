# MicroURL

A production-ready URL shortener backend built with Go following **Hexagonal Architecture** (Ports & Adapters) principles.

## Prerequisites

- Go 1.21 or higher
- PostgreSQL database (Supabase Postgres recommended)
- Environment variables configured

## Setup

### 1. Create Database

First, create the database table in your PostgreSQL database:

```sql
CREATE TABLE links (
    id TEXT PRIMARY KEY,
    slug TEXT UNIQUE NOT NULL,
    original_url TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP
);

CREATE INDEX idx_links_slug ON links(slug);
```

### 2. Configure Environment Variables

Create a `.env` file in the project root:

```env
DATABASE_URL=postgres://user:password@localhost:5432/url_shortener?sslmode=disable
BASE_URL=http://localhost:8080
PORT=8080
RATE_LIMIT_RPS=10.0
RATE_LIMIT_BURST=20
CACHE_SERVERS=localhost:11211
CACHE_TTL=3600
```

Or set environment variables directly:

```bash
export DATABASE_URL="postgres://user:password@localhost:5432/url_shortener?sslmode=disable"
export BASE_URL="http://localhost:8080"
export PORT="8080"
```

### 3. Install Dependencies

```bash
go mod download
```

### 4. Run the Application

```bash
go run cmd/api/main.go
```

Or build and run:

```bash
go build -o api cmd/api/main.go
./api
```

The server will start on the port specified in `PORT` (default: 8080).

## API Endpoints

### 1. API Documentation

**GET** `/docs`

Interactive API documentation powered by Scalar. Provides a beautiful, interactive interface to explore and test all API endpoints.

**Access:** Open in browser: `http://localhost:8080/docs`

### 2. OpenAPI Specification

**GET** `/openapi.json`

Returns the OpenAPI 3.1.0 specification in JSON format. Used by the documentation UI and can be imported into API testing tools.

**Example:**
```bash
curl http://localhost:8080/openapi.json
```

### 3. Health Check

**GET** `/health`

Checks the health status of the API and database connectivity.

**Response (200 OK):**
```json
{
  "status": "ok",
  "database": "ok",
  "timestamp": "2025-01-01T12:00:00Z"
}
```

**Response (503 Service Unavailable)** - When database is unavailable:
```json
{
  "status": "unhealthy",
  "database": "unhealthy",
  "timestamp": "2025-01-01T12:00:00Z"
}
```

**Example:**
```bash
curl http://localhost:8080/health
```

### 4. Create Short Link

**POST** `/api/links`

Creates a new short link for the provided URL.

**Request Body:**
```json
{
  "url": "https://example.com/very/long/url",
  "expires_at": "2025-12-31T23:59:59Z"
}
```

**Response (201 Created):**
```json
{
  "id": "1735123456789-abc123xyz",
  "slug": "aBc123Xy",
  "original_url": "https://example.com/very/long/url",
  "created_at": "2025-01-01T12:00:00Z",
  "expires_at": "2025-12-31T23:59:59Z",
  "short_url": "http://localhost:8080/aBc123Xy"
}
```

**Example:**
```bash
curl -X POST http://localhost:8080/api/links \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com"}'
```

### 5. Resolve Short Link

**GET** `/{slug}`

Redirects to the original URL associated with the slug.

**Response:**
- **302 Found** - Redirects to original URL
- **404 Not Found** - Slug does not exist
- **410 Gone** - Link has expired

**Example:**
```bash
curl -L http://localhost:8080/aBc123Xy
```

Or open in browser:
```
http://localhost:8080/aBc123Xy
```

## Testing with cURL

### Create a short link:
```bash
curl -X POST http://localhost:8080/api/links \
  -H "Content-Type: application/json" \
  -d '{"url": "https://www.google.com"}'
```

### Create a short link with expiration:
```bash
curl -X POST http://localhost:8080/api/links \
  -H "Content-Type: application/json" \
  -d '{
    "url": "https://www.google.com",
    "expires_at": "2025-12-31T23:59:59Z"
  }'
```

### Check health:
```bash
curl http://localhost:8080/health
```

### Resolve a short link:
```bash
curl -I http://localhost:8080/{slug}
```

## Testing with Browser

1. **Create a link**: Open browser developer tools (F12) and run:
```javascript
fetch('http://localhost:8080/api/links', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ url: 'https://www.google.com' })
})
.then(r => r.json())
.then(console.log)
```

2. **Resolve a link**: Simply navigate to `http://localhost:8080/{slug}` in your browser

## Project Structure

```
.
├── cmd/
│   └── api/
│       └── main.go              # Application entry point
├── internal/
│   ├── domain/                  # Domain entities and errors
│   ├── application/             # Use cases and repository interfaces
│   ├── infrastructure/           # HTTP handlers and database implementations
│   │   ├── http/                # HTTP layer
│   │   └── persistence/         # Database layer
│   └── config/                   # Configuration
└── .env                          # Environment variables (create this)
```

## Architecture

This project follows **Hexagonal Architecture** (also known as Ports & Adapters Architecture):

### Hexagonal Architecture vs Clean Architecture

While both architectures share similar principles, this implementation specifically follows **Hexagonal Architecture**:

- **Hexagonal Architecture** focuses on **Ports** (interfaces) and **Adapters** (implementations)
  - The core application is isolated in the center
  - External concerns (HTTP, database) are adapters that plug into ports
  - The application can be driven by different adapters (HTTP, CLI, tests) without changing core logic

- **Clean Architecture** focuses on **dependency rules** and **layer boundaries**
  - Defines strict dependency direction (dependencies point inward)
  - More prescriptive about layer structure

This implementation uses **Hexagonal Architecture** with Clean Architecture principles:

- **Domain Layer** (`internal/domain/`): Pure business entities and domain errors, zero dependencies
- **Application Layer** (`internal/application/`): Use cases and **ports** (repository interfaces)
- **Infrastructure Layer** (`internal/infrastructure/`): **Adapters** (HTTP handlers, Postgres repository)

The key principle: **The domain must survive if Supabase (or any infrastructure) disappears tomorrow.**

## Rate Limiting

The API implements IP-based rate limiting using a token bucket algorithm:

- **Default**: 10 requests per second per IP address
- **Burst**: Allows up to 20 requests in a short burst
- **Health endpoint**: Excluded from rate limiting
- **Response**: Returns `429 Too Many Requests` with `Retry-After` header when limit is exceeded

Rate limiting is configurable via environment variables:
- `RATE_LIMIT_RPS`: Requests per second (default: `10.0`)
- `RATE_LIMIT_BURST`: Burst size (default: `20`)

The rate limiter uses in-memory storage and automatically cleans up unused IP entries.

## Caching

The API implements Memcached caching for slug lookups to improve redirect performance:

- **Cache-first strategy**: ResolveLink use case checks cache before database
- **Automatic population**: Cache is populated on both create and resolve operations
- **TTL-based expiration**: Links are cached with configurable TTL
- **Expiration handling**: Expired links are removed from cache automatically

### Cache Architecture

Following Hexagonal Architecture principles:

- **Cache port** (`application.Cache`): Interface defined in application layer
- **Memcached adapter**: Implementation in infrastructure layer
- **Use case orchestration**: Cache logic is explicit in use cases, not hidden in decorators

### Configuration

```env
CACHE_SERVERS=localhost:11211
CACHE_TTL=3600
```

For multiple Memcached servers (high availability):
```env
CACHE_SERVERS=memcache1:11211,memcache2:11211,memcache3:11211
```

### Cache Behavior

- **ResolveLink**: Checks cache first, falls back to database, then populates cache
- **CreateLink**: Populates cache immediately after creation
- **Expired links**: Automatically removed from cache when detected
- **Cache misses**: Treated as optimization failure, not business logic failure

## Environment Variables

| Variable | Required | Description | Default |
|----------|----------|-------------|---------|
| `DATABASE_URL` | Yes | PostgreSQL connection string | - |
| `BASE_URL` | Yes | Base URL for short links | - |
| `PORT` | No | Server port | `8080` |
| `RATE_LIMIT_RPS` | No | Requests per second per IP | `10.0` |
| `RATE_LIMIT_BURST` | No | Burst size (max requests in short burst) | `20` |
| `CACHE_SERVERS` | No | Comma-separated Memcached servers | `localhost:11211` |
| `CACHE_TTL` | No | Cache TTL in seconds | `3600` |

## License

MIT

