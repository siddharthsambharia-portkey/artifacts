# docs-04: Docs — auth guide: Okta (OIDC)

> Vertical slice from the docs-and-agent-files initiative. Independently
> verifiable: the page renders and the YAML/env it shows matches
> `internal/config/config.go` + `internal/auth/oidc.go`. When done, update the
> row in `plans/README.md`.

## Status

- **Priority**: P1
- **Effort**: S
- **Type**: AFK
- **Category**: docs
- **Depends on**: none

## What to build

`docs/auth-okta.md` — step-by-step setup for running Artifact behind **Okta OIDC**. This also
satisfies the README's existing `docs/auth-okta.md` link.

Cover:

- Creating an Okta OIDC web app, the redirect URI (`https://<domain>/auth/callback` — the
  real callback route registered in `internal/server/server.go` for callback-capable auth),
  and the sign-in route (`/login`).
- The `artifact.yaml` `auth` block with `mode: oidc` and `oidc.{issuer, client_id,
  client_secret_env, groups_claim}` — issuer is the Okta org/auth-server URL; secret is
  supplied via the env var named in `client_secret_env` (default `ARTIFACT_OIDC_SECRET`, see
  `.env.example`).
- The `groups` claim: how groups flow into `artifact.me.groups` and feed governed-mode
  visibility + admin gating (Okta groups claim configuration).
- Session cookie behavior (HttpOnly, Secure, SameSite=Lax, scoped to the apex `*.<domain>`)
  so subdomains share the session — the trust-bubble mechanism.
- A verification checklist (boot, hit `/login`, land back authenticated, `artifact.me`
  populated, an admin-group user can reach `admin.<domain>`).

Keep it copy-pasteable; use placeholders for org-specific values; screenshots optional
(placeholders ok). Cross-link `configuration.md` and `governance-and-admin.md`.

## Acceptance criteria

- [ ] `docs/auth-okta.md` exists and the YAML matches the `OIDC` struct fields
- [ ] Redirect URI uses the real `/auth/callback` route and `/login` sign-in route
- [ ] Explains the `groups` claim → `artifact.me.groups` → visibility/admin path
- [ ] Relative links resolve

## Blocked by

None — can start immediately.
