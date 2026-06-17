# Guaranteed deployment profile: GCP + Okta, trim everything else

Artifact's config advertised many drivers (auth: dev/oidc/header-trust; warehouse:
postgres/bigquery/snowflake; slack: webhook/bot), but several were stubs or aliases
that made the claims diverge from reality. We decided to guarantee exactly **one**
profile end-to-end — **GCP + Okta**: `auth.mode: oidc` (Okta), `storage.driver: gcs`,
`database.driver: postgres` (Cloud SQL), `warehouse.driver: bigquery`,
`notify.slack.mode: webhook`, governance `trust` or `governed`. Every driver in this
profile is backed by real code today.

Everything outside the profile is made honest one of two ways:

- **Fakes are deleted, not aspired to.** The fake Snowflake driver and the unimplemented
  Slack `bot` mode are removed rather than left as misleading config options. They return
  later only as explicit, real features — never as stubs.
- **Real alternatives are kept as supported-but-not-guaranteed.** header-trust auth and the
  S3 / local / sqlite drivers work and stay; they just aren't the one tested profile. So
  "trim everything else" means *delete the lies*, not *delete the alternatives*.

## Considered Options

- **Support everything**: build real Snowflake, real Slack bot, etc. Rejected — large
  effort with no current user, and it was the source of the claim≠reality problem.
- **Demo/dev only**: guarantee just `artifact dev`. Rejected — the project's purpose is
  a real internal platform, and GCP+Okta is the concrete target.
- **GCP + Okta, trim the rest** (chosen): smallest honest surface that still delivers the
  intended platform.

## Consequences

- **Auth stance: support both, native OIDC is the default.** native OIDC (`auth.mode: oidc`)
  is the recommended/guaranteed mode and the only one that delivers governed mode + admins.
  header-trust (`auth.mode: header-trust`) is a fully supported alternative for proxy-fronted
  and GCP-without-Okta deployments (e.g. oauth2-proxy + Google). The selection guidance lives
  in `docs/auth-overview.md`.
- **header-trust groups gap is resolved** (issue 001): `auth.header_trust.groups_header`
  (default `X-Auth-Request-Groups`) now reads groups from the proxy with an `["employees"]`
  fallback, so admins and governed-mode group visibility work behind a proxy. header-trust is
  therefore at parity with native OIDC except for not being the single tested profile.
- **Trims executed.** `warehouse.driver: snowflake` (issue 003) and `notify.slack.mode: bot`
  (issue 004) have been removed from code, config, validation, docs, and the agent skill files.
  The warehouse driver now accepts only `none`/`postgres`/`bigquery` (unknown drivers error in
  `NewQuerier`), and `notify.slack.mode` accepts only `off`/`webhook` (`bot` is rejected at
  config validation). Both may return later as explicit, real features — never as stubs.
