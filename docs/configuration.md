# Configuration

Artifact is configured with a single YAML file — `artifact.yaml` — and a small set of
environment variables for secrets and deployment-time overrides.

## How configuration is loaded

1. Artifact starts with built-in dev defaults (safe for `artifact dev` on a laptop).
2. If `ARTIFACT_CONFIG` is set, or `--config` is passed to `artifact serve`, that file is
   loaded and merged on top of the defaults.
3. Seven env vars can override specific fields after the file is parsed (see
   [Env overrides](#env-overrides)).
4. `config.Validate()` runs last; the binary refuses to boot on invalid config.

```bash
artifact serve --config /etc/artifact/artifact.yaml
# or
ARTIFACT_CONFIG=/etc/artifact/artifact.yaml artifact serve
```

## Full example

```yaml
branding:
  name: Artifact
  logo: ""

domain: artifact.corp.example.com
listen: ":8443"

tls:
  mode: off  # off when running behind a corporate LB or ZTNA proxy

auth:
  mode: oidc  # dev | oidc | header-trust
  oidc:
    issuer: https://corp.okta.com
    client_id: ""
    client_secret_env: ARTIFACT_OIDC_SECRET
    groups_claim: groups
  header_trust:
    email_header: X-Auth-Request-Email
    name_header: X-Auth-Request-User
    proxy_secret_env: ARTIFACT_PROXY_SECRET

storage:
  driver: s3  # local | s3 | gcs
  bucket: artifact-sites
  endpoint: ""  # leave empty for AWS S3; set for MinIO or GCS
  path: .artifact-data

database:
  driver: postgres  # postgres | sqlite
  url_env: ARTIFACT_DATABASE_URL
  url: .artifact-data/artifact.db

ai:
  upstream_url: https://gateway.corp.com/v1
  api_key_env: ARTIFACT_AI_KEY
  image_model: ""
  models_allowlist: []

warehouse:
  driver: none  # bigquery | snowflake | postgres | none
  credentials_env: ARTIFACT_WAREHOUSE_CREDS
  allowed_datasets: []
  row_limit: 10000

notify:
  slack:
    mode: off  # webhook | bot | off
    secret_env: ARTIFACT_SLACK_SECRET
    channel_allowlist: []

governance:
  mode: trust  # trust | governed
  quotas:
    site_max_mb: 500
    db_max_docs_per_site: 100000
    upload_max_mb: 50
    ai_daily_calls_per_user: 0
    warehouse_daily_queries_per_user: 200

data_dir: .artifact-data
```

---

## Field reference

### `branding`

Visual identity shown on the home page and admin console.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `branding.name` | string | `"Artifact"` | Display name shown in the UI header |
| `branding.logo` | string | `""` | URL or path to a logo image; empty uses the default wordmark |

---

### `domain`

| Field | Type | Default | Env override |
|-------|------|---------|--------------|
| `domain` | string | `"localhost"` | `ARTIFACT_DOMAIN` |

The base domain for subdomain routing. A site named `my-poll` is served at
`https://my-poll.<domain>`. The apex (`<domain>`) serves the home directory. The admin
console is at `admin.<domain>`. Both `*.<domain>` and `admin.<domain>` must resolve to the
Artifact server — see [DNS requirements](self-hosting.md#dns-requirements).

---

### `listen`

| Field | Type | Default | Env override |
|-------|------|---------|--------------|
| `listen` | string | `":8443"` | `ARTIFACT_LISTEN` |

TCP address the HTTP server binds to. Examples: `":8443"`, `"0.0.0.0:80"`.

---

### `tls`

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `tls.mode` | string | `"off"` | TLS handling. `"off"` — serve plain HTTP (typical behind a corporate LB or ZTNA proxy) |

> **Current behavior:** only `"off"` is implemented today. Terminate TLS at the proxy or
> load balancer in front of Artifact.

---

### `auth`

| Field | Type | Default | Env override |
|-------|------|---------|--------------|
| `auth.mode` | string | `"dev"` | `ARTIFACT_AUTH_MODE` |

**Validation rule:** `auth.mode` must be one of `dev`, `oidc`, or `header-trust`. Any other
value causes Artifact to refuse to boot.

| Mode | Description |
|------|-------------|
| `dev` | No real auth. Every request is signed in as `dev@localhost`. Safe only on a laptop. |
| `oidc` | Artifact handles the OIDC login/callback flow directly. Requires `auth.oidc.*` fields. |
| `header-trust` | Trust identity headers forwarded by a proxy (Pomerium, oauth2-proxy, GCP IAP). Requires `auth.header_trust.*` fields. **Artifact refuses to boot in this mode unless `proxy_secret_env` is set and the referenced env var is present** — this is a hard safety check so you cannot accidentally run without proxy authentication. |

#### `auth.oidc`

Used when `auth.mode: oidc`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `auth.oidc.issuer` | string | `""` | OIDC issuer URL (e.g. `https://corp.okta.com`) |
| `auth.oidc.client_id` | string | `""` | OIDC client ID from your identity provider |
| `auth.oidc.client_secret_env` | string | `""` | Name of the env var that holds the OIDC client secret |
| `auth.oidc.groups_claim` | string | `""` | JWT claim name that contains the user's group list (e.g. `groups`) |

The client secret itself is never in `artifact.yaml`. Set the env var named by
`client_secret_env` (e.g. `ARTIFACT_OIDC_SECRET=<secret>`). See [Auth — Okta](auth-okta.md)
for a step-by-step setup.

#### `auth.header_trust`

Used when `auth.mode: header-trust`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `auth.header_trust.email_header` | string | `""` | Request header that carries the authenticated user's email |
| `auth.header_trust.name_header` | string | `""` | Request header that carries the user's display name |
| `auth.header_trust.proxy_secret_env` | string | `""` | **Required.** Name of the env var holding the shared secret Artifact uses to verify the proxy's `X-Artifact-Proxy-Auth` header |

`proxy_secret_env` must be set in config, and the env var it names must be present at
boot time; Artifact refuses to start otherwise. This prevents accidentally exposing Artifact
without proxy authentication. See [Auth — header-trust](auth-header-trust.md) for a full Pomerium
example.

---

### `storage`

Where site files, uploaded files, and manifests are stored.

| Field | Type | Default | Env override |
|-------|------|---------|--------------|
| `storage.driver` | string | `"local"` | `ARTIFACT_STORAGE_DRIVER` |
| `storage.bucket` | string | `""` | — |
| `storage.endpoint` | string | `""` | — |
| `storage.path` | string | `".artifact-data"` | — |

| Driver | Description |
|--------|-------------|
| `local` | Files stored on the local filesystem under `storage.path` |
| `s3` | AWS S3 or any S3-compatible store (MinIO, Cloudflare R2). Set `bucket` and `endpoint` (leave `endpoint` empty for real AWS S3) |
| `gcs` | Google Cloud Storage. Set `bucket`; `endpoint` is unused |

Pass S3 credentials (`ARTIFACT_S3_ACCESS_KEY` / `ARTIFACT_S3_SECRET_KEY`) as env vars
directly to the AWS SDK, not through `artifact.yaml`.

---

### `database`

| Field | Type | Default | Env override |
|-------|------|---------|--------------|
| `database.driver` | string | `"sqlite"` | — |
| `database.url_env` | string | `""` | — |
| `database.url` | string | `".artifact-data/artifact.db"` | `ARTIFACT_DATABASE_URL` |

| Driver | Description |
|--------|-------------|
| `sqlite` | Embedded SQLite. Fine for a single VM with moderate load. No external dependency. |
| `postgres` | PostgreSQL 14+. Required for multi-replica deployments. |

`url_env` is an indirection — it names an env var that holds the DSN. `ARTIFACT_DATABASE_URL`
is the direct override. Set either; `ARTIFACT_DATABASE_URL` takes precedence.

Artifact runs embedded, forward-only migrations on boot. No separate migration step is needed.

---

### `ai`

AI gateway proxy. Keys stay on the server; the browser SDK calls `/api/v1/ai/*`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `ai.upstream_url` | string | `""` | Base URL of your OpenAI-compatible gateway (e.g. `https://gateway.corp.com/v1`) |
| `ai.api_key_env` | string | `"ARTIFACT_AI_KEY"` | Name of the env var holding the API key for the upstream |
| `ai.image_model` | string | `""` | Model name used for image generation via `artifact.ai.image()`. Empty disables image generation. |
| `ai.models_allowlist` | []string | `[]` | If non-empty, only these model names are accepted in chat requests. Empty = allow any model the upstream accepts. |

When `ai.upstream_url` is empty, all `/api/v1/ai/*` calls return an error. See
[AI gateway](ai-gateway.md).

---

### `warehouse`

Read-only SQL proxy. Queries are SELECT-only; the server enforces an allowlist and row cap.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `warehouse.driver` | string | `"none"` | `bigquery` \| `snowflake` \| `postgres` \| `none` |
| `warehouse.credentials_env` | string | `""` | Name of the env var holding credentials (JSON service account for BigQuery; DSN for Snowflake/Postgres) |
| `warehouse.allowed_datasets` | []string | `[]` | Dataset/schema allowlist. Empty = allow any dataset |
| `warehouse.row_limit` | int | `10000` | Hard row cap per query |

When `warehouse.driver` is `none`, the `/api/v1/warehouse/query` endpoint is not mounted.
See [Warehouse](warehouse.md).

---

### `notify.slack`

Server-side Slack integration for `artifact.notify.slack()`.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `notify.slack.mode` | string | `"off"` | `webhook` (incoming webhook URL) \| `bot` (bot token) \| `off` |
| `notify.slack.secret_env` | string | `""` | Name of the env var holding the webhook URL or bot token |
| `notify.slack.channel_allowlist` | []string | `[]` | Channels sites are permitted to post to. Empty = all channels allowed |

---

### `governance`

| Field | Type | Default | Env override |
|-------|------|---------|--------------|
| `governance.mode` | string | `"trust"` | `ARTIFACT_GOVERNANCE_MODE` |

**Validation rule:** `governance.mode` must be exactly `trust` or `governed`.

| Mode | Description |
|------|-------------|
| `trust` | All employees can read/write all sites. No ownership, no visibility scopes. The audit log still records all destructive actions. |
| `governed` | The first deployer owns a site. Visibility can be scoped to groups. Deletion is restricted. Admin console exposes audit search, quotas, and usage. |

See [Governance & admin](governance-and-admin.md).

#### `governance.quotas`

Quotas apply in both modes but are enforced with hard errors only in governed mode.

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `governance.quotas.site_max_mb` | int | `500` | Maximum total size of a deployed site in MB |
| `governance.quotas.db_max_docs_per_site` | int | `100000` | Maximum documents across all collections for one site |
| `governance.quotas.upload_max_mb` | int | `50` | Maximum size of a single file upload via `artifact.files.upload()` |
| `governance.quotas.ai_daily_calls_per_user` | int | `0` | Maximum AI *calls* per user per day (not tokens). `0` = unlimited |
| `governance.quotas.warehouse_daily_queries_per_user` | int | `200` | Maximum warehouse queries per user per day |

---

### `data_dir`

| Field | Type | Default | Env override |
|-------|------|---------|--------------|
| `data_dir` | string | `".artifact-data"` | `ARTIFACT_DATA_DIR` |

Root directory for local storage and SQLite. Artifact creates it on boot if it does not
exist. Not used when `storage.driver` is `s3` or `gcs` and `database.driver` is `postgres`.

---

## Env overrides

Eight environment variables are recognised in total. `ARTIFACT_CONFIG` sets the config file
path (consumed in step 2 above); the remaining seven override specific fields after the file
is parsed and take precedence over any value in `artifact.yaml`.

| Env var | Overrides | Example |
|---------|-----------|---------|
| `ARTIFACT_CONFIG` | Path to the config file to load | `/etc/artifact/artifact.yaml` |
| `ARTIFACT_DOMAIN` | `domain` | `artifact.corp.example.com` |
| `ARTIFACT_LISTEN` | `listen` | `:443` |
| `ARTIFACT_AUTH_MODE` | `auth.mode` | `oidc` |
| `ARTIFACT_STORAGE_DRIVER` | `storage.driver` | `s3` |
| `ARTIFACT_DATABASE_URL` | `database.url` | `postgres://user:pass@host:5432/artifact` |
| `ARTIFACT_DATA_DIR` | `data_dir` | `/var/lib/artifact` |
| `ARTIFACT_GOVERNANCE_MODE` | `governance.mode` | `governed` |

All other fields can only be set in the YAML file. Secrets (OIDC client secret, proxy shared
secret, AI key, warehouse credentials, Slack token) are never in `artifact.yaml` — only the
**name** of the env var is stored in the file (the `*_env` fields). See `.env.example` in
the repo root for a commented template of all secret env vars.
