# CLAUDE.md — working on Artifact itself

For agents and humans modifying the Artifact platform (this repo), not building
sites on top of it.

## What this is

Artifact is an open-source internal hosting platform: drop a folder of static
files and get a live internal URL with a database, files, AI, websockets,
warehouse queries, and Slack — no API keys in client code, one Go binary. For
the domain vocabulary (trust bubble, sites, governed mode, drop-to-deploy) read
[`docs/concepts.md`](docs/concepts.md); the SDK surface is in
[`skills/SKILL.md`](skills/SKILL.md).

## Commands

```bash
make dev      # go run ./cmd/artifact dev — local server, SQLite, dev auth
make build    # go build -o bin/artifact ./cmd/artifact
make test     # go test ./... -count=1
make lint     # golangci-lint run ./...
make sdk      # cd sdk && npm install && npm run build
make e2e      # go test ./e2e/... -count=1 -timeout 5m
```

Full gate before a PR: `go test ./... -count=1 -race`.

## Architecture in six lines

- Single Go binary (`cmd/artifact`); all logic in `internal/`.
- HTTP via a chi router (`internal/server/server.go`); static pages embedded with `//go:embed static/*`.
- Subdomain routing: `mysite.<domain>` → that site; `admin.<domain>` → console; apex → home.
- Auth is pluggable: `dev`, `oidc`, `header-trust` (`internal/auth`).
- Storage is pluggable: `local`, `s3`, `gcs` (`internal/storage`); deploys are atomic manifest-pointer swaps.
- Database is SQLite (dev) or Postgres (prod) via sequential, embedded, both-dialect migrations in `internal/db/migrations/`.

## Hard constraints

- **No custom backends, no cron, no per-site secrets — ever.** This is a
  closed-by-design decision (see [`docs/faq.md`](docs/faq.md) and
  `docs/concepts.md`). When something feels missing, combine two existing SDK
  primitives; do not add a backend escape hatch.
- **Trust-bubble assumption:** every request is an authenticated employee. Don't
  reintroduce per-app auth or API keys in client code.
- **Embedded pages use no external assets** and **no build step** — the static
  UI (`internal/server/static/`) is plain HTML/CSS/JS served as-is.

## Conventions

- Table-driven tests; exemplar: [`internal/config/config_test.go`](internal/config/config_test.go).
- Conventional commits (`feat:`, `fix:`, `docs:`…).
- HTTP handlers respond with `writeJSON` / `writeError` (`{"error": "..."}`), never raw `http.Error` for API routes.
- Any Artifact-owned page pulls design tokens from `internal/server/static/ui.css`.

## Docs touchpoints — keep these in sync in the same PR

| When you change… | Update… |
|------------------|---------|
| SDK surface (`sdk/src/artifact.ts`) | `skills/SKILL.md` + `docs/sdk-reference.md` + README features table |
| A new/changed HTTP endpoint | `docs/http-api.md` |
| Governance / visibility behavior | `docs/concepts.md` (and an ADR, once `docs/adr/` exists) |
| Any shipped feature | `CHANGELOG.md` `[Unreleased]` |
| A plan you executed | its row in `plans/README.md` |

If your PR changes behavior and touches none of these, say why in the PR description.

## Plans process

Implementation plans live in `plans/`; the index is `plans/README.md`. Statuses
there are verified by **probes against the code**, not by self-declaration — a
plan file saying `Status: DONE` is a claim to check, not a fact. Plan 011
(`plans/011-docs-and-plans-reconciliation.md`) is the re-runnable reconciliation
procedure: run it after any large batch of plans lands to bring this file, the
changelog, the README, and the index back in line with what actually shipped.
