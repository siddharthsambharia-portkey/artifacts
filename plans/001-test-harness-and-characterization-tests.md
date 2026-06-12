# Plan 001: Build a test harness and characterization tests for the security-critical paths

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat f1702de..HEAD -- internal/governance internal/auth internal/warehouse internal/sites internal/storage internal/ratelimit internal/files`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: LOW
- **Depends on**: none
- **Category**: tests
- **Planned at**: commit `f1702de`, 2026-06-12

## Why this matters

13 of 16 internal packages have zero test files. The untested code includes every authorization decision (`internal/governance`), session parsing that feeds those decisions (`internal/auth/session.go` hand-rolls a JSON array parser), the regex guards that keep warehouse access read-only (`internal/warehouse`), and the path checks that keep static serving inside a site's folder. Plans 002–006 in this series change exactly these files; without characterization tests first, those changes can silently break auth or governance. This plan adds table-driven unit tests that pin down current behavior, so later plans change behavior deliberately and visibly.

**Important**: these are *characterization* tests — they assert what the code does TODAY, including known bugs. Two known bugs you will encounter (do NOT fix them here; later plans fix them and will flip the assertions):

1. `SessionStore.Get` returns `(nil, nil)` for an expired session (bug — fixed in plan 002). Write the test asserting today's behavior with a comment `// BUG: expired session returns nil error — see plans/002`.
2. The warehouse dataset allowlist is a substring match that a `UNION SELECT` against a non-allowed dataset passes (fixed in plan 006). Assert today's behavior with a comment pointing at plans/006.

## Current state

This is a Go 1.25 module, `github.com/siddharthsambharia-portkey/artifacts`. The only existing unit tests are:

- [internal/config/config_test.go](../internal/config/config_test.go) — table-driven tests, the structural pattern to copy.
- [internal/server/api_test.go](../internal/server/api_test.go) — shows how to stand up a real SQLite-backed `db.DB` in a temp dir:

```go
// internal/server/api_test.go:19-27
tmp := t.TempDir()
cfg := config.DefaultDev()
cfg.DataDir = filepath.Join(tmp, "data")
cfg.Database.URL = filepath.Join(tmp, "data", "test.db")
database, err := db.Open(cfg)
if err != nil { t.Fatal(err) }
database.Migrate(context.Background())
```

Files under test (read each fully before writing its tests — they are all short):

- `internal/governance/governance.go` (69 lines) — `CanDeploy`, `CanWriteDB`, `CanReadSite`, `IsAdmin`. Pure functions of `(user, site, existing *db.SiteRecord)` plus `cfg.Governance.Mode`. Note current behavior: `CanReadSite` does NOT grant admins access to private sites (only owner or `visibility == "public"`), and `existing == nil` always allows.
- `internal/auth/session.go` (113 lines) — `parseGroups`, `groupsJSON`, `trim`, `splitComma` (hand-rolled JSON array handling, lines 55–113), and `SessionStore.Get` (lines 29–42; the expired-session path at 37–39 returns `nil, err` where `err` is nil).
- `internal/warehouse/warehouse.go` — package-level regexes at lines 18–19:

```go
var selectOnly = regexp.MustCompile(`(?i)^\s*SELECT\b`)
var denyPatterns = regexp.MustCompile(`(?i)\b(INSERT|UPDATE|DELETE|DROP|ALTER|CREATE|TRUNCATE|GRANT|REVOKE|EXEC|EXECUTE)\b`)
```

  and `datasetAllowed` (lines 93–104, case-insensitive `strings.Contains` over `cfg.Warehouse.AllowedDatasets`).
- `internal/sites/serve.go` — `StaticHandler.ServeHTTP` path handling (lines 37–46): trims leading `/`, rejects any path containing `..` with HTTP 400, then builds `sites/%s/deploys/%s/%s`.
- `internal/storage/local.go` — `fullPath` (lines 27–33): `filepath.Clean`, rejects results starting with `..` by returning `""`.
- `internal/ratelimit/ratelimit.go` (63 lines) — token bucket `Allow(key)`; `New(requestsPerSecond float64, burst int)`.
- `internal/files/files.go` — `isDangerousContentType` (lines 151–159). Note current behavior: the check is case-sensitive `strings.HasPrefix`, so `TEXT/HTML` is NOT flagged. Characterize that (comment: case-sensitivity is a known gap, mitigated by attachment+nosniff headers on serve).

