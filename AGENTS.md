# Artifact Operator Playbook

**Artifact** is an open-source internal hosting platform: a single Go binary that gives every employee a live `<site>.<domain>` URL with a zero-config browser backend (database, realtime, files, AI, warehouse SQL, WebSockets, Slack notifications). It runs behind your company's SSO and is designed exclusively for company-internal use — all routes are identity-gated and sites must **never** be exposed to the public internet.

> **Not what you're looking for?** To modify the Go source, see [CONTRIBUTING.md](CONTRIBUTING.md). To build a site on a running Artifact instance, point your agent at [skills/](skills/).

---

## Fast path

### Option A — Docker Compose (dev / demo)

```bash
cd deploy
docker compose up
```

Starts Postgres + MinIO + Artifact on `:8443`. Auth defaults to `dev` mode (any email accepted). Good for smoke-testing the full stack locally. Config lives in `deploy/artifact.docker.yaml`. See [`deploy/docker-compose.yml`](deploy/docker-compose.yml).

### Option B — Single binary + `artifact.yaml`

```bash
./artifact serve --config artifact.yaml
```

Edit `artifact.yaml` (repo root) to choose your auth, storage, and database before first boot. The binary starts, runs schema migrations, and is ready immediately. For cloud/VM/Kubernetes recipes see [Self-hosting](docs/self-hosting.md).

---

## Decision points

### Auth mode

| Mode | When to use | Docs |
|---|---|---|
| `dev` | Local dev / smoke tests only. Any email accepted, no real identity. **Never use in production.** | — |
| `oidc` | Production with Okta, Microsoft Entra ID, or Google Workspace. Redirects to your IdP for sign-in. | [docs/auth-okta.md](docs/auth-okta.md) |
| `header-trust` | Behind an existing identity proxy (Pomerium, oauth2-proxy, GCP IAP). Reads identity from trusted HTTP headers. | [docs/auth-header-trust.md](docs/auth-header-trust.md) |

Set with `auth.mode` in `artifact.yaml` or the `ARTIFACT_AUTH_MODE` env override.

### Storage driver

| Driver | When to use |
|---|---|
| `local` | Single VM. Files stored under `data_dir`. No extra infra required. |
| `s3` | AWS S3 or any S3-compatible endpoint (MinIO, Wasabi). Preferred for multi-node or high-durability deployments. |
| `gcs` | Google Cloud Storage. |

Set with `storage.driver`. For `s3` / `gcs` also set `storage.bucket` and (for non-AWS endpoints) `storage.endpoint`.

### Database

| Driver | When to use |
|---|---|
| `sqlite` | Single VM, low traffic, or dev. DB file at `database.url` (default `.artifact-data/artifact.db`). |
| `postgres` | Production, multi-node, or when you need connection pooling and concurrent writes. |

Set with `database.driver`. For Postgres, set `database.url_env` to the name of an env var holding the DSN (see `.env.example`), or set `database.url` directly.

### Governance mode

| Mode | Behavior |
|---|---|
| `trust` | All authenticated employees can read and write all sites. Zero friction. Default. |
| `governed` | Site owners control per-site visibility. Quotas enforced. Admin console available at `admin.<domain>`. |

Set with `governance.mode` or `ARTIFACT_GOVERNANCE_MODE`. For quota configuration and the admin console, see [Governance & admin](docs/governance-and-admin.md).

Full field reference for all options → [docs/configuration.md](docs/configuration.md).  
Infra recipes (single VM, Docker Compose, Kubernetes/Helm, GCP, AWS) → [docs/self-hosting.md](docs/self-hosting.md).  
Internal request flow and package structure → [docs/architecture.md](docs/architecture.md).

---

## Configuration and secrets model

**Primary config file**: `artifact.yaml` in the working directory (or the path set by `ARTIFACT_CONFIG`). The repo ships an annotated `artifact.yaml` at the root with all available fields.

**Secrets**: never put secret values in `artifact.yaml`. Use `*_env` fields to name the environment variable that holds the secret:

```yaml
auth:
  oidc:
    client_secret_env: ARTIFACT_OIDC_SECRET   # env var name, not the value
warehouse:
  credentials_env: ARTIFACT_WAREHOUSE_CREDS
```

Copy `.env.example` → `.env` and fill in the placeholders for your deployment. Never commit `.env`.

### Env overrides (take precedence over `artifact.yaml`)

