# Plan 004: Implement group-scoped site visibility (ADR 0003 pre-launch requirement)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat f1702de..HEAD -- internal/db internal/governance internal/server`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition. (Plans 002–003 will have touched
> some of these files — that drift is expected; verify against their diffs.)

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MED
- **Depends on**: plans/003-enforce-governance-on-static-ws-kv.md
- **Category**: security / direction
- **Planned at**: commit `f1702de`, 2026-06-12

## Why this matters

ADR 0003 (docs/adr/0003-governed-mode-requires-group-visibility.md) decides, verbatim: "Group-scoped visibility (visible to employees in a named IdP group) is a **pre-launch requirement**, not a v0.2 deferral … Without group visibility, governed mode cannot support sites that hold sensitive data (HR, compensation, legal)." The current schema has only `visibility TEXT` with values `public`/anything-else; there is no way to scope a site to a named group. This plan adds the third visibility level the ADR requires: **owner-private** (default), **group-scoped**, **public**.

## Current state

- `docs/adr/0003-...md` specifies the design — read it first. Key consequences quoted: "`db.SiteRecord` needs a `visibility_groups []string` column (nullable; empty = owner-private or public). `Governor.CanReadSite` must check group membership against `user.Groups` when `existing.Visibility == "group"`."
- `internal/db/migrations/` — sequential SQL files embedded via `embed.FS` (`internal/db/migrate.go:12`). Existing: `001_initial.sql`, `002_sessions.sql`, `003_ai_usage_calls.sql` (003 is currently untracked in git — if it is absent on your checkout, STOP). The `sites` table from 001:

```sql
CREATE TABLE IF NOT EXISTS sites (
    name TEXT PRIMARY KEY,
    owner TEXT,
    deploy_id TEXT NOT NULL,
    deployed_by TEXT NOT NULL,
    deployed_at DATETIME NOT NULL,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    visibility TEXT NOT NULL DEFAULT 'public'
);
```

- `internal/db/migrate.go` — read it fully before writing the migration: it applies files in name order and contains a `postgresShim` that rewrites some SQLite syntax for Postgres. Your migration must be valid for BOTH drivers (plain `ALTER TABLE sites ADD COLUMN visibility_groups TEXT NOT NULL DEFAULT '[]'` is portable).
- `internal/db/db.go` — `SiteRecord` struct and its scan sites (`GetSite`, `ListSites`, `UpsertSite`/equivalent — find every `SELECT ... FROM sites` and `INSERT/UPDATE ... sites` and extend the column list). Groups are serialized as a JSON string elsewhere in this codebase (`sessions.groups_json`, see `internal/auth/session.go:55-67`); reuse that convention: store `visibility_groups` as a JSON array string, expose `VisibilityGroups []string` on the struct. Use `encoding/json` for marshal/unmarshal in the db layer (do NOT copy the hand-rolled parser from session.go).
- `internal/governance/governance.go` — `CanReadSite` (after plan 003: trust-mode short-circuit, public allow, owner allow, admins allow). Add the group branch per the ADR:

```go
if existing.Visibility == "group" {
	for _, g := range user.Groups {
		for _, allowed := range existing.VisibilityGroups {
			if g == allowed { return nil }
		}
	}
}
```

  (Order: check the group branch before falling through to deny; owner and admins must still pass for group sites.)
- `auth.User.Groups` is populated from the IdP groups claim in OIDC (`internal/auth/oidc.go:156-166`), hardcoded `["employees"]` in header-trust, `["employees","admins"]` in dev mode.
- How visibility gets SET today: inspect `internal/sites/deploy.go` and `internal/db` for where `visibility` is written on deploy (the `sites` row is upserted in `Deployer.Deploy`). There is currently no API or CLI to change a site's visibility. Minimal-scope decision for this plan: add an admin endpoint `PUT /api/v1/admin/sites/{site}/visibility` accepting `{"visibility": "private"|"group"|"public", "groups": ["g1"]}` — admin-only, in `internal/admin/admin.go`, following its `requireAdmin` pattern (admin.go:102-109).
- Vocabulary (CONTEXT.md, use these exact terms in errors/docs): "owner-private", "group-scoped", "public"; groups come from "the IdP `groups` claim".

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Build     | `go build ./...`         | exit 0              |
| Unit      | `go test ./internal/... -count=1` | all pass   |
| e2e       | `go test ./e2e/... -count=1 -timeout 5m` | all pass |
| Race      | `go test ./... -count=1 -race` | all pass      |

## Scope

**In scope**:
- `internal/db/migrations/004_visibility_groups.sql` (create)
- `internal/db/db.go` (SiteRecord + every sites-table query)
- `internal/governance/governance.go` + `governance_test.go`
- `internal/admin/admin.go` (visibility endpoint) + route registration in `internal/server/server.go`
- `internal/server/api_test.go` or a new `internal/admin/admin_test.go`

