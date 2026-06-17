# Architecture

Artifact is a single Go binary — one process, one HTTP server, no sidecar daemons. This
page describes how that process is structured: its request flow, route table, storage model,
realtime design, deploy mechanics, and each internal package's responsibilities.

---

## One-process design

```
                        ┌──────────────────────────────────────────────────┐
  Browser / CLI         │                  artifact (binary)               │
 ─────────────►         │                                                  │
  HTTP/WS               │  chi router                                      │
                        │    ├── middleware chain (RequestID, RealIP,      │
                        │    │   Recoverer, logging, auth, rate-limit)     │
                        │    ├── /api/v1/*  ──► API handlers               │
                        │    ├── /ws        ──► realtime Hub               │
                        │    └── /*         ──► static handler             │
                        │                       (home / admin / site)      │
                        │                                                  │
                        │  DB driver      Storage driver                   │
                        │  ┌──────┐       ┌──────────────────────┐        │
                        │  │SQLite│  or   │ local │ S3  │  GCS   │        │
                        │  │Postgres      └──────────────────────┘        │
                        │  └──────┘                                        │
                        │                                                  │
                        │  (optional) NATS pub/sub for multi-replica WS    │
                        └──────────────────────────────────────────────────┘
```

Everything runs in one OS process. There is no separate API server, no separate static file
server, no message queue daemon, and no background worker. The only external dependencies
are the database (Postgres or SQLite) and object storage (S3/GCS/local). Both are swappable
via driver interfaces.

---

## Startup sequence

1. `config.Load()` — merge defaults → YAML file → env overrides → `Validate()`.
2. `storage.New()` — open the configured storage driver.
3. `db.Open()` — open Postgres or SQLite connection pool.
4. `db.Migrate()` — run embedded, forward-only SQL migrations (no external tool needed).
5. `auth.NewAuthenticator()` — initialize the auth mode; OIDC fetches the provider's JWKS.
6. `realtime.NewHub()` + optional `realtime.ConnectNATS()` — start the in-process WS hub.
7. `server.routes()` — wire the chi router.
8. `http.Server.ListenAndServe()` — accept connections.

---

## Middleware chain

Every request passes through these middleware layers in order:

| Middleware | Responsibility |
|------------|---------------|
| `middleware.RequestID` | Attaches a unique request ID to context and response headers |
| `middleware.RealIP` | Resolves the real client IP from `X-Forwarded-For` / `X-Real-IP` (service runs behind a trusted proxy) |
| `middleware.Recoverer` | Catches panics and returns 500 |
| `loggingMiddleware` | Structured `slog` log line per request (method, path, status, duration, host) |
| `auth.Middleware` | Validates session; populates `auth.User` in context; redirects to `/login` when unauthenticated |
| `auth.RequireUser` | Hard-rejects (401) requests that passed the outer middleware but have no user (used on `/api/v1/*`) |
| `ratelimit.Middleware` | Token-bucket rate limiter keyed by user email (or remote IP pre-auth) |

---

## Auth middleware modes

The auth middleware is swapped at startup based on `auth.mode`:

| Mode | Implementation |
|------|---------------|
| `dev` | Always returns `dev@localhost`; no real validation. For local development only. |
| `oidc` | OIDC authorization-code flow. `/login` redirects to the provider; `/auth/callback` exchanges the code, creates a session cookie, and redirects back. Sessions are persisted in the database. |
| `header-trust` | Reads user identity from the configured request headers (`email_header`, `name_header`). Validates the `X-Artifact-Proxy-Auth` header against `proxy_secret_env` on every request. No session cookie — the proxy is the session. |

---

## Route table

All routes are registered in `internal/server/server.go` (infrastructure routes) and
`internal/server/api.go` (the `/api/v1` subrouter).