Repo conventions: table-driven tests with `tests := []struct{...}` and `t.Run(tt.name, ...)`, plain `testing` stdlib, no assertion libraries. Match [internal/config/config_test.go](../internal/config/config_test.go).

Domain vocabulary (from CONTEXT.md, use in test names/comments): "trust mode" = governance mode where everything is allowed; "governed mode" = ownership enforced; "site" = a deployed folder mapped to a subdomain.

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Build     | `go build ./...`         | exit 0              |
| Vet       | `go vet ./...`           | exit 0              |
| All tests | `go test ./... -count=1` | all pass            |
| One pkg   | `go test ./internal/governance/ -count=1 -v` | PASS |
| Race      | `go test ./... -count=1 -race` | all pass       |

## Scope

**In scope** (create only; no production code changes):
- `internal/governance/governance_test.go`
- `internal/auth/session_test.go`
- `internal/warehouse/warehouse_test.go`
- `internal/sites/serve_test.go`
- `internal/storage/local_test.go`
- `internal/ratelimit/ratelimit_test.go`
- `internal/files/files_test.go`

**Out of scope** (do NOT touch):
- Any non-`_test.go` file. This plan changes zero production code. If a test can't be written without refactoring production code, skip that case and note it in your report.
- `e2e/` — existing e2e tests stay as they are.
- OIDC flow tests (`internal/auth/oidc.go`) — requires mocking an OIDC issuer; deferred deliberately.

## Git workflow

- Branch: `advisor/001-characterization-tests`
- Commit message style (from `git log`): conventional commits, e.g. `test: add characterization tests for governance and auth session parsing`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Governance tests

Create `internal/governance/governance_test.go`. Construct `*config.Config` via `config.DefaultDev()` and flip `cfg.Governance.Mode` between `"trust"` and `"governed"`. Table-driven cases for `CanDeploy` / `CanWriteDB` / `CanReadSite`:

- trust mode: everything allowed regardless of owner/visibility.
- governed, `existing == nil`: allowed (new site).
- governed, `existing.Owner == ""`: allowed.
- governed, owner mismatch, no admin group: error.
- governed, owner mismatch, user has `"admins"` group: `CanDeploy` allowed.
- governed `CanReadSite`: `visibility == "public"` allowed; owner allowed; non-owner with `"admins"` group **denied** (characterize: admins cannot read private sites today — add comment that plan 004 revisits this).

`db.SiteRecord` is in `internal/db`; construct literals (`&db.SiteRecord{Owner: "a@co", Visibility: "private"}`). `auth.User` is in `internal/auth`.

**Verify**: `go test ./internal/governance/ -count=1 -v` → PASS, ≥10 subtests.

### Step 2: Session parsing + expiry tests

Create `internal/auth/session_test.go`:

- `parseGroups` round-trips with `groupsJSON`: `[]`, `["employees"]`, `["employees","admins"]`, a group containing a comma (`["a,b"]` — characterize whatever the current parser does with it; if it splits wrongly, assert the wrong output with a `// BUG:` comment), empty string input.
- `SessionStore` against a real temp SQLite DB (use the `api_test.go` setup pattern shown above; `auth.NewSessionStore(database)`): `Create` then `Get` returns the user with groups intact; `Get` with unknown id returns error; `Create` with `ttl = -1*time.Hour` (already expired) then `Get` — characterize today's behavior: returns `(nil, nil)`. Comment: `// BUG: expired session returns nil error and nil user — plan 002 changes this to a non-nil error. Flip this assertion then.`

**Verify**: `go test ./internal/auth/ -count=1 -v` → PASS.

### Step 3: Warehouse guard tests

