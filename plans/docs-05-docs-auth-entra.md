# docs-05: Docs — auth guide: Microsoft Entra ID (OIDC)

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

`docs/auth-entra.md` — step-by-step setup for running Artifact behind **Microsoft Entra ID
(Azure AD) OIDC**.

Cover:

- Registering an app in Entra ID, the redirect URI (`https://<domain>/auth/callback`), and
  the v2.0 issuer URL (`https://login.microsoftonline.com/<tenant>/v2.0`).
- The `auth` block (`mode: oidc`, `oidc.{issuer, client_id, client_secret_env, groups_claim}`);
  client secret via `ARTIFACT_OIDC_SECRET`.
- Entra group claims: emitting `groups` in the token (optional groups claim / group object
  IDs) and mapping them to admin/visibility; note the practical gotcha that Entra emits group
  **object IDs**, so admin/visibility groups must be configured to those IDs.
- Same session/trust-bubble notes as the Okta guide (link to it rather than repeating in
  full).
- Verification checklist.

## Acceptance criteria

- [ ] `docs/auth-entra.md` exists; YAML matches the `OIDC` struct
- [ ] Uses the v2.0 issuer and the real `/auth/callback` route
- [ ] Notes the Entra group-object-ID behavior for `groups_claim`
- [ ] Relative links resolve

## Blocked by

- docs-04 (soft) — share OIDC structure/voice.
