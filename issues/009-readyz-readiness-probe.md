# 009: Add `/readyz` readiness probe that checks the database

> Type: AFK · Priority: P2 · Effort: S
> Glossary: Operator (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md` (Cloud SQL Postgres
> is part of the guaranteed profile).

## What to build

Artifact exposes `/healthz`, but it returns a static `ok` — it doesn't check whether dependencies are
actually reachable. The Helm `deployment.yaml` points **both** the liveness and readiness probes at
`/healthz`, so a pod reports "ready" before Cloud SQL is reachable. During rolling updates and on
cold starts this causes the classic Cloud-SQL-not-ready race: traffic is routed to a pod that then
500s on the first DB-backed request.

Split liveness from readiness:

- Keep `/healthz` as the **liveness** signal (process is up; stays cheap and dependency-free).
- Add **`/readyz`** that checks real readiness — at minimum a DB ping (e.g. `db.PingContext` with a
  short timeout), and optionally a storage reachability check. Return `200` only when dependencies
  are reachable; non-200 otherwise.
- Point the Helm **readinessProbe** at `/readyz` (liveness stays on `/healthz`).

## Acceptance criteria

- [ ] `GET /readyz` returns `200` when the DB is reachable and a non-200 when it is not
- [ ] `/healthz` remains a cheap, dependency-free liveness check
- [ ] Helm `deployment.yaml` readinessProbe targets `/readyz`; livenessProbe stays on `/healthz`
- [ ] A unit/e2e test covers `/readyz` returning non-200 when the DB is unavailable
- [ ] `docs/self-hosting.md` (or the relevant ops doc) documents both endpoints
- [ ] `go build ./...` and `go test ./...` green

## Blocked by

None — can start immediately.
