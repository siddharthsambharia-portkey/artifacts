# Plan 008: Add the HTTP deploy API — `POST /api/v1/deploy` (multipart files or zip)

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat 9e8565a..HEAD -- internal/server internal/sites/deploy.go internal/governance`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition. NOTE: this plan was written when
> plans 003–007 had landed; if executing on a branch where they haven't,
> verify the excerpts and ignore references to code those plans added.

## Status

- **Priority**: P1
- **Effort**: M
- **Risk**: MED
- **Depends on**: none
- **Category**: direction
- **Planned at**: commit `9e8565a`, 2026-06-12

## Why this matters

Artifact has no way to deploy over HTTP. `artifact deploy` (CLI) opens object storage and the database *directly* — so Builders need infrastructure credentials, the browser can never deploy, and governed-mode ownership is recorded as the hardcoded `dev@localhost`. This endpoint is the foundation for the Drop-to-Deploy browser experience (plan 010) and the eventual CLI client mode: an authenticated employee POSTs files (or a zip), the server stages them, runs the existing `Deployer` (which already enforces governance, quotas, manifests, atomic pointer flip, audit), and returns the live URL. After this plan, deploys are attributed to the real signed-in user for the first time.

**Hard product constraint (ADR 0002 — quote: "No custom backends, no cron, no per-site secrets — ever")**: Artifact hosts static files only. This endpoint must NOT run builds. If the upload looks like a source project (package.json without index.html), reject with a friendly message telling the user to drop their build output folder.

## Current state

- `internal/sites/deploy.go:45` — `Deployer.Deploy(ctx, siteName, sourceDir string, user *auth.User) (string, error)` does everything we need from a **local directory**: validates `validSiteName` (lowercase/digits/hyphens, ≤63), checks `gov.CanDeploy`, walks files (skipping dotfiles), enforces `Quotas.SiteMaxMB`, uploads with sha256 manifest, atomically flips `.artifact-current`, upserts the site record (`Owner` = first deployer), invalidates the deploy cache, writes an audit entry, returns the site URL. The HTTP handler should therefore **stage uploads into a temp dir and call this method unchanged** — do not reimplement deploy logic.
- `internal/server/server.go` — `routes()`. The authenticated group (`r.Use(auth.RequireUser)`, general `limiter = ratelimit.New(20, 50)`) mounts `/api/v1/*`. The `Server` struct already holds `s.deployer *sites.Deployer` (constructed in `New`).
- `internal/server/api.go` — `writeError(w, msg, code)` / `writeJSON(w, v)`: the response conventions to use.
- Body-cap convention: file uploads exemplar `internal/files/files.go:52-56` (`http.MaxBytesReader` sized from quota, then `ParseMultipartForm`).
- `internal/config/config.go` — `Quotas.SiteMaxMB` (default 500). Use it to cap the whole request body.
- Rate limiting exemplar: `server.go` — `r.With(ratelimit.Middleware(aiLimiter, rateKey)).Post(...)`.
- Audit/identity: `auth.UserFromContext(r.Context())` is non-nil inside the `RequireUser` group.
- Existing e2e deploy test: `e2e/deploy_test.go` (uses the in-process deployer). Use its server-boot pattern for the new e2e test.

Upload protocol (decided — implement exactly this; the browser client in plan 010 matches it):

- `POST /api/v1/deploy`, `multipart/form-data`, fields:
  - `site` (required): site name.
  - `confirm_overwrite` (optional, `"true"`): required when the site already exists — see step 4.
  - EITHER one part named `zip` (a `.zip` of the site) OR repeated parts named `files`, where each part's **filename is the file's site-relative path** (e.g. `assets/app.js` — the browser sets this via `formData.append('files', file, relPath)`).
- Success `200`: `{"site": "...", "url": "https://...", "deploy_id": "...", "file_count": N, "total_bytes": N, "warnings": ["..."]}`.
- Site exists and `confirm_overwrite` missing → `409` with `{"error": "...", "exists": true, "last_deployed_by": "...", "owner": "..."}`.
- Source project detected → `422` with the guidance message (step 5).

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
- `internal/server/deploy_api.go` (create — the handler)
- `internal/server/deploy_api_test.go` (create)
- `internal/server/server.go` (route registration + a `deployLimiter`)
- `e2e/http_deploy_test.go` (create, optional but preferred)

**Out of scope** (do NOT touch):
- `internal/sites/deploy.go` — the Deployer is reused as-is. If you believe it needs changes, STOP and report.
- `internal/cli/deploy.go` — switching the CLI to this endpoint is a recorded follow-up, not this plan.
- Any build/framework execution (npm, node, etc.) — hard product constraint.
- The home page UI — plan 010.
- MCP server.

## Git workflow

- Branch: `advisor/008-http-deploy-api`
- Commit style: `feat: HTTP deploy endpoint (multipart files or zip)`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Handler skeleton + staging

Create `internal/server/deploy_api.go` with `func (s *Server) handleDeploy(w http.ResponseWriter, r *http.Request)`:

1. `maxBytes := int64(s.cfg.Governance.Quotas.SiteMaxMB)*1024*1024 + (8<<20)`; `r.Body = http.MaxBytesReader(w, r.Body, maxBytes)`.
2. `if err := r.ParseMultipartForm(32 << 20); err != nil { writeError(w, "Upload too large or malformed multipart form.", http.StatusBadRequest); return }`.
3. `site := r.FormValue("site")`; reject empty with 400. (Deployer re-validates the name format.)
4. `tmp, err := os.MkdirTemp("", "artifact-deploy-*")` + `defer os.RemoveAll(tmp)`.

### Step 2: Stage `files` parts safely

For `r.MultipartForm.File["files"]`: each `*multipart.FileHeader`'s `.Filename` is the relative path. Sanitize each:

```go
rel := filepath.ToSlash(filepath.Clean(strings.ReplaceAll(fh.Filename, "\\", "/")))
if rel == "" || rel == "." || strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "..") || strings.Contains(rel, "../") {
	writeError(w, fmt.Sprintf("Invalid file path %q in upload.", fh.Filename), http.StatusBadRequest); return
}
```

Cap file count at 10,000 (413 beyond). Write each to `filepath.Join(tmp, filepath.FromSlash(rel))` after `os.MkdirAll` of its parent; copy with `io.Copy`. Never create symlinks (you're writing regular files, so this holds by construction).

### Step 3: Stage `zip` part safely

If a part named `zip` exists (and no `files` parts — if both, 400): copy it to a temp file, open with `zip.NewReader`. For each entry: skip directories; apply the same path sanitization as step 2 to `f.Name`; **reject non-regular entries** (`f.Mode()&os.ModeSymlink != 0` → 400 "zip contains symlinks"); enforce cumulative extracted size ≤ the `SiteMaxMB` quota *during* extraction (copy through a remaining-budget `io.LimitReader` and error when exceeded — zip bombs must not fill the disk); cap entry count at 10,000. Extract into `tmp`.

### Step 4: Existence check / overwrite confirmation

Before deploying: `existing, _ := s.db.GetSite(r.Context(), site)`. If `existing != nil` and `r.FormValue("confirm_overwrite") != "true"`, respond 409 with the JSON shape from Current state (`writeJSON` after `w.WriteHeader(http.StatusConflict)`). Governance still runs inside `Deploy` — this 409 is UX, not authorization.

### Step 5: Static-site checks + root-page handling

Inspect the staged tree before deploying:

- **Source project rejection**: if `tmp/package.json` exists AND `tmp/index.html` does not → 422: `"This looks like a source project (package.json found). Artifact hosts static files — run your build locally (e.g. npm run build) and drop the output folder (dist/, build/, or out/)."` Same for `next.config.js|mjs|ts` or `vite.config.js|ts` at root without index.html.
- **Single-root unwrap**: if `tmp` contains exactly one entry and it's a directory (the classic zipped-folder shape), treat that directory as the site root.
- **Root page promotion**: if no `index.html` at the chosen root and exactly one `*.html` file exists there, copy it to `index.html` and append to `warnings`: `"No index.html found — <name>.html will load at your site's root."` If no html files at all → 422 `"No HTML files found. Drop a folder with an index.html."` If multiple html files and no index.html → deploy anyway with warning `"No index.html found — visitors will see a 404 at the root path."`

### Step 6: Deploy + respond

```go
u := auth.UserFromContext(r.Context())
url, err := s.deployer.Deploy(r.Context(), site, root, u)
```

Map errors: governance denial (message contains "governed mode") → 403; quota ("exceeds") → 413; invalid name → 400; otherwise 500 (match on the error strings from `internal/sites/deploy.go`; keep mappings in one small helper). Success → 200 with the JSON shape from Current state. Count files/bytes during staging for the response.

### Step 7: Route + rate limit

In `server.go`, next to the other limiters: `deployLimiter := ratelimit.New(0.2, 3)`. Inside the `RequireUser` group:

```go
r.With(ratelimit.Middleware(deployLimiter, rateKey)).Post("/api/v1/deploy", s.handleDeploy)
```

**Verify** (after steps 1–7): `go build ./...` → exit 0; `go vet ./...` → exit 0.

### Step 8: Tests

`internal/server/deploy_api_test.go` — build multipart bodies with `mime/multipart.Writer`; temp-SQLite + `storage.NewLocal(t.TempDir())` setup (cfg/db pattern: `internal/server/api_test.go:19-27`); construct `sites.NewDeployer(cfg, store, database, governance.New(cfg), sites.NewDeployCache(store, 16))` and a minimal `Server`. Cases:

1. `files` upload with `index.html` + `assets/app.js` → 200; URL non-empty; `store.Exists` confirms objects; site record owner = test user email.
2. zip upload of the same tree → 200.
3. zipped-folder shape (single top dir) → 200 (unwrap works).
4. Path traversal: `files` part named `../evil.html` → 400; zip entry `../../evil` → 400.
5. Existing site without `confirm_overwrite` → 409 with `"exists": true`; with it → 200.
6. `package.json` without index.html → 422.
7. No HTML at all → 422.
8. Single `about.html`, no index → 200 with promotion warning and `index.html` in storage.

Optional `e2e/http_deploy_test.go`: boot the server, POST a real multipart deploy, GET the site → 200 with the page body.

**Verify**: `go test ./... -count=1 -race` → all pass.

## Test plan

The 8 unit cases in step 8 (happy ×3, traversal ×2, overwrite, source-rejection, root-promotion) plus the optional e2e round-trip.

## Done criteria

- [ ] `go test ./... -count=1 -race` exits 0
- [ ] `grep -n "api/v1/deploy" internal/server/server.go` → one route inside the RequireUser group with its own limiter
- [ ] Traversal cases return 400 (tests prove it); no file is ever written outside the temp dir
- [ ] HTTP deploys attributed to the session user, not `dev@localhost` (test asserts owner)
- [ ] No build tooling invoked (`grep -rn "exec.Command" internal/server/` → no matches)
- [ ] Only in-scope files modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back (do not improvise) if:

- `Deployer.Deploy`'s signature differs from the excerpt or it no longer enforces governance/quota internally.
- You want to modify `internal/sites/deploy.go` — report why temp-dir staging doesn't work instead.
- Multipart filenames arrive basename-only from Go's parser (path separators stripped) — verify early with a quick test; if stripped, report (fallback protocol would be a `paths` JSON field — a design change).
- The 409/422 response shapes conflict with something plan 010 already implemented (if executed out of order).

## Maintenance notes

- Plan 010 (Drop UI) consumes this endpoint — field names (`site`, `files`, `zip`, `confirm_overwrite`) and response shapes are a contract.
- Recorded follow-ups: CLI client mode (`artifact deploy --remote` with a token), root-page *picker* (`root_page` field), per-user deploy quotas.
- Reviewers: scrutinize the zip extraction limits (entry count, cumulative size, symlink rejection) — the riskiest surface here.
