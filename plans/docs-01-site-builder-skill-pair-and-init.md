# docs-01: Site-builder agent skill pair (`AGENTS.md` + `CLAUDE.md`) + `init` wiring

> Vertical slice from the docs-and-agent-files initiative. Independently
> verifiable: `artifact init x` produces a site folder containing `AGENTS.md`
> and `CLAUDE.md` (and no `SKILL.md`), and `go build ./...` is green. When done,
> update the row in `plans/README.md`.

## Status

- **Priority**: P1
- **Effort**: S
- **Type**: AFK
- **Category**: docs + cli
- **Depends on**: none

## What to build

Deliver the **site-builder** agent skill as a pair of full, standalone files that
`artifact init` drops into every new site, so an agent in Claude Code (or any client) can
**publish, retrieve, and change** Artifact sites with no other context.

Decisions already made (do not re-litigate):

- Two **full standalone copies** — `skills/AGENTS.md` and `skills/CLAUDE.md` are each a
  complete copy of the same content (different harnesses read different filenames). Keep them
  byte-identical except the title line.
- **Retire `skills/SKILL.md`** — delete it; `AGENTS.md`/`CLAUDE.md` replace it going forward.
- The content is the site-builder SDK skill, corrected to the **real** SDK surface (see
  `sdk/artifact.d.ts` and `internal/server/api.go`): `ready`, `me`, `db.collection`
  (`create/update/delete/list/subscribe`, `list` opts `where/order/limit/site`), `kv`,
  `files.upload/list`, `ai.chat/image`, `warehouse.query`, `ws.room` (+`presence`),
  `notify.slack`.
- Lead the content with the publish/retrieve/change workflow the user emphasized:
  `artifact deploy` (publish + overwrite-to-change), `artifact list` / `artifact open` /
  `artifact logs` (retrieve), drop-to-deploy and `POST /api/v1/deploy` as the no-CLI path.
- Keep the existing constraints block (no custom backends, no API keys, no per-site secrets,
  site is server-derived, warehouse SELECT-only, trust-mode cross-site reads).

## Files

- `skills/AGENTS.md` (rewrite — full site-builder skill)
- `skills/CLAUDE.md` (create — full copy of `skills/AGENTS.md`)
- `skills/SKILL.md` (delete)
- `internal/cli/init.go` (edit — change the dropped-file list from `["SKILL.md","AGENTS.md"]`
  to `["AGENTS.md","CLAUDE.md"]`; keep the fallback-content write for each)
- `demo-site/AGENTS.md`, `demo-site/CLAUDE.md`, `my-test/AGENTS.md`, `my-test/CLAUDE.md`
  (reconcile — these currently cross-reference `SKILL.md`/`CLAUDE.md`; regenerate so each
  points only at files that exist)

## Acceptance criteria

- [ ] `skills/AGENTS.md` and `skills/CLAUDE.md` exist, are full standalone copies, and match
      the real SDK (every method documented appears in `sdk/artifact.d.ts`)
- [ ] `skills/SKILL.md` no longer exists; nothing in the repo references it (grep)
- [ ] `internal/cli/init.go` drops `AGENTS.md` + `CLAUDE.md`; `go build ./...` exits 0
- [ ] `artifact init tmp-check` creates `tmp-check/AGENTS.md` + `tmp-check/CLAUDE.md`, no
      `SKILL.md` (then delete `tmp-check/`)
- [ ] `demo-site/` and `my-test/` agent files reference only existing files

## Blocked by

None — can start immediately.
