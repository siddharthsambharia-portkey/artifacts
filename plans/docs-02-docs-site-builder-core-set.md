# docs-02: Docs — site-builder core set + index

> Vertical slice from the docs-and-agent-files initiative. Independently
> verifiable: every page renders as valid Markdown and all relative links
> resolve. When done, update the row in `plans/README.md`.

## Status

- **Priority**: P1
- **Effort**: M
- **Type**: AFK
- **Category**: docs
- **Depends on**: none
- **Progress**: `docs/README.md`, `docs/quickstart.md`, `docs/concepts.md`,
  `docs/sdk-reference.md` already drafted; `cli-reference.md` and `faq.md` remain.

## What to build

The human-facing docs a person needs to **build sites on Artifact**, as plain
GitHub-rendered Markdown (no generator), with a `docs/README.md` index. Everything must match
the actual code, not the build spec's aspirations.

Pages:

- `docs/README.md` — index / table of contents linking every doc.
- `docs/quickstart.md` — install, `artifact dev`, `init`, `deploy`, first SDK app.
- `docs/concepts.md` — sites, the trust bubble, server-derived identity/site, the zero-config
  API, cross-site reads, trust vs governed, hard non-goals.
- `docs/sdk-reference.md` — every `artifact.*` method, the HTTP endpoint behind it, runnable
  examples, rate limits, error shape.
- `docs/cli-reference.md` — every subcommand that actually exists:
  `serve, dev, deploy, init, login, list, open, logs, mcp, version`. Document **real**
  behavior, including: `artifact login` is currently a stub (prints guidance, no device
  flow), and CLI `deploy`/`mcp deploy_site` run as `dev@localhost` and talk to storage/DB
  directly (so for production, deploys go through the web UI / `POST /api/v1/deploy` behind
  SSO). Include the `--config` global flag and the `mcp` tool list
  (`deploy_site, list_sites, read_logs, query_db`) with their params.
- `docs/faq.md` — "why no custom backends" philosophy + the ADR-0002-required "not a fit"
  examples (email, cron, secret-bearing internal API calls) each with the one-line
  alternative. Satisfies the README's existing `docs/faq.md` link.

## Acceptance criteria

- [ ] All six files exist and are internally consistent
- [ ] `cli-reference.md` states the real `login` stub + CLI-deploy-as-dev behavior (no
      fictional device-flow auth)
- [ ] `sdk-reference.md` lists only methods present in `sdk/artifact.d.ts`
- [ ] `docs/README.md` links every page; all relative links resolve
- [ ] `docs/faq.md` includes the three "not a fit" examples

## Blocked by

None — can start immediately.
