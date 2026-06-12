# Plan 007: Fix quota enforcement on Postgres, add usage-table indexes, cap JSON request bodies

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat f1702de..HEAD -- internal/ai internal/warehouse/warehouse.go internal/db internal/server/api.go internal/notify`
> Plans 003/004/006 touch some of these files — verify against their diffs;
> on any other mismatch with the excerpts below, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none (merge after 006 to avoid conflicts in warehouse.go)
- **Category**: bug / perf
- **Planned at**: commit `f1702de`, 2026-06-12

## Why this matters

Three related robustness gaps on the API hot paths:

1. **Quotas silently don't work on Postgres.** The AI and warehouse daily-quota checks use SQLite-only syntax — `timestamp > datetime('now', '-1 day')` — and both swallow the query error (`_ = …QueryRowContext(...).Scan(&count)`). On a Postgres deployment (the documented production path), the query errors, `count` stays 0, and quotas are never enforced. Same class of bug: the AI usage insert error is discarded, so quota accounting can silently fail too.
2. **The quota/audit queries have no indexes.** `audit_log`, `ai_usage`, and `uploaded_files` have zero secondary indexes; every AI request and warehouse query runs a full-table scan of an append-only table that grows forever.
3. **JSON bodies are unbounded.** Every JSON endpoint decodes `r.Body` without a size cap (file uploads correctly use `http.MaxBytesReader` — `internal/files/files.go:53`; nothing else does). A multi-GB body OOMs the single binary that serves the whole company.

## Current state

- `internal/ai/ai.go:151-164` — the quota check (same shape in `internal/warehouse/warehouse.go:106-119` with an extra `action='warehouse_query'` predicate):

```go
var total int
_ = h.db.QueryRowContext(r.Context(),
	`SELECT COUNT(*) FROM ai_usage WHERE user_email=? AND timestamp > datetime('now', '-1 day')`,
	email).Scan(&total)
```

- `internal/ai/ai.go:113` — usage insert, error discarded: `_ = h.db.InsertAIUsage(...)`.
- `internal/ai/ai.go:116-149` — `Image` handler: has NO quota check and NO usage insert (the `Chat` handler has both). Both endpoints share the `aiLimiter` rate limit in `server.go:132-133`, but daily quotas skip Image entirely.
- `internal/db/db.go` — `QueryRowContext` passes through to `database/sql`; the placeholder shim is in `internal/db/shim.go` / `migrate.go` (read both to see how `?` placeholders are handled for Postgres — pgx requires `$1`; check how other queries work on Postgres before assuming).
- `internal/db/migrations/` — `001_initial.sql` (tables, one index on documents), `002_sessions.sql`, `003_ai_usage_calls.sql`. No indexes on `audit_log(user_email, timestamp)`, `ai_usage(user_email, timestamp)`, `uploaded_files(site, uploaded_at)`.
- JSON decode sites without body caps (grep `json.NewDecoder(r.Body)`):
  - `internal/server/api.go:75, 145, 188` (create, update, kv-set)
  - `internal/ai/ai.go:58, 126` (chat, image)
  - `internal/warehouse/warehouse.go:51` (query)
  - `internal/notify/slack.go:37` (slack)
- The portable-time fix: compute the cutoff in Go and pass it as a parameter — works identically on SQLite and Postgres:

```go
cutoff := time.Now().Add(-24 * time.Hour)
err := h.db.QueryRowContext(r.Context(),
	`SELECT COUNT(*) FROM ai_usage WHERE user_email=? AND timestamp > ?`,
	email, cutoff).Scan(&total)
