# 001: Configurable groups header for header-trust mode

> Type: AFK · Priority: P2 · Effort: S
> Glossary: Builder, Operator, header-trust, trust mode, governed mode (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md` (header-trust is
> supported but out-of-profile; its gaps are features, not blockers).

## What to build

In **header-trust** mode, derive the authenticated user's groups from a header the identity
proxy forwards, instead of hardcoding every user into `["employees"]`. This brings header-trust
to parity with native OIDC: the admin console and governed-mode group-scoped visibility start
working behind a proxy (oauth2-proxy, Pomerium, IAP, etc.).

Behavior:

- New config field `groups_header` under `auth.header_trust` (suggested default
  `X-Auth-Request-Groups`, which is what oauth2-proxy emits with `set_xauthrequest = true`).
- The authenticator reads that header, splits it into a list (comma-separated, trimmed), and
  stamps it onto the user's groups.
- If the header is absent or empty, fall back to `["employees"]` so no one is locked out in
  trust mode (mirrors the OIDC fallback).

## Docs to update (do them in this issue/branch)

- `docs/auth-header-trust.md`: replace the "Current limitation — groups are hardcoded to
  `["employees"]`" section with the new `groups_header` config; show the oauth2-proxy setup
  (`set_xauthrequest = true` + `groups` scope) and how Pomerium/IAP forward groups; update the
  verification checklist (admin console should now be reachable for the `admins` group, not a
  permanent 403).
- `docs/governance-and-admin.md`: drop any "header-trust can't reach admin" caveat.
- Config reference / `artifact.yaml` field docs: add the `groups_header` row.
- `plans/README.md` backlog: strike the "header-trust groups hardcoded" bullet.

## Acceptance criteria

- [ ] `auth.header_trust.groups_header` is configurable; default documented
- [ ] A request carrying the groups header lands with those groups on `artifact.me.groups`
- [ ] Missing/empty header falls back to `["employees"]` (no lockout in trust mode)
- [ ] A user in the `admins` group reaches `admin.<domain>` in header-trust mode
- [ ] `docs/auth-header-trust.md` no longer describes the hardcoding as a limitation
- [ ] `go build ./...` and `go test ./...` green

## Blocked by

None — can start immediately.
