# docs-06: Docs — auth guide: Google Workspace (OIDC)

> Vertical slice from the docs-and-agent-files initiative. Independently
> verifiable: the page renders and the YAML/env matches the OIDC config struct.
> When done, update the row in `plans/README.md`.

## Status

- **Priority**: P2
- **Effort**: S
- **Type**: AFK
- **Category**: docs
- **Depends on**: docs-04 (soft — reuse its OIDC structure/voice)

## What to build

`docs/auth-google.md` — step-by-step setup for running Artifact behind **Google Workspace
OIDC**.

Cover:

- Creating an OAuth 2.0 Client ID (Web application) in Google Cloud, the authorized redirect
  URI (`https://<domain>/auth/callback`), and the issuer
  (`https://accounts.google.com`).
- The `auth` block (`mode: oidc`, `oidc.{issuer, client_id, client_secret_env, groups_claim}`);
  client secret via `ARTIFACT_OIDC_SECRET`.
- The honest caveat on groups: Google's standard OIDC ID token does **not** include group
  membership. Explain the options — restrict to the Workspace domain (`hd` claim) and treat
  everyone as an employee (trust mode), or document that group-scoped visibility / admin
  gating needs an additional mechanism (Cloud Identity groups via the Directory API or an
  identity proxy that injects groups — point to `docs/auth-header-trust.md`). Do not claim
  groups work out of the box.
- Same session/trust-bubble notes (link to the Okta guide).
- Verification checklist.

## Acceptance criteria

- [ ] `docs/auth-google.md` exists; YAML matches the `OIDC` struct
- [ ] Uses `https://accounts.google.com` issuer and the real `/auth/callback` route
- [ ] States the no-groups-in-standard-ID-token caveat and the realistic options
- [ ] Relative links resolve

## Blocked by

- docs-04 (soft) — share OIDC structure/voice.
