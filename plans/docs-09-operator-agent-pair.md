# docs-09: Operator/platform agent pair (root `AGENTS.md` + `CLAUDE.md`)

> Vertical slice from the docs-and-agent-files initiative. Independently
> verifiable: both files exist at the repo root, links resolve, and they only
> describe modes/recipes that exist in code. When done, update the row in
> `plans/README.md`.

## Status

- **Priority**: P1
- **Effort**: M
- **Type**: AFK
- **Category**: docs
- **Depends on**: docs-03 (soft — to link configuration/self-hosting/architecture)

## What to build

The **operator/platform** agent skill as a pair of full standalone files at the **repo
root**, so a platform team can drop the repo into a coding agent and have it deploy & run
Artifact end-to-end. Agent harnesses auto-load root `AGENTS.md` / `CLAUDE.md`.

Decisions already made:

- Two **full standalone copies** — `/AGENTS.md` and `/CLAUDE.md`, byte-identical except the
  title line. (This is the operator audience; it is distinct from the site-builder pair in
  `skills/` from docs-01.)
- Audience = an agent **deploying/operating** Artifact, not one building sites and not one
  modifying the Go codebase. Include a one-line pointer to `CONTRIBUTING.md` for code changes
  and to `skills/` for site building, so it's not a dead end.

Content (a deploy playbook, grounded in real code/recipes):

- What Artifact is in 3 lines + the trust-bubble assumption (internal only).
- Prerequisites and the fast path: `docker compose up` (`deploy/docker-compose.yml`) or the
  single binary + `artifact.yaml`.
- Decision points with the real options: auth mode (`dev` | `oidc` | `header-trust`),
  storage driver (`local` | `s3` | `gcs`), database (`sqlite` | `postgres`), governance
  (`trust` | `governed`) — each linking the relevant `docs/` page.
- The config + secrets model: `artifact.yaml` + `*_env` vars (`.env.example`), and the exact
  7 env overrides that exist.
- Hard rules an operating agent must respect: header-trust refuses to boot without
  `proxy_secret_env`; warehouse is SELECT-only / read-only creds; AI keys are server-side;
  never expose sites to the public internet; wildcard DNS `*.<domain>` + `admin.<domain>`.
- Verification steps: `/healthz`, sign-in flow, deploy a sample, check `admin.<domain>`.
- Known current limitations (CLI deploy runs as `dev@localhost`; `artifact login` is a stub;
  header-trust groups hardcoded) so the agent doesn't promise behavior that isn't there.
- Links into `docs/` for every deep topic (do not duplicate the docs; point to them).

## Acceptance criteria

- [ ] `/AGENTS.md` and `/CLAUDE.md` exist as full standalone copies
- [ ] Only auth/storage/db/governance modes that exist in `internal/config` are described
- [ ] Includes the known-limitations section and pointers to `docs/`, `CONTRIBUTING.md`,
      `skills/`
- [ ] All relative links resolve

## Blocked by

- docs-03 (soft) — links to `configuration.md` / `self-hosting.md` / `architecture.md`.
