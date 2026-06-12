# Plan 002: Make expired sessions fail closed — return an error instead of a nil user

> **Executor instructions**: Follow this plan step by step. Run every
> verification command and confirm the expected result before moving to the
> next step. If anything in the "STOP conditions" section occurs, stop and
> report — do not improvise. When done, update the status row for this plan
> in `plans/README.md` — unless a reviewer dispatched you and told you they
> maintain the index.
>
> **Drift check (run first)**: `git diff --stat f1702de..HEAD -- internal/auth/session.go internal/auth/oidc.go internal/auth/session_test.go`
> If any in-scope file changed since this plan was written, compare the
> "Current state" excerpts against the live code before proceeding; on a
> mismatch, treat it as a STOP condition.

## Status

- **Priority**: P1
- **Effort**: S
- **Risk**: LOW
- **Depends on**: plans/001-test-harness-and-characterization-tests.md (the characterization test it flips; can run before 001 if you also add the test yourself)
- **Category**: bug
- **Planned at**: commit `f1702de`, 2026-06-12

## Why this matters

When an OIDC session expires (24h TTL), `SessionStore.Get` deletes the row but then returns `(nil, nil)` — a nil user with a nil error. The caller `lookupSession` treats `err == nil` as "session valid", so the request proceeds with a **nil user in the request context** instead of redirecting to `/login`. Concretely: after 24 hours, every API call returns 401 (from `RequireUser`) while the browser still holds a "valid" cookie and is never redirected to re-authenticate — the user is stuck until the cookie expires. Worse, routes outside `RequireUser` that dereference the user panic: `serveAdmin` calls `IsAdmin(u)` which ranges over `user.Groups` on a nil pointer (500 via Recoverer). One line fixes it.

## Current state

- `internal/auth/session.go:29-42` — the bug. `err` at line 39 is the nil error from the successful `Scan`:

```go
func (s *SessionStore) Get(ctx context.Context, id string) (*User, error) {
	var email, name, groupsRaw string
	var expires time.Time
	err := s.db.QueryRowContext(ctx,
		`SELECT email, name, groups_json, expires_at FROM sessions WHERE id=?`, id).Scan(&email, &name, &groupsRaw, &expires)
	if err != nil {
		return nil, err
	}
	if time.Now().After(expires) {
		s.db.ExecContext(ctx, `DELETE FROM sessions WHERE id=?`, id)
		return nil, err            // <-- BUG: err is nil here
	}
	return &User{Email: email, Name: name, Groups: parseGroups(groupsRaw)}, nil
}
```

- `internal/auth/oidc.go:91-94` — the caller that converts `err == nil` into "logged in":

```go
func (o *OIDCAuthenticator) lookupSession(ctx context.Context, id string) (*User, bool) {
	if o.useDB && o.sessions != nil {
		u, err := o.sessions.Get(ctx, id)
		return u, err == nil
	}
```

- `internal/auth/oidc.go:80-85` — on `ok == false` the middleware correctly clears the cookie and redirects to `/login`. That is the behavior we want for expired sessions.

- Error-message convention in this package (see oidc.go:40): `fmt.Errorf` with a plain lowercase message.

## Commands you will need

| Purpose   | Command                  | Expected on success |
|-----------|--------------------------|---------------------|
| Build     | `go build ./...`         | exit 0              |
| Tests     | `go test ./internal/auth/ -count=1 -v` | all pass |
| Full      | `go test ./... -count=1 -race` | all pass      |

## Scope

**In scope**:
- `internal/auth/session.go` (the one-line fix; add `"fmt"` to imports)
- `internal/auth/session_test.go` (flip/add the expired-session assertion)

**Out of scope** (do NOT touch):
- `internal/auth/oidc.go` — `lookupSession`'s `err == nil` contract is correct once Get returns a real error.
- The in-memory session path (`memSessions` in oidc.go) — it already handles expiry correctly (oidc.go:99).
- Session TTL, cookie attributes, or any other auth behavior.

## Git workflow

- Branch: `advisor/002-expired-session-fix`
- Commit style: `fix: return error for expired sessions instead of nil user`
- Do NOT push or open a PR unless the operator instructed it.

## Steps

### Step 1: Fix the return

In `internal/auth/session.go`, change line 39 from `return nil, err` to:

```go
return nil, fmt.Errorf("session expired")
```

Add `"fmt"` to the import block.

**Verify**: `go build ./...` → exit 0.

### Step 2: Update the test

In `internal/auth/session_test.go` (created by plan 001 — if it doesn't exist, write this single test now using the temp-SQLite setup from `internal/server/api_test.go:19-27`): the expired-session case (`Create` with negative TTL, then `Get`) must now assert `err != nil` and `u == nil`. Remove the `// BUG:` comment.

**Verify**: `go test ./internal/auth/ -count=1 -v` → PASS, including the expired-session subtest.

## Test plan

Covered by step 2: one subtest asserting expired sessions return a non-nil error; existing valid-session and unknown-id subtests still pass. `go test ./... -count=1 -race` → all pass.

## Done criteria

- [ ] `go build ./...` exits 0
- [ ] `go test ./... -count=1 -race` exits 0
- [ ] `internal/auth/session.go:39` no longer returns a nil error on the expired path (`grep -n "session expired" internal/auth/session.go` → one match)
- [ ] Only the two in-scope files are modified (`git status`)
- [ ] `plans/README.md` status row updated

## STOP conditions

Stop and report back if:

- `session.go:29-42` doesn't match the excerpt above (drift).
- Fixing the test reveals `lookupSession` callers that *depend* on the nil-user-no-error behavior (grep `sessions.Get` — there should be exactly one caller, in oidc.go).

## Maintenance notes

- After this lands, an expired session redirects to `/login` like a missing cookie — verify manually if possible: set a session row's `expires_at` to the past in the SQLite db and reload a page.
- Follow-up deferred: there is no periodic cleanup of expired session rows (they're only deleted when the expired session is presented). Harmless growth; worth a sweep job eventually.