| Variable | Overrides | Valid values |
|---|---|---|
| `ARTIFACT_CONFIG` | config file path | any file path |
| `ARTIFACT_DOMAIN` | `domain` | e.g. `artifact.corp.com` |
| `ARTIFACT_LISTEN` | `listen` | e.g. `:8443` |
| `ARTIFACT_AUTH_MODE` | `auth.mode` | `dev` \| `oidc` \| `header-trust` |
| `ARTIFACT_STORAGE_DRIVER` | `storage.driver` | `local` \| `s3` \| `gcs` |
| `ARTIFACT_DATABASE_URL` | `database.url` | Postgres DSN or SQLite file path |
| `ARTIFACT_DATA_DIR` | `data_dir` | local data directory path |
| `ARTIFACT_GOVERNANCE_MODE` | `governance.mode` | `trust` \| `governed` |

---

## Hard rules

These constraints are enforced by the binary or are architectural — do not work around them:

1. **`header-trust` refuses to boot without a proxy secret.** `auth.header_trust.proxy_secret_env` must be set in the config, and the named env var must be non-empty. Configure your identity proxy's shared secret before starting in header-trust mode.

2. **Wildcard DNS is required.** Point `*.<domain>` and `admin.<domain>` to Artifact's listen address. Each site is served at `<name>.<domain>`; without wildcard DNS those hostnames never reach the binary.

3. **Never expose Artifact to the public internet.** All routes are SSO-gated, but Artifact is designed as a trust-bubble tool — it must run behind your corp network, ZTNA, or VPN.

4. **Warehouse credentials must be read-only.** Artifact forwards SQL queries to the warehouse as-is. The credentials in `warehouse.credentials_env` must be scoped to `SELECT` only — no DDL, DML, or admin permissions.

5. **AI API keys stay server-side.** `ai.api_key_env` is read at startup and used only in server-to-upstream requests. Never inject AI keys into site HTML or client-visible config.

6. **`admin.<domain>` is served by the same binary.** The admin console (audit log, usage, site visibility) is at `admin.<domain>`. No separate process; same binary, same port.

---

## Verification

After starting, confirm the stack is healthy end-to-end:

```bash
# 1. Health check — no auth required
curl https://artifact.corp.com/healthz
# → ok   (with X-Artifact-Version: 0.1.0 response header)

# 2. Sign-in flow
# Open https://artifact.corp.com in a browser.
# dev mode:         auto-signs you in.
# oidc:             redirects to your IdP and back.
# header-trust:     identity is injected by your identity proxy.

# 3. Deploy a sample site
artifact init demo && cd demo
# Option A — drag the demo/ folder onto https://artifact.corp.com
# Option B — POST via the API (the correct path for production deploys):
curl -s \
  -F "site=demo" \
  -F "files=@index.html" \
  -H "Cookie: <session-cookie>" \
  https://artifact.corp.com/api/v1/deploy
# → site live at https://demo.artifact.corp.com

# 4. Admin console (governed mode only)
# Open https://admin.artifact.corp.com → audit log, usage, site visibility
```

---

## Known current limitations

Do not promise or automate around behavior that does not yet exist:

| Limitation | Detail |
|---|---|
| **CLI `artifact deploy` bypasses the server** | `artifact deploy` opens storage and the database directly, deploying as the hardcoded identity `dev@localhost`, bypassing SSO entirely. Production deploys must go through the web UI (drag-and-drop on the home page) or `POST /api/v1/deploy` with an authenticated session. |
| **`artifact login` is a stub** | The CLI login / token-issuance flow is not yet implemented. Use the web UI for actions that require an authenticated session. |
| **`header-trust` groups are hardcoded** | In header-trust mode, `Groups` is unconditionally set to `["employees"]` — there is no configurable groups header. Group-scoped visibility and admin/governed-mode group features require OIDC today. |

---

## Deploy recipes

| Recipe | Where |
|---|---|
| Docker Compose (dev / demo) | [`deploy/docker-compose.yml`](deploy/docker-compose.yml) |
| Kubernetes + Helm | [`deploy/helm/artifact/`](deploy/helm/artifact/) |
| GCP starter (GCS + Cloud SQL) | `deploy/terraform/gcp/main.tf` |
| AWS starter (S3 + RDS) | `deploy/terraform/aws/main.tf` |
| Okta OIDC | [docs/auth-okta.md](docs/auth-okta.md) |
| Header-trust (Pomerium / oauth2-proxy / IAP) | [docs/auth-header-trust.md](docs/auth-header-trust.md) |

> Terraform examples are starting points — add load balancers, networking, and your identity proxy per your org's requirements.