Create `internal/warehouse/warehouse_test.go` testing the package-level regexes and `datasetAllowed` (construct `Handler{cfg: cfg}` directly; `datasetAllowed` doesn't need a querier):

- `selectOnly`: matches `SELECT 1`, `  select x`, rejects `INSERT ...`, `WITH t AS (...) SELECT` (characterize: CTE form is rejected today).
- `denyPatterns`: catches `INSERT/DROP/...` anywhere, including after a semicolon; does NOT catch `UNION` (characterize with `// gap closed in plans/006`).
- `datasetAllowed` with `AllowedDatasets: ["analytics.public"]`: allows `SELECT * FROM analytics.public.orders`; allows `SELECT * FROM analytics.public.orders UNION SELECT * FROM secret.salaries` (characterize the bypass, `// BUG: substring match — see plans/006`); denies a query not mentioning any allowed dataset; denies everything when allowlist is empty.

**Verify**: `go test ./internal/warehouse/ -count=1 -v` → PASS.

### Step 4: Static path-traversal tests

Create `internal/sites/serve_test.go`. Build a `StaticHandler` with a real `LocalStore` rooted in `t.TempDir()` (use `storage.NewLocal(tmp)`) and a `DeployCache` (`sites.NewDeployCache(store, 16)` — you are inside package `sites`, so call `NewStaticHandler` / `NewDeployCache` directly). Seed storage: `store.Put(ctx, "sites/demo/.artifact-current", strings.NewReader("123"), 3, "text/plain")` and `store.Put(ctx, "sites/demo/deploys/123/index.html", ...)`.

Cases via `httptest.NewRequest` with `req.Host = "demo.localhost:8443"` and `cfg := config.DefaultDev()`:

- `GET /` → 200, body matches seeded index.html.
- `GET /../secret`, `GET /a/../../b`, `GET /%2e%2e/x` (note: `httptest.NewRequest` keeps the encoded form in `URL.Path`? Verify what `r.URL.Path` contains; assert on observed status) → expect 400 for paths whose decoded `URL.Path` contains `..`; record actual behavior for the encoded variant.
- `GET /nonexistent.css` → 404.
- Unknown host (`req.Host = "localhost:8443"` apex) → 404 (site is empty).

**Verify**: `go test ./internal/sites/ -count=1 -v` → PASS.

### Step 5: Local storage fullPath tests

Create `internal/storage/local_test.go`: `fullPath` is unexported — test from inside the package. Cases: normal relative path joins under root; `../escape` → `""`; `a/../../escape` → `""`; absolute path input (characterize: `filepath.Clean("/etc/passwd")` = `/etc/passwd`, doesn't start with `..`, so it currently joins under root via `filepath.Join` — assert the joined result stays under root). Also `Put` then `Get` round-trip with content + ETag non-empty.

**Verify**: `go test ./internal/storage/ -count=1 -v` → PASS.

### Step 6: Rate limiter tests

Create `internal/ratelimit/ratelimit_test.go`: `New(10, 5)` → 5 immediate `Allow("k")` true, 6th false; separate keys independent; after `time.Sleep(150*time.Millisecond)` at rate 10/s at least one more allowed. Keep sleeps ≤200ms total so the suite stays fast.

**Verify**: `go test ./internal/ratelimit/ -count=1 -v` → PASS.

### Step 7: File content-type filter tests

Create `internal/files/files_test.go` for `isDangerousContentType`: `text/html` true; `text/html; charset=utf-8` true; `application/javascript` true; `image/png` false; `application/pdf` false; `TEXT/HTML` — characterize current (false) with comment about case-sensitivity.

**Verify**: `go test ./internal/files/ -count=1 -v` → PASS, then full suite `go test ./... -count=1 -race` → all pass.

## Test plan

This plan IS the test plan. Final state: 7 new `_test.go` files, ≥40 subtests total, all passing, zero production-code diffs.

## Done criteria

- [ ] `go build ./...` exits 0
- [ ] `go vet ./...` exits 0
- [ ] `go test ./... -count=1 -race` exits 0
- [ ] `git diff --stat` shows ONLY new `_test.go` files (no production file modified)
- [ ] Each known bug characterized has a `// BUG:` comment naming the follow-up plan
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- Any excerpt in "Current state" doesn't match the live code (drifted).
- A characterization test reveals behavior so broken the test can't express it (e.g. a panic in `parseGroups`) — report the input and stack instead of fixing.
- You find yourself editing a non-test file for any reason.
- `go test ./... -race` reports a data race in existing code — report it, don't fix it here.

## Maintenance notes

- Plans 002, 003, 004, 006 modify the code under these tests and must flip the `// BUG:` assertions they fix. Reviewers: reject any of those PRs that don't touch the corresponding characterization test.
- The deferred OIDC-flow tests (mock issuer via `httptest`) are the next-highest-value test investment after this lands.