| Method | Path | Handler | Auth required |
|--------|------|---------|---------------|
| GET | `/healthz` | Returns `ok` + `X-Artifact-Version` header | No |
| GET | `/login` | Redirect to OIDC provider (oidc mode) or no-op (dev/header-trust) | No |
| GET | `/auth/callback` | OIDC code exchange; sets session cookie | No |
| GET | `/artifact.js` | Serves embedded SDK JavaScript | Outer auth middleware |
| GET | `/ui.css` | Serves embedded design system CSS | Outer auth middleware |
| GET | `/api/v1/sites` | List all sites (filtered by visibility in governed mode) | Outer auth middleware |
| GET | `/api/v1/me` | Returns the signed-in user object | RequireUser |
| POST | `/api/v1/db/{collection}` | Create document | RequireUser |
| GET | `/api/v1/db/{collection}` | List documents | RequireUser |
| PUT | `/api/v1/db/{collection}/{id}` | Update document | RequireUser |
| DELETE | `/api/v1/db/{collection}/{id}` | Delete document | RequireUser |
| PUT | `/api/v1/kv/{key}` | Set key/value entry | RequireUser |
| GET | `/api/v1/kv/{key}` | Get key/value entry | RequireUser |
| GET | `/api/v1/files` | List uploaded files for the site | RequireUser |
| POST | `/api/v1/files` | Upload a file | RequireUser |
| GET | `/api/v1/files/{id}` | Serve a file by ID | RequireUser |
| POST | `/api/v1/ai/chat` | Streaming chat proxy to configured upstream | RequireUser |
| POST | `/api/v1/ai/image` | Image generation proxy | RequireUser |
| POST | `/api/v1/warehouse/query` | Read-only SQL query | RequireUser |
| POST | `/api/v1/deploy` | Deploy a site (multipart files or zip) | RequireUser |
| POST | `/api/v1/notify/slack` | Post a Slack message | RequireUser |
| GET | `/ws` | WebSocket upgrade; realtime hub | RequireUser |
| GET | `/api/v1/admin/audit` | Audit log search | RequireUser |
| GET | `/api/v1/admin/usage` | Usage statistics | RequireUser |
| GET | `/api/v1/admin/config` | Runtime config snapshot | RequireUser |
| GET | `/api/v1/admin/stats` | Aggregate stats | RequireUser |
| PUT | `/api/v1/admin/sites/{site}/visibility` | Set site visibility | RequireUser |
| GET | `/*` (apex host) | Serve `home.html` (site directory) | Outer auth middleware |
| GET | `/*` (`admin.<domain>`) | Serve `admin.html` (admin console) | RequireUser |
| GET | `/*` (site subdomain) | Serve site static files from object storage | Outer auth middleware |

> `/api/v1/warehouse/query` is only mounted when `warehouse.driver` is not `none`.
>
> `/auth/callback` is only registered when the authenticator implements `CallbackCapable`
> (i.e. OIDC mode).

### Rate limits

| Bucket | Rate | Burst | Keyed by |
|--------|------|-------|---------|
| General API (`/api/v1/*`) | 20 req/s | 50 | User email (or remote IP pre-auth) |
| AI (`/api/v1/ai/*`) | 5 req/s | 10 | User email |
| Warehouse | 2 req/s | 5 | User email |
| Deploy | 0.2 req/s (1 per 5s) | 3 | User email |

---

## Static file serving

Site files are stored in object storage under the prefix `sites/<site>/`. On each request
to a site subdomain, the static handler:

1. Resolves the site name from the `Host` header.
2. In governed mode, checks `CanReadSite` (visibility, group membership).
3. Reads the current manifest pointer to find the active deploy ID.
4. Fetches the file from storage and streams it to the browser.

A `DeployCache` (LRU, 512 entries) holds recently served manifests in memory to avoid
repeated storage reads for the manifest pointer on hot sites.

Uploaded files (via `artifact.files`) are served from a separate path in object storage.
They always include `Content-Disposition: attachment` and a restrictive CSP to prevent
uploaded content from executing as page scripts.

---

## Atomic deploys

