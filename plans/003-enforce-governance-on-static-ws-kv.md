# Plan 003: Enforce governed-mode visibility on static serving, WebSockets, KV, and site listing

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat f1702de..HEAD -- internal/server internal/sites/serve.go internal/realtime/hub.go internal/governance`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MED
- **Depends on**: plans/001-test-harness-and-characterization-tests.md (governance characterization tests must exist first)
- **Category**: security
- **Planned at**: commit `f1702de`, 2026-06-12

## Why this matters

Governed mode promises: "First deployer owns the site; visibility enforced" (CONTEXT.md). In the current code, visibility is enforced on exactly one path — the cross-site DB read in `handleList`. Everything else is open:

1. **Static serving** (`server.go:145-155` → `StaticHandler.ServeHTTP`) never calls `CanReadSite`. Any employee can open `private-hr-site.artifact.corp.com` and read every page of an owner-private site.
2. **WebSockets** (`hub.go:115-138`) never calls `CanReadSite`, and `websocket.Accept` is called with `InsecureSkipVerify: true` (line 122), which disables browser Origin checking. Any employee — and any page able to make a same-site request — can subscribe to a private site's realtime DB events and room messages.
3. **KV endpoints** (`api.go:182-208`) have no governance check at all (compare `handleCreate`, which checks `CanWriteDB`). Reads and writes of any site's KV are open in governed mode.
4. **Site listing** (`server.go:204-214`) returns every site record (name, owner, deploy metadata) to every employee.

Until this lands, governed mode does not deliver its core promise, which CONTEXT.md calls the enterprise adoption justification. This plan closes the enforcement gaps for the two visibility levels that exist today (owner-private, public). Group-scoped visibility (ADR 0003) is plan 004, built on top of this.

## Current state

- `internal/governance/governance.go:49-60` — the check to apply (trust mode short-circuits to allow, so all changes below are no-ops in trust mode):

```go
func (g *Governor) CanReadSite(ctx context.Context, user *auth.User, site string, existing *db.SiteRecord) error {
	if g.IsTrustMode() || existing == nil {
		return nil
	}
	if existing.Visibility == "public" {
		return nil
	}
	if existing.Owner == user.Email {
		return nil
	}
	return fmt.Errorf("governed mode: you do not have access to site %q", site)
}
```

  Note: admins currently cannot read private sites via this function. Per the audit decision, **add an admins bypass** in this plan (see step 1) so the admin console story is coherent — `CanDeploy` already has the same bypass (governance.go:34-38).

- `internal/server/server.go:145-155` — the static fall-through route. `staticHandler.ServeHTTP(w, r)` is called with no governance:

```go
r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
	if s.cfg.IsApexHost(r.Host) { s.serveHome(w, r); return }
	if s.cfg.IsAdminHost(r.Host) { s.serveAdmin(w, r); return }
	staticHandler.ServeHTTP(w, r)
})
```

- `internal/sites/serve.go:25-35` — `StaticHandler` resolves `site := h.cfg.SiteFromHost(r.Host)` itself; it has `cfg`, `store`, `cache` fields but no governor and no db (it cannot look up `SiteRecord`). The site record lives in `internal/db` (`GetSite`).
- `internal/realtime/hub.go:115-138` — `ServeWS` gets `u := auth.UserFromContext(r.Context())` and `site := h.cfg.SiteFromHost(r.Host)` then accepts unconditionally with `InsecureSkipVerify: true`.
- `internal/server/api.go:182-208` — `handleKVSet`/`handleKVGet`: no `CanWriteDB`/`CanReadSite` calls (compare `handleCreate` at api.go:69-73, the pattern to copy).
- `internal/server/server.go:204-214` — `listSites` returns `s.db.ListSites(...)` unfiltered. It is mounted at `/api/v1/sites` (server.go:123) *outside* the `RequireUser` group, so `u` may be nil in dev… actually `auth.Middleware` always sets a user in dev/header-trust/oidc-valid cases; treat nil user as "filter to public only".
- Trust bubble reminder (CONTEXT.md): in **trust mode nothing changes** — `IsTrustMode()` short-circuits every check. All new enforcement is governed-mode-only. Do not add checks that alter trust-mode behavior.
- e2e governed-mode test exemplar: `e2e/realtime_test.go:80-109` (governed redeploy denial) — shows how e2e spins up a server with `ARTIFACT_GOVERNANCE_MODE=governed`.

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Build     | `go build ./...`         | exit 0              |
| Vet       | `go vet ./...`           | exit 0              |
| Unit      | `go test ./internal/... -count=1` | all pass   |
| e2e       | `go test ./e2e/... -count=1 -timeout 5m` | all pass |
| Race      | `go test ./... -count=1 -race` | all pass      |

## Scope

**In scope**:
- `internal/governance/governance.go` — admins bypass in `CanReadSite`
- `internal/governance/governance_test.go` — flip the "admins denied" characterization
- `internal/sites/serve.go` — accept an authorizer hook
- `internal/server/server.go` — wire governance into static route + filter `listSites`
- `internal/realtime/hub.go` — origin check + read authorization in `ServeWS`
- `internal/server/api.go` — governance on KV handlers
- `internal/server/api_test.go`, `internal/sites/serve_test.go` — new cases
- `e2e/` — one new governed-visibility e2e test file (optional but preferred)

**Out of scope** (do NOT touch):
- Group-scoped visibility / `visibility_groups` column — that is plan 004.
- CORS headers on static responses — plan 005 (note: plan 005's credentialed cross-origin fetches will pass through the gate you add here; that's intended — ADR 0004 explicitly says "In governed mode, CanReadSite should still gate access even for cross-origin static requests").
- The deploy path (`CanDeploy` already enforced in `sites/deploy.go`).
- `internal/files/files.go` — file serving is keyed to the requesting site's own uploads; leave as is.
- Cookie/session mechanics.

## Git workflow

- Branch: `advisor/003-governed-read-enforcement`
- Commit per step; style: `fix: enforce governed-mode visibility on <surface>`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Admins bypass in CanReadSite

In `internal/governance/governance.go`, after the owner check in `CanReadSite`, add the same admins loop used in `CanDeploy` (lines 34-38). Also guard `user == nil`: return the access-denied error (fail closed) if `user` is nil and the site is not public.

Update `internal/governance/governance_test.go`: the "admins denied private read" characterization flips to allowed; add a nil-user case (denied for private, allowed for public).

**Verify**: `go test ./internal/governance/ -count=1 -v` → PASS.

### Step 2: Authorization hook on StaticHandler

In `internal/sites/serve.go`, add a field to `StaticHandler`:

```go
type ReadAuthorizer func(ctx context.Context, user *auth.User, site string) error
```

(import `internal/auth`; keep the function-type indirection so the `sites` package does not import `governance` or `db` — check for an import cycle before choosing a direct dependency instead). Extend `NewStaticHandler(cfg, store, cache)` to `NewStaticHandler(cfg, store, cache, authz ReadAuthorizer)`. In `ServeHTTP`, after resolving `site` (line 26-30) and before the cache lookup, when `authz != nil`:

```go
if err := h.authz(r.Context(), auth.UserFromContext(r.Context()), site); err != nil {
	http.Error(w, "You do not have access to this site.", http.StatusForbidden)
	return
}
```

**Verify**: `go build ./...` → exit 0 (the call site in server.go won't compile yet — fix in step 3; build after both).

### Step 3: Wire it in server.go

In `internal/server/server.go:103`, construct the handler with a closure that loads the site record and delegates:

```go
staticHandler := sites.NewStaticHandler(s.cfg, s.store, s.cache, func(ctx context.Context, u *auth.User, site string) error {
	if gov.IsTrustMode() { return nil }
	rec, _ := s.db.GetSite(ctx, site)
	return gov.CanReadSite(ctx, u, site, rec)
})
```

Note `gov` is declared at server.go:104 — move its declaration above the staticHandler line. The `IsTrustMode` fast path avoids a DB query per static request in trust mode (the default).

Also in `listSites` (server.go:204-214): in governed mode, filter the result to records where `gov.CanReadSite(r.Context(), auth.UserFromContext(r.Context()), rec.Name, &rec) == nil`. Trust mode returns the unfiltered list as today. (`listSites` is a method on `Server`; build a `governance.New(s.cfg)` locally or store the governor on `Server`.)

**Verify**: `go build ./...` → exit 0; `go test ./internal/... -count=1` → pass (update `serve_test.go` constructor calls: pass `nil` authorizer in existing tests).

### Step 4: WebSocket origin + read check

In `internal/realtime/hub.go` `ServeWS` (line 115):

1. Before `websocket.Accept`, enforce read access via a new optional hook on `Hub` — add field `Authz func(ctx context.Context, user *auth.User, site string) error` to the `Hub` struct; call it like step 2 (403 + return on error). Wire the same closure in `server.go` right after `hub` is available — set `s.hub.Authz = ...` in `routes()` (the hub is created in `New` before `routes` runs; setting the field there is fine since `routes` runs before listening).
2. Replace `&websocket.AcceptOptions{InsecureSkipVerify: true}` with:

```go
&websocket.AcceptOptions{OriginPatterns: []string{"*." + h.cfg.Domain, h.cfg.Domain}}
```

For dev (`Domain == "localhost"`), hosts look like `my-poll.localhost:8443` — `coder/websocket` matches OriginPatterns against the Origin host with `filepath.Match`-style patterns and ignores ports. Add `"*.localhost"` and `"localhost"` when `h.cfg.Domain == "localhost"`. Run the e2e realtime test to confirm patterns are right; if `e2e/realtime_test.go` dials without an Origin header (non-browser client), `websocket.Accept` allows it — only browser-sent Origins are validated, which is the intent.

**Verify**: `go test ./e2e/... -count=1 -timeout 5m` → all pass (realtime e2e still connects).

### Step 5: Govern the KV endpoints

In `internal/server/api.go`, mirror the `handleCreate` pattern (api.go:69-73):

- `handleKVSet`: before writing, `existing, _ := a.db.GetSite(r.Context(), site)` then `a.gov.CanWriteDB(r.Context(), u, site, existing)` → 403 on error. Get `u := auth.UserFromContext(r.Context())` (currently the handler doesn't read the user).
- `handleKVGet`: same but `a.gov.CanReadSite` (read, not write).

**Verify**: `go test ./internal/server/ -count=1 -v` → PASS.

### Step 6: Tests for the new enforcement

- `internal/server/api_test.go`: add a governed-mode table — `cfg.Governance.Mode = "governed"`, seed a site record owned by `alice@co` with `visibility "private"` via `database.UpsertSite` (check the exact helper name in `internal/db/db.go`; if it's `UpsertSite(ctx, *SiteRecord)` use that, otherwise STOP and report), then: KV set as non-owner → 403; KV set as owner → 200; KV get as non-owner → 403.
- `internal/sites/serve_test.go`: with a denying authorizer → 403; with nil authorizer → 200 (trust-mode equivalence).
- Optional e2e: governed server, deploy as one user, fetch site as another → 403. Model after `e2e/realtime_test.go:80-109`.

**Verify**: `go test ./... -count=1 -race` → all pass.

## Test plan

See step 6. Minimum: 3 new api_test cases (KV governed), 2 serve_test cases (authorizer allow/deny), governance unit flips from step 1. All existing tests (including e2e) still pass — trust-mode behavior is unchanged by construction (`IsTrustMode` short-circuits).

## Done criteria

- [ ] `go test ./... -count=1 -race` exits 0, including e2e
- [ ] `grep -n "InsecureSkipVerify" internal/realtime/hub.go` → no matches
- [ ] `grep -n "CanReadSite\|CanWriteDB" internal/server/api.go` → matches in both KV handlers
- [ ] In governed mode, static fetch of a private site as non-owner returns 403 (covered by a test)
- [ ] Trust-mode: full e2e suite passes unchanged
- [ ] Only in-scope files modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back if:

- Importing `internal/auth` from `internal/sites` creates an import cycle (check with `go build ./...`) — report the cycle; do not restructure packages on your own.
- `internal/db` has no upsert/insert helper for site records usable from tests.
- The e2e realtime test fails after the OriginPatterns change and adjusting patterns for localhost doesn't fix it within two attempts — report the exact Origin header the test client sends.
- You find additional ungoverned read surfaces beyond the four listed (report, don't expand scope).

## Maintenance notes

- Plan 004 (group-scoped visibility) extends `CanReadSite` — the enforcement points added here are exactly where group checks will take effect; no new wiring should be needed there.
- Reviewers: scrutinize the trust-mode fast path in the static authorizer closure — a DB lookup per static request in trust mode would be a performance regression (the default mode).
- Deferred: per-request caching of `GetSite` lookups for governed-mode static serving (one small SQLite query per request; acceptable at internal-tool scale, revisit if p99 matters).
- Deferred: `/api/v1/sites` is also reachable without `RequireUser`; consider moving it inside the authenticated group in a follow-up (it already sits behind auth middleware, so the user is normally set).
