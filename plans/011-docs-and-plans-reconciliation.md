# Plan 011: Reconcile the plans index and bring every doc in line with the shipped code

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md`.
>
> **Drift check (run first)**: none — drift is the *subject* of this plan.
> Step 1 establishes ground truth from the code at execution time; never
> trust a status row, a plan file's self-declared status, or this plan's
> own assumptions over what the probes show.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none (run after any batch of plans lands; re-runnable)
- **Category**: docs
- **Planned at**: commit `5ca375f`, 2026-06-13

## Why this matters

The repo's code has moved fast (plans 001–010 executed in various branches, history rewritten once to a PR flow) and the documentation now lies about the product: README omits shipped SDK surface, CHANGELOG ends at 0.1.0, CONTEXT.md and two ADRs still describe shipped features as "pre-launch gaps," docs/faq.md is missing content ADR 0002 *requires*, and the plans index statuses have drifted from reality more than once. For a project whose primary contributor is a coding agent (CONTEXT.md), stale docs aren't cosmetic — they are wrong context that gets injected into every future agent session. This plan (a) derives ground truth from the code, (b) syncs the plans index, (c) syncs every user-facing doc, and (d) leaves behind a checklist that keeps docs in lockstep with future plans.

## Current state

Repository documentation surfaces (all at repo root unless noted):

- `plans/README.md` — the index. Statuses have flip-flopped during a history rewrite; rows for plans 008–010 may be missing even though their plan files and (apparently) implementations exist. Has "Backlog" and "Findings considered and rejected" sections that contain items the landed work may have resolved.
- `plans/001…010-*.md` — plan files. Some were rewritten by executors as completed-work descriptions with self-declared `Status: DONE` headers (e.g. 009, 010); treat those self-declarations as claims to verify, not facts.
- `README.md` — product readme. Known stale points: features table has no `artifact.kv` row (the SDK and CONTEXT.md both have kv); no mention of browser Drop-to-Deploy (if landed); "Deploy at your company" table oversells `deploy/terraform/{gcp,aws}` (both are ~30-line starters whose own comments say "Add ALB + OIDC…") and points to `deploy/recipes/` (plural) which contains only `pomerium.md`.
- `CHANGELOG.md` — Keep-a-Changelog format, single `[0.1.0] - 2026-06-11` entry. Nothing since.
- `CONTEXT.md` — domain glossary. Says group-scoped visibility is "a pre-launch gap" (§Governed mode visibility levels) and cross-site CORS "Requires CORS headers on static responses (pre-launch gap — see ADR 0004)" (§Ecosystem). Both statements must match reality after step 1.
- `docs/adr/0003-…md`, `docs/adr/0004-…md` — both `Status: Accepted` (0004 says "Accepted — pre-launch gap"). When implemented, status lines should record that.
- `docs/faq.md` — missing the concrete "not a fit" examples that ADR 0002's Consequences section explicitly requires ("docs/faq.md must include concrete 'not a fit' examples (email, cron, secret-bearing API calls)").
- `docs/quickstart.md`, `docs/concepts.md`, `docs/sdk-reference.md`, `docs/auth-okta.md` — verify claims against code; `sdk-reference.md` overlaps `skills/SKILL.md` with no statement of which is canonical.
- `skills/SKILL.md` + the template dropped by `internal/cli/init.go` — Builder/agent-facing SDK docs; verify against `sdk/src/artifact.ts` exports.
- No repo-level `CLAUDE.md` exists (a known audit finding) — created here because docs-maintenance is exactly what it institutionalizes.

Conventions: docs are plain Markdown, sentence-case headings, tables for enumerable facts; ADRs are numbered immutable decisions (append status notes, never rewrite the Decision section); CHANGELOG follows keepachangelog.com 1.1.0 with semver.

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Build     | `go build ./...`         | exit 0              |
| Tests     | `go test ./... -count=1` | all pass            |
| Link check| see step 7               | no missing targets  |

## Scope

**In scope** (docs and the plans index ONLY):
- `plans/README.md`
- `README.md`, `CHANGELOG.md`, `CONTEXT.md`
- `docs/` (faq.md, quickstart.md, concepts.md, sdk-reference.md, README.md (create), adr/0003, adr/0004 — status lines only)
- `skills/SKILL.md`
- `CLAUDE.md` (create)
- `.env.example` (create)

**Out of scope** (do NOT touch):
- Any `.go`, `.ts`, `.css`, `.html`, `.sql`, or config file — this plan changes zero behavior. If a doc disagrees with code, the DOC moves, never the code.
- The body of plan files 001–010 (their Steps/Scope/etc.) — only their rows in the index. Exception: you may append a one-line `> Reconciled <date>: <status>` note at the very top of a plan file whose self-declared status contradicted the probe.
- Launch copy (`launch/`), learning materials (`lessons/`, `learning-records/`, `reference/`, `NOTES.md`, `MISSION.md`, `RESOURCES.md`).
- `artifact-master-build-spec.md` / one-pager / use-cases (product planning docs, not living docs).

## Git workflow

- Branch: `advisor/011-docs-reconciliation`
- Commit per step; style: `docs: <what>` (e.g. `docs: sync plans index with shipped code`)
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Ground-truth matrix

Run every probe and record the result in a scratch table (plan number → LANDED / NOT LANDED). A plan is LANDED only if its probe passes **on the current branch**:

| Plan | Probe | LANDED iff |
|------|-------|-----------|
| 001 | `ls internal/governance/governance_test.go internal/auth/session_test.go internal/warehouse/warehouse_test.go internal/ratelimit/ratelimit_test.go` | all exist |
| 002 | `grep -n "session expired" internal/auth/session.go` | match |
| 003 | `grep -c "ReadAuthorizer\|readAuthz" internal/server/server.go` ≥1 AND `grep -c "InsecureSkipVerify" internal/realtime/hub.go` = 0 | both |
| 004 | `ls internal/db/migrations/ \| grep -i visibility` AND `grep -n "SetVisibility" internal/admin/admin.go` | both |
| 005 | `grep -n "Access-Control-Allow-Origin" internal/sites/serve.go` | match |
| 006 | `grep -n "UNION" internal/warehouse/warehouse.go` | match |
| 007 | `grep -rn "datetime('now'" internal/` = none AND `ls internal/db/migrations/ \| grep -i usage_indexes` | both |
| 008 | `test -f internal/server/deploy_api.go` AND `grep -n "api/v1/deploy" internal/server/server.go` | both |
| 009 | `test -f internal/server/static/ui.css` AND `grep -n "ui.css" internal/server/static/home.html` | both |
| 010 | `grep -n "new-site" internal/server/static/home.html` AND `grep -ni "drop" internal/server/static/home.html` (drop-zone JS present) | both |

Then run `go build ./...` and `go test ./... -count=1` — both must pass before you document anything as shipped. Also record `git rev-parse --short HEAD` and the current migration list (`ls internal/db/migrations/`) — several later steps reference them.

**Verify**: the matrix has an explicit LANDED/NOT-LANDED entry for all ten plans; build and tests green.

### Step 2: Sync `plans/README.md`

- Status column: set each of 001–010 to DONE (probe passed) or TODO (failed); keep BLOCKED/REJECTED only with a one-line reason. Add rows for 008/009/010 if missing (Priority P1, Effort M; 010 depends on 008, 009).
- Dependency notes: delete or rewrite any note that no longer matches reality — specifically check the "commit the untracked `internal/db/migrations/003_ai_usage_calls.sql`" note (verify with `git status --short internal/db/migrations/ && ls internal/db/migrations/`: if the file doesn't exist or is already tracked, the note is stale) and the migration-numbering note (state the actual numbers now present).
- Backlog: strike items the landed code resolved. Specifically re-check: "CLI deploy bypasses the server" (if 008 landed, rewrite: endpoint exists; remaining = CLI/MCP client mode), and the DX/docs bullet (this plan itself resolves the faq/README-kv/.env.example/CLAUDE.md parts — remove them from the bullet, keep what remains, e.g. config reference).
- Add one line under the intro: `Last reconciled: <date> at <HEAD short SHA> (plan 011).`
- For any plan file whose self-declared status contradicted your probe (e.g. a header says DONE but the probe failed), append the one-line reconciliation note at its top per Scope.

**Verify**: every status row matches the step-1 matrix; `grep -n "Last reconciled" plans/README.md` → match.

### Step 3: CHANGELOG

Add an `## [Unreleased]` section above `[0.1.0]`, with entries ONLY for landed plans, grouped per Keep-a-Changelog:

