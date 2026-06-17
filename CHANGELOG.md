# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- **GCP + Okta deployment profile (issues 005 / 007 / 008 / 009)** — the project's guaranteed end-to-end profile is now fully wired:
  - `deploy/terraform/gcp/main.tf` HCL syntax fixed (`region` variable multi-attribute bug); starter now provisions a Workload Identity service account + bucket IAM binding and exposes `storage_bucket`, `cloudsql_connection_name`, and `workload_identity_sa_email` as `output`s mapped (in comments) to their Helm values
  - Helm chart: `config.storage.driver: gcs` is now expressible; a `serviceAccount` block can carry the `iam.gke.io/gcp-service-account` Workload Identity annotation; `auth.mode: header-trust` is fully representable with all four `header_trust.*` fields; proxy and OIDC secrets are injected from Kubernetes Secrets via `headerTrustSecret` and `oidcSecret` blocks
  - `deploy/helm/artifact/values-gcp.yaml` example showing the complete GCP + Okta profile (GCS driver, Workload Identity, Cloud SQL DSN from a Secret, Okta OIDC secret from a Secret, with a commented header-trust alternative)
  - Optional cert-manager `Certificate` resource (`certificate.enabled`, `certificate.clusterIssuer`, `certificate.secretName`) for `*.<domain>` + apex, gated off by default; Ingress template updated to render TLS blocks from `ingress.tls`
  - `GET /readyz` endpoint: pings the database with a 2-second timeout and returns `200 ok` when reachable, `503 Service Unavailable` otherwise; Helm `deployment.yaml` `readinessProbe` now targets `/readyz` (liveness stays on `/healthz`)
  - `deploy/recipes/wildcard-tls-gcp.md` — full walkthrough: why DNS-01 (not HTTP-01) is required for wildcards, Cloud DNS `ClusterIssuer`, Helm cert request, cross-subdomain SSO for both native OIDC and oauth2-proxy

### Removed

- Fake `warehouse.driver: snowflake` alias — it only ever worked with a postgres-compatible DSN and otherwise errored. Advertised warehouse drivers are now exactly `none`, `postgres`, and `bigquery`. Real Snowflake support may return later as an explicit feature (issue 003)
- Unimplemented `notify.slack.mode: bot` — the handler only ever POSTed to an incoming webhook, so `bot` silently behaved like `webhook`. `notify.slack.mode` now accepts only `off` and `webhook`; `bot` is rejected at config validation. Real Slack bot-token support may return later as an explicit feature (issue 004)

### Changed

- `db.DB` escape hatch sealed — `*sql.DB` is now a private field; all SQL for `ai_usage`, `uploaded_files`, `audit_log` quota, and sessions is encapsulated behind named domain methods (`CountAIUsageSince`, `ListAIUsageSummary`, `InsertFile`, `ListFiles`, `GetFileByID`, `CountWarehouseQueriesSince`, `InsertSession`, `GetSession`, `DeleteSession`)
- `realtime.EventPublisher` interface now includes `PublishDocumentEvent`; type assert on `*Hub` removed from DB API handler
- `governance.Governor` injected into `admin.Handler` at construction time instead of being re-allocated per request
- Feature handlers (`ai`, `files`, `notify`, `warehouse`) now accept narrow config interfaces instead of `*config.Config`
- `notify.Handler` accepts a `SlackPoster` interface; first unit tests added covering mode-off, allowlist enforcement, valid post, and audit-row insertion

### Added

- Drop-to-deploy: drag a folder, file, or zip onto the home page to publish a site, no CLI required
- HTTP deploy API (`POST /api/v1/deploy`) accepting multipart `files` or a `zip` part, with overwrite confirmation and per-site size quotas
- Design system with shared tokens served at `/ui.css`; redesigned home, admin console, and error pages
- Group-scoped site visibility — sites can be limited to members of a named IdP group, with an admin endpoint to set visibility
- Reflect-origin CORS on static responses, enabling cross-site script/asset imports between sites on the instance
- Per-table usage indexes on `audit_log`, `ai_usage`, and `uploaded_files`
- Characterization test suite pinning security-critical behavior (governance, sessions, warehouse query guards, rate limiting)

### Changed

- AI daily quota now counts requests (`ai_daily_calls_per_user`) instead of tokens; the `ai_usage.tokens` column is dropped

### Fixed

- Expired sessions now return an error instead of a nil authenticated user
- Governed-mode visibility is now enforced on static serving, WebSockets, KV, and site listing; WebSocket connections validate request Origin
- Warehouse query guards reject `UNION`, multi-statement, and `LIMIT`-bypass attempts
- Daily quota checks are portable across SQLite and Postgres (no SQLite-only date functions); oversized JSON request bodies are rejected

## [0.1.0] - 2026-06-11

### Added

- DB realtime subscribe wired to write-path events (live-poll updates across tabs)
- Admin API: audit search, AI usage, stats, config/quotas
- files.list() endpoint
- NATS pubsub adapter for multi-replica WebSocket broadcast
- Warehouse drivers: postgres, BigQuery, Snowflake (postgres-compatible DSN)
- OIDC sessions persisted in database
- Deploy manifest LRU cache
- AI per-user daily token quotas and model allowlist
- Warehouse per-user daily query quotas
- Governed mode ownership enforcement (tested)
- Single Go binary with dev, serve, deploy, init, list, open, logs, mcp, version commands
- Browser SDK (`artifact.js`) with db, kv, files, ai, warehouse, ws, notify, me
- Static site hosting with atomic manifest-pointer deploys
- Pluggable auth: dev, OIDC, header-trust
- Pluggable storage: local, S3, GCS
- SQLite (dev) and Postgres (production) database
- WebSocket hub with rooms and presence
- AI proxy (OpenAI-compatible upstream)
- Warehouse query API (SELECT-only)
- Slack notifications
- Trust and governed governance modes
- Admin console and home directory
- Agent skill files (SKILL.md, AGENTS.md)
- MCP server for agent integration
- Docker, Helm, Terraform examples
- Five example sites: guestbook, live-poll, team-dashboard, multiplayer-cursors, lunch-vote
