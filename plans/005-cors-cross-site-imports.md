# Plan 005: Add reflect-origin CORS headers to static responses (ADR 0004 pre-launch gap)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat f1702de..HEAD -- internal/sites/serve.go`
> Plan 003 adds an authorization hook to this file — that drift is expected;
> verify the serving logic below still matches otherwise. On any other
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: none (composes with plan 003 if it landed first)
- **Category**: direction / bug
- **Planned at**: commit `f1702de`, 2026-06-12

## Why this matters

ADR 0004 (docs/adr/0004-cross-site-imports-require-cors.md) records the decided design for the "ecosystem" feature: sites importing shared JS/CSS from other sites (`<script src="//utils.artifact.corp.com/lib.js">`). Because each site is a distinct browser origin, these imports are blocked without CORS headers — and `StaticHandler.ServeHTTP` currently sets none. The ADR's decision, verbatim: "`StaticHandler.ServeHTTP` must reflect the `Origin` header as `Access-Control-Allow-Origin` when that origin matches `*.{configured domain}` … Do **not** use `Access-Control-Allow-Origin: *`." This plan implements exactly that decision.

## Current state

- `internal/sites/serve.go:25-70` — `ServeHTTP` resolves the site from the Host header, fetches the object from storage, and sets these headers (lines 57-64):

```go
w.Header().Set("Content-Type", info.ContentType)
w.Header().Set("ETag", `"`+info.ETag+`"`)
w.Header().Set("Cache-Control", "public, max-age=300")
w.Header().Set("X-Content-Type-Options", "nosniff")
```

No `Access-Control-Allow-Origin`, no `Vary`.

- Config: `h.cfg.Domain` is the apex domain (e.g. `artifact.corp.com`, or `localhost` in dev — see `internal/config/config.go:214-233` `SiteFromHost` for how subdomains are parsed, including the `localhost` special case where hosts look like `my-poll.localhost:8443`).
- ADR 0004 constraints to honor, quoted: respond with the *reflected origin* (not `*`), add `Vary: Origin`, and "In governed mode, `CanReadSite` should still gate access even for cross-origin static requests (pass the session cookie through credentialed fetches)." If plan 003 landed, the authorization hook already runs before headers are written — do not bypass it.

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Build     | `go build ./...`         | exit 0              |
| Unit      | `go test ./internal/sites/ -count=1 -v` | PASS |
| Full      | `go test ./... -count=1 -race` | all pass      |

## Scope

**In scope**:
- `internal/sites/serve.go`
- `internal/sites/serve_test.go` (cases for the new headers)

**Out of scope** (do NOT touch):
- The JSON API (`/api/v1/*`) — no CORS there; cross-site API reads go through the server-side `?site=` parameter (`internal/server/api.go:100-110`), by design.
- `/artifact.js` serving in `internal/server/server.go` — same-origin per site already (each site loads it from its own subdomain).
- Preflight handling beyond a simple OPTIONS response for static assets — script/CSS imports and simple GETs don't preflight; do not build a general CORS framework.

## Git workflow

- Branch: `advisor/005-static-cors`
- Commit style: `feat: reflect-origin CORS on static responses (ADR 0004)`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Origin matcher

In `internal/sites/serve.go`, add:

```go
// allowedOrigin reports whether origin (e.g. "https://site-a.artifact.corp.com")
// is another site on this Artifact instance, per ADR 0004.
func (h *StaticHandler) allowedOrigin(origin string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}
	host := strings.Split(u.Host, ":")[0]
	d := h.cfg.Domain
	return host == d || strings.HasSuffix(host, "."+d)
}
```

(`net/url` import; `strings` is already imported.) Note this also matches the apex host — acceptable, the apex is the same trusted instance.

### Step 2: Reflect on responses

In `ServeHTTP`, with the other header writes (around line 57-64), add:

```go
if origin := r.Header.Get("Origin"); origin != "" && h.allowedOrigin(origin) {
	w.Header().Set("Access-Control-Allow-Origin", origin)
	w.Header().Set("Access-Control-Allow-Credentials", "true")
}
w.Header().Set("Vary", "Origin")
```

Set `Vary: Origin` unconditionally (the response varies on Origin whether or not this request sent one — required for the shared `Cache-Control: public` cache correctness). `Allow-Credentials` is needed because governed-mode cross-site fetches must send the session cookie (ADR 0004 consequences). Reflecting a specific origin + credentials is the safe pattern; `*` + credentials is forbidden by spec and the ADR.

Also apply the same headers on the 304 path — note the existing `If-None-Match` early return at lines 65-68 happens AFTER header writes in the current code order, so placing the CORS block before it covers both. Confirm by reading the final order.

**Verify**: `go build ./...` → exit 0.

### Step 3: Tests

In `internal/sites/serve_test.go` (exists after plan 001; otherwise create with the seeding pattern: a `LocalStore` in `t.TempDir()`, put `sites/demo/.artifact-current` = deploy id and one file under `sites/demo/deploys/<id>/`):

- Request with `Origin: http://other.localhost` → response has `Access-Control-Allow-Origin: http://other.localhost`, `Access-Control-Allow-Credentials: true`, `Vary: Origin`.
- Request with `Origin: https://evil.example.com` → NO `Access-Control-Allow-Origin` header; `Vary: Origin` present.
- Request with no Origin → no ACAO; `Vary: Origin` present.
- Origin equal to apex (`http://localhost`) → reflected.

**Verify**: `go test ./internal/sites/ -count=1 -v` → PASS; `go test ./... -count=1 -race` → all pass.

## Test plan

The four cases in step 3, table-driven, in `internal/sites/serve_test.go`. Plus manual smoke (optional, if a dev server is running): `curl -H "Origin: http://a.localhost:8443" http://b.localhost:8443/index.html -i` shows the reflected header.

## Done criteria

- [ ] `go test ./... -count=1 -race` exits 0
- [ ] `grep -n "Access-Control-Allow-Origin" internal/sites/serve.go` → exactly one write site, reflecting the request origin (no `*` anywhere: `grep -rn 'Allow-Origin", "\*"' internal/` → no matches)
- [ ] `Vary: Origin` set on all static responses (test proves it)
- [ ] External origins are not reflected (test proves it)
- [ ] Only in-scope files modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back if:

- `serve.go`'s header-writing order has changed such that the 304 path skips your CORS block.
- You're tempted to add CORS to any handler other than `StaticHandler` — that's a scope change the ADR doesn't cover.

## Maintenance notes

- If a CDN or shared cache is ever put in front of Artifact, `Vary: Origin` is what keeps reflected-origin responses from being served to the wrong origin — do not remove it.
- The matcher trusts every subdomain of the configured domain, which is the trust-bubble assumption (CONTEXT.md). If subdomain delegation ever changes (e.g. user-controlled DNS), revisit.
- Follow-up explicitly deferred: an `OPTIONS` preflight handler for static assets (only needed if sites start sending non-simple cross-site requests, e.g. `fetch` with custom headers).