**Out of scope** (do NOT touch):
- Admin console UI (`internal/server/static/admin.html`) — backend only; UI is a follow-up.
- CLI surface for visibility — follow-up.
- Auth group sourcing (header-trust's hardcoded groups) — separate finding, noted in plans/README.md.
- The enforcement call sites added in plan 003 — they automatically pick up the new branch.

## Git workflow

- Branch: `advisor/004-group-visibility`
- Commit per step; style: `feat: add group-scoped site visibility (ADR 0003)`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Migration

Create `internal/db/migrations/004_visibility_groups.sql`:

```sql
ALTER TABLE sites ADD COLUMN visibility_groups TEXT NOT NULL DEFAULT '[]';
```

Confirm `internal/db/migrate.go` picks up new files automatically (it globs the embedded dir). Confirm the statement is Postgres-compatible (it is; no shim needed).

**Verify**: `go test ./internal/server/ -count=1` → pass (migrations run in test setup; failure here means the migration is malformed).

### Step 2: SiteRecord plumbing

In `internal/db/db.go`: add `VisibilityGroups []string` to `SiteRecord` (JSON tag `visibility_groups`). Find every query touching the `sites` table (`grep -n "FROM sites\|INTO sites\|UPDATE sites" internal/db/db.go`) and add the column: scan into a `string`, `json.Unmarshal` into the slice (empty/invalid → empty slice, never error the whole scan); marshal on write (nil slice → `"[]"`).

**Verify**: `go build ./...` → exit 0; `go test ./internal/... -count=1` → pass.

### Step 3: Governance branch

Add the group branch to `CanReadSite` as shown in Current state. Update `internal/governance/governance_test.go` with cases: group site + user in group → allowed; group site + user not in group → denied; group site + owner not in group → allowed; group site + admin not in group → allowed; group site + empty `VisibilityGroups` → denied for non-owner (fail closed).

**Verify**: `go test ./internal/governance/ -count=1 -v` → PASS.

### Step 4: Admin visibility endpoint

In `internal/admin/admin.go`, add `SetVisibility(w, r)`: `requireAdmin` gate; parse `{"visibility": string, "groups": []string}` with `http.MaxBytesReader(w, r.Body, 1<<20)`; validate visibility ∈ {`private`,`group`,`public`}; require non-empty `groups` when visibility is `group`; persist via a new `db.UpdateSiteVisibility(ctx, site, visibility, groups)` (404 if the site doesn't exist); write an audit entry (`action: "visibility_change"`, follow the pattern at `internal/server/api.go:90-93`). Register in `internal/server/server.go` next to the other admin routes (server.go:139-142): `r.Put("/api/v1/admin/sites/{site}/visibility", adminHandler.SetVisibility)` — note the other admin routes don't use URL params; import of `chi` in admin.go is needed for `chi.URLParam`.

**Verify**: `go build ./...` → exit 0.

### Step 5: Tests

New `internal/admin/admin_test.go` (temp-SQLite setup pattern from `internal/server/api_test.go:19-27`): non-admin → 403; admin sets `group` visibility with groups → 200 and the row round-trips via `GetSite` with `VisibilityGroups` intact; invalid visibility value → 400; `group` with empty groups → 400. Plus one integration case in `internal/server/api_test.go`: governed mode, group-scoped site, KV get as in-group non-owner → 200, as out-of-group → 403 (exercises plan 003's enforcement with the new level).

**Verify**: `go test ./... -count=1 -race` → all pass.

## Test plan

See steps 3 and 5: ≥5 governance unit cases, ≥4 admin endpoint cases, 1 end-to-end-ish governed KV case. `go test ./... -count=1 -race` green.

## Done criteria

- [ ] `go test ./... -count=1 -race` exits 0
- [ ] Migration 004 exists and applies on both fresh SQLite (unit tests) — and contains only portable SQL
- [ ] `CanReadSite` grants group-scoped access only to members, owner, admins (tests prove all three)
- [ ] Admin can set all three visibility levels via the new endpoint (tests)
- [ ] Only in-scope files modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back if:

- `internal/db/migrations/003_ai_usage_calls.sql` is missing from your checkout (the numbering for your new migration would collide).
- Plan 003 has not landed (no authorization on static serving) — group visibility without enforcement points is meaningless; report rather than proceed.
- `migrate.go`'s Postgres shim rejects or mangles the ALTER TABLE — report the shim behavior.
- The `sites`-table queries in db.go differ structurally from what `grep` finds (e.g. queries built dynamically).

## Maintenance notes

- Header-trust mode hardcodes `Groups: ["employees"]` (`internal/auth/header_trust.go:38`) — group-scoped sites are unusable in header-trust deployments until a groups header is supported. Recorded as a follow-up finding in plans/README.md; mention it in the PR description.
- Admin console UI for the visibility selector is the natural next step (ADR 0003 consequences mention it).
- Reviewers: check fail-closed behavior — `visibility = "group"` with empty groups must deny non-owners, not allow.