`POST /api/v1/deploy` (and the CLI's `artifact deploy`) follow a three-step atomic
sequence:

1. **Upload files** — each file is written to storage under
   `sites/<site>/deploys/<deploy-id>/<path>`.
2. **Write manifest** — a JSON manifest listing all files and their sizes is written to
   `sites/<site>/deploys/<deploy-id>/.artifact-manifest.json`.
3. **Flip pointer** — a single object (`sites/<site>/.artifact-current`) is overwritten with
   the new deploy ID.

The pointer flip is the only step that makes a new deploy visible. A half-uploaded or crashed
deploy is never served. The previous pointer continues to resolve until the flip succeeds.

---

## Realtime hub

The realtime hub (`internal/realtime`) manages WebSocket connections and delivers events to
browser subscribers.

- **Single-replica**: the hub is in-process. When a document is created, updated, or deleted
  via `/api/v1/db/*`, the API handler calls `events.PublishDocumentEvent()`. The hub fans
  the event out to all WebSocket clients subscribed to that site and collection.
- **Multi-replica** (opt-in): when `nats.enabled` is true in the Helm chart, the hub
  publishes events to a NATS subject (`artifact.db.<domain>`). Each replica subscribes to
  the same subject, so events reach all pods. Without NATS, clients on different replicas
  do not see each other's events.

`artifact.ws` (rooms, presence, messages) and `artifact.db.subscribe()` both go through the
same WebSocket connection and the same hub.

---

## Embedded assets and migrations

The binary embeds all static assets and all SQL migrations using Go's `//go:embed` directive.

| Embedded asset | Location in binary | Description |
|----------------|--------------------|-------------|
| `artifact.js` | `internal/server/static/artifact.js` | Browser SDK |
| `ui.css` | `internal/server/static/ui.css` | Design system CSS |
| `home.html` | `internal/server/static/home.html` | Site directory page |
| `admin.html` | `internal/server/static/admin.html` | Admin console |
| SQL migrations | `internal/db/migrations/*.sql` | Forward-only numbered migrations |

Migrations run automatically on startup (`db.Migrate()`). There is no separate migration
tool and no manual step. Migrations are forward-only with no rollback scripts.

---

## Package map

| Package | Responsibility |
|---------|---------------|
| `internal/config` | `Config` struct, `DefaultDev()`, YAML loading, `applyEnvOverrides()`, `Validate()` |
| `internal/server` | chi router wiring, HTTP server lifecycle, embedded static asset serving |
| `internal/auth` | `Authenticator` interface; dev / OIDC / header-trust implementations; session cookie management; `RequireUser` middleware |
| `internal/db` | `*sql.DB` wrapper for Postgres and SQLite; documents, KV, sites, uploaded files, sessions, audit log, AI usage, warehouse usage counts; embedded migrations |
| `internal/sites` | `Deployer` (atomic manifest-pointer deploy logic); `StaticHandler` (site file serving from object storage); `DeployCache` |
| `internal/storage` | `Store` interface; `local`, `s3`, `gcs` driver implementations |
| `internal/realtime` | In-process WebSocket hub; NATS adapter for multi-replica; `EventPublisher` interface |
| `internal/ai` | AI chat and image generation proxy; streams SSE from upstream; enforces `ai_daily_calls_per_user` quota |
| `internal/warehouse` | Read-only SQL proxy; SELECT guard; dataset allowlist; row-limit enforcement |
| `internal/files` | File upload and serve HTTP handler; enforces `upload_max_mb`; sets `Content-Disposition: attachment` + restrictive CSP |
| `internal/notify` | Slack webhook / bot-token notification handler; channel allowlist; `SlackPoster` interface |
| `internal/governance` | `Governor`; trust vs governed mode checks (`CanReadSite`, `CanWriteDB`, `IsAdmin`); quota enforcement |
| `internal/admin` | Admin console HTTP handler: audit log search, usage stats, runtime config snapshot, site visibility management |
| `internal/ratelimit` | Token-bucket rate-limiter middleware; per-user or per-IP key |
| `internal/cli` | `cobra` CLI commands: `serve`, `dev`, `deploy`, `init`, `version`, etc. |