- Under `### Added` (if landed): group-scoped site visibility + admin visibility endpoint (004); reflect-origin CORS for cross-site imports (005); HTTP deploy API `POST /api/v1/deploy` (008); design system + redesigned home/admin/error pages (009); browser Drop-to-Deploy (010); characterization test suite (001).
- Under `### Fixed` (if landed): expired-session handling (002); warehouse SQL guard bypasses (006); quota portability on Postgres + usage indexes + JSON body caps (007).
- Under `### Security` (if landed): governed-mode visibility enforcement on static serving, WebSockets, KV, and site listing + WebSocket Origin validation (003); HTML-escaping of the site name on the 404 page (009, if that fix is present — `grep -n "EscapeString" internal/sites/serve.go`).

Write entries in the file's existing voice (terse feature bullets, no plan numbers in the text — put `(plan NNN)` at line end is acceptable but match: the existing file doesn't use them, so prefer omitting).

**Verify**: `grep -n "Unreleased" CHANGELOG.md` → match; no entry exists for a NOT-LANDED plan.

### Step 4: README.md

All conditional on the matrix:

- Features table: add `| Key-value | artifact.kv.set / .get |` (unconditional — kv shipped in 0.1.0; confirm with `grep -n "kv" sdk/src/artifact.ts`).
- If 010 landed: add a row `| Drop to deploy | drag a file/folder/zip onto the home page |` and one quickstart line after the CLI flow: "Or skip the CLI: open `http://localhost:8443` and drag a folder onto the page."
- If 008 landed: in the Features table or below it, mention the deploy API one-liner (`POST /api/v1/deploy`).
- Fix the deploy table rows: "GCP starter (GCS + Cloud SQL) | `deploy/terraform/gcp/`", same for AWS, and "Header-trust (Pomerium) | `deploy/recipes/pomerium.md`". Add a sentence under the table: "Terraform examples are starting points — add load balancers, networking, and your identity proxy per your org."
- Check every relative link in README resolves (`ls` each target).

