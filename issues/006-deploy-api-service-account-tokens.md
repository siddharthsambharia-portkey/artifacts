# 006: Programmatic auth for the deploy API — service-account / Bearer tokens

> Type: HITL (decision-first) · Priority: P1 · Effort: M
> Glossary: Builder, Operator, Native OIDC, header-trust, Identity proxy (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md`; follow-up recorded in
> `plans/008-http-deploy-api.md` ("CLI client mode `artifact deploy --remote` with a token").

## What to build

The HTTP deploy API (`POST /api/v1/deploy`, and the read endpoints like `/api/v1/sites`) currently
authenticates **only** via a browser session cookie set by the native-OIDC login flow. Programmatic
clients — `artifact deploy --remote`, the MCP server, CI — have no cookie, so in native-OIDC mode
they get a `302` redirect to `/login` (an HTML SSO page) instead of a JSON response. Behind
oauth2-proxy this is worse: the proxy itself bounces the unauthenticated request to the IdP login
page. The result is that the headline `artifact deploy` workflow can't run non-interactively in the
exact deployment profile we recommend.

We need a **non-interactive credential** — a token a machine client sends (e.g. `Authorization:
Bearer <token>`) that authenticates it as a known identity and **bypasses the interactive SSO
redirect**, while staying safe in governed mode and behind a proxy.

**This issue is decision-first (HITL).** The auth model is load-bearing, so the design questions
below must be answered and signed off before any implementation. Once resolved, this issue should be
split into an AFK implementation slice (or this one re-typed to AFK) carrying the chosen design.

## Open questions to resolve (sign-off required)

1. **Identity shape.** Are tokens tied to a real **service-account identity** (its own email/groups,
   shows up in audit as itself) or do they impersonate an existing Builder? Recommendation:
   first-class service accounts with their own identity.
2. **Storage & secrecy.** Where do tokens live — Postgres/SQLite (hashed, à la password hashing, so
   a DB leak isn't a credential leak) vs. an Operator-managed static config/env value? Recommendation:
   hashed in the DB, shown once at creation.
3. **Issuance & lifecycle.** Who mints/revokes them — admin console UI, a CLI subcommand
   (`artifact token create`), or config only? Do they expire / rotate? Is there a revocation list?
4. **Scope.** Single capability (deploy only) vs. scoped tokens (deploy / read / admin)? Per-site
   restriction? Recommendation: start with a `deploy` scope, leave room to grow.
5. **Transport & proxy interaction.** Bearer header is the obvious choice — but behind oauth2-proxy,
   does the proxy reject Bearer-only requests before they reach Artifact? Decide whether the documented
   path is (a) a `/api/*` route the proxy is configured to pass through to Artifact's own token auth,
   or (b) tokens only matter in native-OIDC mode and header-trust deployments rely on the proxy's own
   service-account mechanism. This must be written down so the recommended profile actually works.
6. **Middleware placement.** How does Bearer auth slot into the existing `auth.Authenticator`
   middleware chain (`internal/server/server.go`) so a valid token short-circuits the redirect in
   `OIDCAuthenticator.Middleware` without weakening the cookie path?
7. **CLI surface.** Confirm the `artifact deploy --remote --token ...` (and `ARTIFACT_TOKEN` env)
   ergonomics, since `plans/008` already names this as the follow-up.

## Acceptance criteria

- [ ] A written decision (short ADR or an appendix to ADR 0001) answering questions 1–7
- [ ] The decision explicitly states how programmatic deploy works **both** in native-OIDC mode and
      behind oauth2-proxy/header-trust (no hand-waving on the proxy interaction)
- [ ] Token secrecy approach chosen and justified (hashing / one-time reveal / revocation)
- [ ] Audit attribution for token-driven deploys is specified (who shows up as the actor)
- [ ] A follow-up implementation issue is created (or this issue re-typed AFK) carrying the design

## Blocked by

None to start the decision. Implementation (separate slice) is soft-related to 007 (Helm must be
able to surface any token secret/env the design introduces).
