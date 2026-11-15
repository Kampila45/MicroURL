# MicroURL

Monorepo for **MicroURL**, a URL shortener with a Go backend and web frontend.

## Structure

- [`backend/`](./backend) — Go REST API (PostgreSQL, Memcached, Cloudflare Turnstile, QR code generation, OpenAPI/Scalar docs)
- [`frontend/`](./frontend) — Web frontend

## Quick Start

### Backend
See [`backend/README.md`](./backend/README.md) for environment variables and database setup.

```bash
cd backend
go run ./cmd/api
```

### Frontend
```bash
cd frontend
npm install
npm run dev
```
