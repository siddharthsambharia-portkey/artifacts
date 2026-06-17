# docs-03: Docs — operator core (configuration + self-hosting + architecture)

> Vertical slice from the docs-and-agent-files initiative. Independently
> verifiable: every page renders, links resolve, and every documented config
> field/route exists in code. When done, update the row in `plans/README.md`.

## Status

- **Priority**: P1
- **Effort**: M
- **Type**: AFK
- **Category**: docs
- **Depends on**: none

## What to build

The docs a platform team needs to **deploy and run** Artifact.

- `docs/configuration.md` — every `artifact.yaml` field, grouped exactly as the struct in
  `internal/config/config.go`: `branding`, `domain`, `listen`, `tls.mode`,
  `auth{mode,oidc,header_trust}`, `storage{driver,bucket,endpoint,path}`,
  `database{driver,url_env,url}`, `ai{upstream_url,api_key_env,image_model,models_allowlist}`,
  `warehouse{driver,credentials_env,allowed_datasets,row_limit}`,
  `notify.slack{mode,secret_env,channel_allowlist}`, `governance{mode,quotas}`, `data_dir`.
  Document defaults from `config.DefaultDev()`. Be precise that **only 7 fields are
  env-overridable** today (`ARTIFACT_DOMAIN`, `ARTIFACT_LISTEN`, `ARTIFACT_AUTH_MODE`,
  `ARTIFACT_STORAGE_DRIVER`, `ARTIFACT_DATABASE_URL`, `ARTIFACT_DATA_DIR`,
  `ARTIFACT_GOVERNANCE_MODE`) plus `ARTIFACT_CONFIG`; secrets are referenced via `*_env` keys
  (see `.env.example`). Note quota field is `ai_daily_calls_per_user` (not tokens) and the
  validation rules from `config.Validate()` (auth modes, header-trust requires
  `proxy_secret_env`, governance modes).
- `docs/self-hosting.md` — single-VM, Docker Compose (`deploy/docker-compose.yml`),
  Kubernetes/Helm (`deploy/helm/artifact/`, reference `values.yaml`: external pg/s3, NATS for
  multi-replica), GCP starter (`deploy/terraform/gcp/`), AWS starter
  (`deploy/terraform/aws/`). Be honest that Terraform examples are starting points (no LB /
  identity proxy wired). State the scale posture (one 2 vCPU / 4 GB VM ≈ 5,000 employees) and
  the wildcard-DNS requirement (`*.<domain>` + `admin.<domain>`).
- `docs/architecture.md` — one-process design: chi router, auth middleware
  (dev/oidc/header-trust), static serving from object storage, `/api/v1/*` to Postgres/SQLite,
  realtime hub at `/ws` (NATS adapter optional), atomic manifest-pointer deploys, embedded
  migrations run on boot, embedded static assets. Map the `internal/` packages to
  responsibilities. List the real route table from `internal/server/server.go` +
  `internal/server/api.go`.

## Acceptance criteria

- [ ] Every field in `docs/configuration.md` exists in `internal/config/config.go`; defaults
      match `DefaultDev()`
- [ ] Env-override list is exactly the 7 (+`ARTIFACT_CONFIG`) implemented in
      `applyEnvOverrides`
- [ ] `docs/self-hosting.md` references only deploy artifacts that exist under `deploy/`
- [ ] `docs/architecture.md` route list matches the chi routes in code
- [ ] All relative links resolve

## Blocked by

None — can start immediately.
