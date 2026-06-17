# docs-08: Docs — feature guides (AI gateway, warehouse, governance & admin)

> Vertical slice from the docs-and-agent-files initiative. Independently
> verifiable: each page renders and matches the relevant handler + config.
> When done, update the row in `plans/README.md`.

## Status

- **Priority**: P2
- **Effort**: M
- **Type**: AFK
- **Category**: docs
- **Depends on**: none

## What to build

The three operator-facing feature guides for the server-held capabilities.

- `docs/ai-gateway.md` — wiring `artifact.ai` to any OpenAI-compatible upstream
  (`internal/ai/ai.go`). Cover `ai.{upstream_url, api_key_env, image_model, models_allowlist}`;
  examples for OpenAI direct, Portkey, LiteLLM, Bedrock-compatible gateways; that keys stay
  server-side and the proxy forwards only to the one configured upstream (SSRF-safe);
  `image_model` enables `artifact.ai.image`; `models_allowlist` restricts `model`;
  per-user daily quota `ai_daily_calls_per_user` (0 = unlimited); routes
  `POST /api/v1/ai/chat` + `/image` and their rate limit (~5 req/s burst 10). Mention the
  `x-artifact-user` / `x-artifact-site` headers attached upstream for cost attribution if
  present in code.
- `docs/warehouse.md` — read-only SQL (`internal/warehouse/`). Cover
  `warehouse.{driver(bigquery|snowflake|postgres|none), credentials_env, allowed_datasets,
  row_limit}`; the SELECT-only parser + dataset allowlist + row limit + read-only creds;
  per-driver credential format via `ARTIFACT_WAREHOUSE_CREDS`; that an empty
  `allowed_datasets` denies all; route `POST /api/v1/warehouse/query` + rate limit
  (~2 req/s burst 5); quota `warehouse_daily_queries_per_user`.
- `docs/governance-and-admin.md` — trust vs governed mode (`internal/governance/`), the
  `governance.mode` toggle and `ARTIFACT_GOVERNANCE_MODE` override; what governed mode adds
  (first-deployer ownership, group-scoped visibility, restricted deletion); quotas block
  (`site_max_mb, db_max_docs_per_site, upload_max_mb, ai_daily_calls_per_user,
  warehouse_daily_queries_per_user`); the audit log (always recorded, even in trust mode);
  the admin console at `admin.<domain>` (admin-group gated) and its real endpoints
  (`GET /api/v1/admin/{audit,usage,config,stats}`, `PUT /api/v1/admin/sites/{site}/visibility`).
  Note that admin gating relies on the `groups` claim, so it needs OIDC mode today (tie back
  to docs-07's header-trust limitation).

## Acceptance criteria

- [ ] Three files exist; config blocks match `internal/config/config.go`
- [ ] Admin endpoint list matches the routes in `internal/server/server.go`
- [ ] Warehouse + AI behaviors (SELECT-only, allowlist, server-side keys) match the handlers
- [ ] Relative links resolve

## Blocked by

None — can start immediately.
