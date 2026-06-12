# Plan 006: Harden the warehouse read-only SQL guards (UNION/CTE/multi-statement/LIMIT bypass)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat f1702de..HEAD -- internal/warehouse`
> Plan 001 adds `warehouse_test.go` — that drift is expected. On any other
> mismatch with the excerpts below, treat it as a STOP condition.

## Status

- **Priority**: P2
- **Effort**: M
- **Risk**: MED
- **Depends on**: plans/001-test-harness-and-characterization-tests.md (characterization tests to flip)
- **Category**: security
- **Planned at**: commit `f1702de`, 2026-06-12

## Why this matters

Warehouse queries are a product feature — employees run SELECTs against company data, restricted by (a) read-only enforcement, (b) an allowed-datasets list, (c) a row limit, (d) a daily per-user quota. Three of the four restrictions are bypassable today:

- **Dataset allowlist**: `datasetAllowed` is a case-insensitive substring check, so `SELECT * FROM allowed.t UNION SELECT * FROM restricted.salaries` passes (the allowed name appears as a substring; nothing blocks `UNION` or additional table references).
- **Row limit**: drivers append `LIMIT n` only if the SQL doesn't already contain the substring "LIMIT", so `... LIMIT 999999999` bypasses the configured cap, and all rows are buffered in memory before encoding (memory exhaustion).
- **Multi-statement**: nothing rejects `;`, so on the Postgres driver a second statement can ride along (it still must dodge `denyPatterns`, but e.g. a second giant SELECT can).

This is defensive hardening of an intentionally-exposed query surface, not a redesign. Regex-based SQL guarding has known limits; the goal is closing the cheap, concrete bypasses and adding a hard server-side row cap, not building a SQL parser.

## Current state

- `internal/warehouse/warehouse.go:18-19` — the guards:

```go
var selectOnly = regexp.MustCompile(`(?i)^\s*SELECT\b`)
var denyPatterns = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE|GRANT|REVOKE|EXEC|EXECUTE)\b`)
```

- `internal/warehouse/warehouse.go:55-70` — enforcement order in `Query`: selectOnly → denyPatterns → datasetAllowed → rowLimit default 10000 → 30s timeout → `h.querier.Query(ctx, req.SQL, limit)`.
- `internal/warehouse/warehouse.go:93-104` — `datasetAllowed`: returns true if any configured dataset name is a substring of the lowercased SQL; false when the allowlist is empty.
- `internal/warehouse/postgres.go:26-27` and `bigquery.go:32-33` — duplicated LIMIT injection:

```go
if !strings.Contains(strings.ToUpper(sqlText), "LIMIT") {
	sqlText = fmt.Sprintf("%s LIMIT %d", sqlText, rowLimit)
}
```

- `internal/warehouse/postgres.go:39-57` — `scanRows` appends every row with no cap and returns `result, nil` without checking `rows.Err()`.
- `internal/warehouse/driver.go` — the `Querier` interface: `Query(ctx, sql string, rowLimit int) ([]map[string]any, error)`.
- `internal/warehouse/snowflake.go` — wraps `postgresQuerier`; inherits its behavior, no separate changes needed.
- Characterization tests from plan 001 (`internal/warehouse/warehouse_test.go`) assert today's bypasses with `// BUG: … plans/006` comments — flip those.

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Build     | `go build ./...`         | exit 0              |
| Unit      | `go test ./internal/warehouse/ -count=1 -v` | PASS |
| Full      | `go test ./... -count=1 -race` | all pass      |

## Scope

**In scope**:
- `internal/warehouse/warehouse.go`
- `internal/warehouse/postgres.go`
- `internal/warehouse/bigquery.go`
- `internal/warehouse/driver.go` (shared `enforceRowCap`/`scanRows` helpers may live here)
- `internal/warehouse/warehouse_test.go`

**Out of scope** (do NOT touch):
- Adding a SQL parser dependency — explicitly rejected; regex + hard caps only.
- The quota query in `checkDailyQuota` (lines 106-119) — its SQLite/Postgres portability bug is plan 007.
- `internal/server/server.go` routing/rate limits.
- Streaming the response — buffered-with-hard-cap is acceptable at this scale; noted as deferred.

## Git workflow

- Branch: `advisor/006-warehouse-guards`
- Commit style: `fix: close warehouse SQL guard bypasses (UNION, multi-statement, LIMIT)`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Extend the deny rules

In `warehouse.go`, extend the guards:

```go
var denyPatterns = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE|GRANT|REVOKE|EXEC|EXECUTE|UNION|INTO|MERGE|CALL)\b`)
var denyTokens = regexp.MustCompile(`;|--|/\*`)
```

and in `Query`, after the existing denyPatterns check, reject when `denyTokens.MatchString(req.SQL)` with the error message: `{"error":"Query contains forbidden tokens (;, comments) — submit a single plain SELECT."}`. Keep the existing message for keyword matches.

Rationale: `UNION` is what defeats the substring allowlist; `;` defeats single-statement assumptions; comment tokens (`--`, `/*`) defeat any later textual reasoning and are rarely needed in ad-hoc SELECTs. `WITH`/CTEs remain rejected by `selectOnly` (must start with SELECT) — do not add WITH support here.

**Verify**: `go test ./internal/warehouse/ -count=1` → the characterization tests for UNION/comment cases now FAIL (expected — flip them in step 4). `go build ./...` → exit 0.

### Step 2: Always enforce the row cap server-side

Remove the conditional LIMIT injection from `postgres.go:26-27` and `bigquery.go:32-33`. Instead enforce the cap where rows are read:

- In `postgres.go` `scanRows` (and the BigQuery row loop at `bigquery.go:~40-55`): stop appending once `len(result) == rowLimit` and break. Pass `rowLimit` through to `scanRows` (signature change; `snowflake.go` delegates, check it still compiles).
- In `scanRows`, change the final `return result, nil` to `return result, rows.Err()`.
- Keep appending `LIMIT` text only as an optimization when absent (the old behavior is fine to keep for the no-LIMIT case — it saves the warehouse work); the in-loop cap is the enforcement.

**Verify**: `go build ./...` → exit 0.

### Step 3: Truncation visibility

In `warehouse.go` `Query`, after a successful query, include a truncation flag so users know results were capped:

```go
json.NewEncoder(w).Encode(map[string]any{"rows": rows, "truncated": len(rows) >= limit})
```

(Adding a key to the response object is backward-compatible for the SDK, which reads `rows`.)

**Verify**: `go build ./...` → exit 0.

### Step 4: Flip and extend tests

In `warehouse_test.go`:

- Flip the plan-001 characterizations: UNION exfiltration now denied; `;` now denied; `--` and `/*` now denied.
- New cases: `SELECT * FROM allowed.t LIMIT 999999` → allowed through guards but capped (unit-test the row cap by calling `scanRows`-level logic if reachable, otherwise note it's covered by the in-loop cap and test via a fake `Querier` asserting the handler passes `limit` ≤ configured cap).
- Confirm legit queries still pass: `SELECT a, b FROM allowed.t WHERE x = 'val' ORDER BY a DESC LIMIT 50`.
- Edge: column/string content containing the word "union" inside a quoted literal will now be rejected (false positive) — add a test documenting this accepted trade-off with a comment.

**Verify**: `go test ./internal/warehouse/ -count=1 -v` → PASS; `go test ./... -count=1 -race` → all pass.

## Test plan

Step 4 covers it: ≥8 guard cases (denied: UNION, semicolon, comments, INSERT, CTE; allowed: plain SELECT with WHERE/ORDER BY/LIMIT), row-cap behavior, truncation flag presence. All in `internal/warehouse/warehouse_test.go`, table-driven.

## Done criteria

- [ ] `go test ./... -count=1 -race` exits 0
- [ ] `grep -n "UNION" internal/warehouse/warehouse.go` → present in denyPatterns
- [ ] `grep -n "strings.Contains(strings.ToUpper" internal/warehouse/` → no LIMIT-bypass conditional remains as the only enforcement (in-loop cap exists in both drivers)
- [ ] A UNION query against a non-allowed dataset is rejected (test proves it)
- [ ] Response includes `truncated` flag
- [ ] Only in-scope files modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back if:

- The BigQuery row-iteration code differs materially from "a loop appending to result" (e.g. arrow-based batch reads) — report its shape before changing it.
- Flipping a characterization test reveals an additional bypass class not listed here (report; don't expand the deny list ad hoc).
- Legitimate-query tests fail after the deny extensions (a false-positive class bigger than quoted-literal "union") — report examples.

## Maintenance notes

- This remains regex-based guarding, deliberately. If warehouse usage grows past internal ad-hoc queries, the durable fix is warehouse-side enforcement: a read-only service account / database role for the configured credentials, which makes the application guards defense-in-depth instead of the only wall. Recommend operators do this in docs regardless.
- Reviewers: scan the deny list for false-positive risk against the examples in `examples/team-dashboard` (it ships warehouse queries).
- Deferred: streaming results instead of buffering (only needed if row caps rise).