**Verify**: `grep -n "artifact.kv" README.md` → match; every path in README's tables exists on disk.

### Step 5: CONTEXT.md + ADR status lines

- If 004 landed: in CONTEXT.md §"Governed mode visibility levels", replace the "current code only has owner-private and public — group-scoped is a pre-launch gap" sentence with "All three levels are implemented; group membership comes from the IdP `groups` claim." And in `docs/adr/0003-…md` change `**Status:** Accepted` to `**Status:** Accepted — implemented (<short SHA>, <date>)`.
- If 005 landed: in CONTEXT.md §"Ecosystem", replace "(pre-launch gap — see ADR 0004)" with "(implemented — see ADR 0004)". In `docs/adr/0004-…md` change the status line to `**Status:** Accepted — implemented (<short SHA>, <date>)`.
- If 008/010 landed: append a short CONTEXT.md glossary entry: `## Drop to deploy` — two sentences, in the glossary's voice, naming the trust-bubble reason it needs no extra auth. Do NOT add entries for unlanded features.
- Touch nothing else in the ADRs — Decision/Context sections are immutable.

**Verify**: `grep -rn "pre-launch gap" CONTEXT.md docs/adr/` returns only lines whose feature is genuinely NOT landed (cross-check the matrix).

### Step 6: docs/ + skills/SKILL.md

