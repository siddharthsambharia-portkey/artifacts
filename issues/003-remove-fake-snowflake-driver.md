# 003: Remove the fake Snowflake warehouse driver

> Type: AFK · Priority: P2 · Effort: S
> Glossary: Operator, deployment profile (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md` (delete the lies).

## What to build

`warehouse.driver: snowflake` is not a real Snowflake driver — `internal/warehouse/snowflake.go`
only works if you hand it a `postgres://` DSN, otherwise it errors. It's a misleading alias.
Remove it so the advertised warehouse drivers are exactly the real ones: `none`, `postgres`,
`bigquery`. Real Snowflake support can return later as an explicit feature.

Scope:

- Delete `internal/warehouse/snowflake.go` and the `snowflake` case in
  `internal/warehouse/driver.go` (`NewQuerier`). An unknown driver should already error clearly.
- Remove `snowflake` from every doc and example: `docs/warehouse.md`, `docs/configuration.md`,
  `docs/README.md`, `docs/concepts.md`, `artifact.yaml`, and any agent skill / build-spec
  reference. Update `CHANGELOG.md`.

## Acceptance criteria

- [ ] `internal/warehouse/snowflake.go` is gone; `NewQuerier` only handles `none`/`postgres`/`bigquery`
- [ ] No remaining reference to a `snowflake` warehouse driver in code, config, or docs (grep)
- [ ] `go build ./...` and `go test ./...` green
- [ ] ADR 0001's "trims decided but not yet executed" note can drop Snowflake

## Blocked by

None — can start immediately.
