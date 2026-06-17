# docs-07: Docs — auth guide: header-trust (IAP / Pomerium / oauth2-proxy / ZTNA)

> Vertical slice from the docs-and-agent-files initiative. Independently
> verifiable: the page renders and matches `internal/auth/header_trust.go` +
> `config.Validate()`. When done, update the row in `plans/README.md`.

## Status

- **Priority**: P1
- **Effort**: S
- **Type**: AFK
- **Category**: docs
- **Depends on**: none

## What to build

`docs/auth-header-trust.md` — running Artifact behind an existing identity proxy that has
already authenticated the user and forwards identity in request headers. This is the
enterprise path (Google IAP, AWS ALB+OIDC, Pomerium, oauth2-proxy, ZTNA).

Cover:

- The `auth` block: `mode: header-trust` with
  `header_trust.{email_header, name_header, proxy_secret_env}` (defaults
  `X-Auth-Request-Email`, `X-Auth-Request-User`). Show how to align these headers with the
  proxy in front (e.g. oauth2-proxy / Pomerium header names).
- The **hard-fail boot rule** (from `config.Validate()`): header-trust refuses to start
  unless `proxy_secret_env` is set in config *and* the named env var is present
  (`ARTIFACT_PROXY_SECRET`, see `.env.example`). Explain why — without proxy authentication,
  spoofed identity headers would bypass the trust bubble.
- **Current behavior / limitation** (from `internal/auth/header_trust.go`): groups are
  currently hardcoded to `["employees"]`, so there are no admins and no group-scoped
  visibility in header-trust mode yet. Document this plainly and point governed-mode users to
  OIDC mode for now.
- Network model: wildcard internal DNS `*.<domain>` + `admin.<domain>` → proxy → Artifact;
  `tls: off` when TLS terminates at the proxy/LB; never exposed to the public internet.
- Point to the existing recipe `deploy/recipes/pomerium.md` and give short notes for
  oauth2-proxy and GCP IAP / AWS ALB.
- Verification checklist (boot fails without the secret; a request with the configured email
  header lands authenticated; a request without it is rejected).

## Acceptance criteria

- [ ] `docs/auth-header-trust.md` exists; header names + `proxy_secret_env` match the
      `HeaderTrust` struct
- [ ] Documents the boot hard-fail rule exactly as `config.Validate()` enforces it
- [ ] States the current `groups: ["employees"]` limitation
- [ ] Links `deploy/recipes/pomerium.md` and resolves all relative links

## Blocked by

None — can start immediately.