- `docs/faq.md`: add the ADR-0002-required entry (unconditional): heading "What can't Artifact do?" with the three concrete examples — sending email, scheduled/cron jobs, calling internal APIs with secret tokens — each with the one-line alternative ("use Slack notifications", "run a separate service", "not a fit — by design") and a pointer to ADR 0002.
- `docs/quickstart.md`: if 010 landed, add the browser path ("drag a folder onto the home page") alongside the CLI path; verify the port/URL it states matches `config.DefaultDev()` (`:8443`).
- `docs/sdk-reference.md`: add a header note: "Quick reference table. For full examples, constraints, and CLI/MCP usage, see `skills/SKILL.md` (dropped into every project by `artifact init`) — that file is canonical." Verify each listed method exists in `sdk/src/artifact.ts`; add `kv` if missing.
- `docs/README.md` (create): a 10-line table of contents — Quickstart, Concepts, FAQ, Okta auth, SDK reference, ADRs — one line each, relative links.
- If 008 landed, create `docs/http-api.md`: a one-page reference for `POST /api/v1/deploy` (multipart contract, the 200/409/422 shapes — copy them from `internal/server/deploy_api.go`'s actual behavior, not from plan 008) and, if 004 landed, `PUT /api/v1/admin/sites/{site}/visibility`. Link it from docs/README.md.
- `skills/SKILL.md`: verify every documented SDK call against `sdk/src/artifact.ts`; if 004 landed, ensure the governed-mode section mentions the three visibility levels. Apply the same check to the skill template inside `internal/cli/init.go` — but since that's a `.go` file (out of scope), if its embedded docs are stale, record it in plans/README.md's backlog instead of editing.

**Verify**: `grep -n "What can't Artifact do" docs/faq.md` → match; `ls docs/README.md` → exists; every relative link added resolves.

### Step 7: CLAUDE.md (create) — including the docs-maintenance contract

Create a repo-level `CLAUDE.md` (~80 lines) for agents working on Artifact itself:

1. **What this is** — two sentences + pointer to CONTEXT.md (domain glossary, canonical vocabulary) and MISSION.md.
2. **Commands** — the Makefile targets verbatim (`make dev/build/test/lint/sdk/e2e`), plus `go test ./... -count=1 -race` as the full gate.
3. **Architecture in six lines** — single binary; chi server; subdomain routing; auth modes (dev/OIDC/header-trust); storage drivers; SQLite/Postgres via migrations in `internal/db/migrations/` (sequential, embedded, both-dialect SQL).
4. **Hard constraints** — ADR 0002 verbatim quote (no backends/cron/per-site secrets — closed as by-design); trust-bubble assumption; no external assets in embedded pages; no build steps for static UI.
5. **Conventions** — table-driven tests (exemplar: `internal/config/config_test.go`); conventional commits; `writeJSON`/`writeError` for handlers; design tokens from `static/ui.css` for any Artifact-owned page.
6. **Docs touchpoints (keep these in sync — the contract this plan establishes)**: a table mapping change-type → docs to update in the same PR: SDK surface → `skills/SKILL.md` + `docs/sdk-reference.md` + README features table; new endpoint → `docs/http-api.md`; governance/visibility behavior → CONTEXT.md + relevant ADR status; any shipped feature → CHANGELOG `[Unreleased]`; plan executed → its `plans/README.md` row. End with: "If your PR changes behavior and touches none of these, say why in the PR description."
7. **Plans process** — one paragraph: plans live in `plans/`, index is `plans/README.md`, statuses verified by probes not assertions, this file (011) is the reconciliation procedure to re-run after big batches.

### Step 8: `.env.example` (create)

One line per `*_env`-referenced variable found by `grep -rn "Env\b" internal/config/config.go` and the root `artifact.yaml` — expected set: `ARTIFACT_OIDC_SECRET`, `ARTIFACT_AI_KEY`, `ARTIFACT_DATABASE_URL`, `ARTIFACT_WAREHOUSE_CREDS`, `ARTIFACT_SLACK_SECRET`, `ARTIFACT_PROXY_SECRET`, plus the direct overrides `ARTIFACT_CONFIG`, `ARTIFACT_DOMAIN`, `ARTIFACT_AUTH_MODE`, `ARTIFACT_GOVERNANCE_MODE`. Dummy values + one-line comments. **Placeholder values only — never copy a real value from the environment or any local file.** Reference it from README's Development section.

### Step 9: Final verification sweep

1. `go build ./... && go test ./... -count=1` → green (proves no code was touched).
2. Link check: `grep -rhoE '\]\(([^)#]+)' README.md CHANGELOG.md CONTEXT.md CLAUDE.md docs/*.md plans/README.md | sed 's/](//' | sort -u` → for each relative path, `test -e` it; no missing targets.
3. Staleness greps: `grep -rn "pre-launch gap" CONTEXT.md docs/` and `grep -rn "artifact.kv" README.md` results consistent with the matrix.
4. `git status` → only in-scope files modified/created.

## Test plan

This plan's "tests" are the probe matrix (step 1), the link check, and the staleness greps (step 9) — all command-based. No Go tests are added or modified; the full suite passing untouched is itself a done criterion.

## Done criteria

- [ ] `go build ./... && go test ./... -count=1` exit 0 with zero non-doc files changed (`git status`)
- [ ] `plans/README.md` rows 001–011 all present, each matching its step-1 probe; "Last reconciled" line present
- [ ] CHANGELOG has `[Unreleased]` covering exactly the landed plans
- [ ] `grep -rn "pre-launch gap" CONTEXT.md docs/adr/` consistent with the matrix
- [ ] `docs/faq.md` contains the ADR-0002-required "not a fit" examples
- [ ] `CLAUDE.md`, `docs/README.md`, `.env.example` exist; README links them where specified
- [ ] All relative doc links resolve (step 9 check)
- [ ] No secret values anywhere in created files (`.env.example` is placeholders only)

## STOP conditions

Stop and report back (do not improvise) if:

- A probe gives a split verdict (e.g. `deploy_api.go` exists but the route is absent, or tests for a "landed" plan fail) — the work is half-merged; report which plan and what's missing instead of documenting it either way.
- `go test ./... -count=1` fails at step 1 — the tree is broken; reconciliation on a broken tree produces lies.
- You find documentation contradicting an ADR's *decision* (not just its status) — e.g. a doc promising cron or per-site secrets; flag it, don't silently rewrite product decisions.
- Two plan files claim the same migration number or the migrations on disk don't form a contiguous sequence — report the actual list.

## Maintenance notes

- This plan is **re-runnable by design**: after any future batch of plans lands, re-execute steps 1–3 and 9 (the matrix, the index, the changelog, the sweep). The CLAUDE.md "Docs touchpoints" table is what should make full re-runs rare.
- Reviewers: the highest-risk failure here is *documenting unlanded work as shipped* — check every CHANGELOG/README claim against the step-1 matrix in the PR description (the executor should paste the matrix there).
- Deferred deliberately: a full `docs/config-reference.md` for every `artifact.yaml` field (M-effort writing task; backlog), and updating the skill template inside `internal/cli/init.go` (code file — needs its own small plan if stale).