if err != nil {
	return fmt.Errorf("quota check failed: %w", err)   // fail closed
}
```

  Failing closed (erroring the request when the quota query fails) is the correct posture: a quota that can't be evaluated must not silently grant.

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Build     | `go build ./...`         | exit 0              |
| Unit      | `go test ./internal/... -count=1` | all pass   |
| Full      | `go test ./... -count=1 -race` | all pass      |

## Scope

**In scope**:
- `internal/ai/ai.go` + new `internal/ai/ai_test.go`
- `internal/warehouse/warehouse.go` (quota function only — guards belong to plan 006)
- `internal/db/migrations/005_usage_indexes.sql` (create; if plan 004 hasn't landed and 004 doesn't exist yet, name this `004_usage_indexes.sql` and note it in plans/README.md so plan 004 renumbers)
- `internal/server/api.go`, `internal/notify/slack.go` (body caps)

**Out of scope** (do NOT touch):
- Warehouse SQL guards / row caps (plan 006).
- The file-upload path (already capped).
- Retention/cleanup of usage tables — deferred (noted in maintenance).
- Schema changes beyond indexes.

## Git workflow

- Branch: `advisor/007-quota-portability`
- Commit per step; style: `fix: make daily quotas portable across SQLite/Postgres and fail closed`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Portable, fail-closed quota checks

In `internal/ai/ai.go` `checkCallQuota` and `internal/warehouse/warehouse.go` `checkDailyQuota`: replace `datetime('now', '-1 day')` with a Go-computed `cutoff` parameter (excerpt above), check the Scan error, and return it wrapped (fail closed). Callers already convert the error to HTTP 429 — adjust: a real DB failure should be 500, not 429. Give the quota functions a distinct error for "limit reached" (e.g. keep the existing message) and return DB errors separately; in the handlers, branch: limit-reached → 429, other error → 500 with `{"error":"quota check failed"}`.

**Verify**: `go build ./...` → exit 0.

### Step 2: Meter the Image endpoint and stop discarding the usage insert

In `internal/ai/ai.go`:

- `Image` (line 116): add the same `checkCallQuota` call as `Chat` (line 52), and the same `InsertAIUsage` after a successful upstream response.
- Both handlers: check the `InsertAIUsage` error; on failure, log it (`log/slog` is used in `internal/server/server.go` — but the ai package has no logger; keep it simple: ignore-but-comment is NOT acceptable here; return nothing but write a `slog.Default().Warn("ai usage insert failed", "err", err)`).

**Verify**: `go build ./...` → exit 0.

### Step 3: Indexes migration

Create `internal/db/migrations/005_usage_indexes.sql` (see Scope note about numbering):

```sql
CREATE INDEX IF NOT EXISTS idx_audit_user_action_time ON audit_log(user_email, action, timestamp);
CREATE INDEX IF NOT EXISTS idx_audit_site_time ON audit_log(site, timestamp);
CREATE INDEX IF NOT EXISTS idx_ai_usage_user_time ON ai_usage(user_email, timestamp);
CREATE INDEX IF NOT EXISTS idx_uploaded_files_site_time ON uploaded_files(site, uploaded_at);
```

All four statements are portable SQLite/Postgres. Confirm `internal/db/migrate.go` applies new files automatically.

**Verify**: `go test ./internal/server/ -count=1` → pass (migrations run in test setup).

### Step 4: Body caps on JSON endpoints

At the top of each handler listed in Current state (before `json.NewDecoder`), add:

```go
r.Body = http.MaxBytesReader(w, r.Body, 1<<20) // 1 MiB JSON cap
```

Use 1 MiB everywhere except `internal/ai/ai.go` Chat (use `4<<20` — long prompts/conversations are legitimate). When `Decode` then fails due to the cap, the existing "Invalid JSON body." 400 responses fire; that's acceptable (no need to special-case 413).

**Verify**: `go build ./...` → exit 0.

### Step 5: Tests

New `internal/ai/ai_test.go` (temp-SQLite pattern from `internal/server/api_test.go:19-27`):

- `checkCallQuota` with `AIDailyCallsPerUser: 2`: insert 1 usage row with current timestamp → check passes; insert another → check returns limit-reached; insert rows older than 24h only → passes (rolling window).
- Quota disabled (`0`) → always passes.

In `internal/server/api_test.go`: add one case POSTing a >1 MiB body to `/api/v1/db/entries` → 400.

**Verify**: `go test ./... -count=1 -race` → all pass.

## Test plan

Step 5: ≥4 quota cases (under/at/rolling-window/disabled), 1 oversized-body case. Note the quota tests run against SQLite; the portability claim is "no SQLite-only SQL functions remain" — enforce via done-criteria grep rather than a live Postgres test.

## Done criteria

- [ ] `go test ./... -count=1 -race` exits 0
- [ ] `grep -rn "datetime('now'" internal/` → no matches
- [ ] `grep -rn "json.NewDecoder(r.Body)" internal/ | wc -l` matches the count of handlers, and each has a `MaxBytesReader` line above it (`grep -B3 "json.NewDecoder(r.Body)" internal/<pkg>/*.go` shows the cap)
- [ ] `Image` endpoint quota-checked and metered (test or grep `checkCallQuota` → 2 call sites in ai.go)
- [ ] Indexes migration applies cleanly in tests
- [ ] Only in-scope files modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back if:

- `internal/db`'s placeholder handling does NOT translate `?` for Postgres (check `shim.go`/`migrate.go`/`db.go`; if other runtime queries use `?` and the README claims Postgres support, assume translation exists somewhere — find it; if it genuinely doesn't exist, the Postgres path is more broken than this plan assumes: report).
- Migration numbering conflicts with plan 004's migration (coordinate via plans/README.md status).
- The `time.Time` parameter comparison misbehaves on SQLite (timestamps stored as strings) — if the rolling-window test fails, inspect how `InsertAIUsage` stores timestamps and report the format mismatch rather than hacking string formats.

## Maintenance notes

- `ai_usage` and `audit_log` grow forever; retention (e.g. 90-day sweep) is a natural follow-up, deferred deliberately.
- Reviewers: confirm fail-closed quota behavior doesn't take down AI endpoints when the DB hiccups — the 500-vs-429 split in step 1 is what to scrutinize.
- If/when an `artifact.kv` quota is added, follow the same portable-cutoff pattern.
