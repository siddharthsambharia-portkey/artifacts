# Plan 010 — Drop-to-Deploy UI on the home page

**Priority:** P1  **Effort:** M  **Depends on:** 008, 009  **Status:** DONE

---

## Problem

Deploying a site requires running `artifact deploy` in a terminal, which means users need
the CLI installed and know the command.  The home page at the apex domain shows the site
list but offers no way to deploy anything.  The server has a `POST /api/v1/deploy`
endpoint (plan 008) and a shared design system (plan 009) — the missing piece is a UI that
lets any authenticated employee drop a folder and have it live in seconds.

---

## Goals

1. **Zero-CLI deployment**: drag a folder (or `.zip`) onto the home page → site is live.
2. **Progressive disclosure**: the UI only appears when the user actively triggers it (button
   or drag); the list view is unchanged for users who don't need to deploy.
3. **Good error handling**: show conflict (409), validation errors (400/422), progress, and
   a direct link to the live site on success.
4. **Design system aligned**: uses `/ui.css` component classes exclusively; no new tokens
   or ad-hoc styles beyond page-specific layout.

---

## Non-goals

- Replacing the CLI for scripted/CI deployments
- Server-side changes (deploy endpoint and server routes are plan 008's responsibility)
- Admin-only deploy; any authenticated user in trust mode can deploy

---

## Changes

### `internal/server/static/home.html` (updated)

All changes are client-side HTML + vanilla JS. No new build steps.

#### Interaction model

| Trigger | What happens |
|---|---|
| User drags any file/folder over the browser window | Full-page semi-opaque drop overlay appears |
| User releases (drop) | Overlay dismisses; "New Site" modal opens pre-loaded with the dropped files |
| User clicks "New Site" button in the header | Modal opens empty |
| User clicks inside modal's drop zone | Browser dialog to pick a folder (`webkitdirectory`) or a `.zip` |

#### Modal states

```
idle ──► files selected ──► [optional: name validation] ──► deploying
                                                              │
                         ┌──────────────────────────────────┘
                         ├──► 200 success → show link, refresh list, stay open
                         ├──► 409 conflict → show conflict warning + overwrite checkbox
                         │      └──► user checks box → deploy again with confirm_overwrite=true
                         └──► 400/422/5xx → show error, re-enable Deploy button
```

#### API contract used

```
POST /api/v1/deploy  (multipart/form-data)
  site              string   required
  files             file[]   one-or-more file parts (webkitRelativePath as filename)
  zip               file     mutually exclusive with files
  confirm_overwrite string   "true" — only sent on the second attempt after 409

200 → { site, url, deploy_id, file_count, total_bytes, warnings[] }
409 → { error, exists, last_deployed_by, owner }
400 → { error }   validation / malformed
422 → { error }   source project detected / no HTML files
413 → { error }   too large
```

#### Key implementation details

- **Upload progress**: `XMLHttpRequest` (not `fetch`) so `xhr.upload.progress` events give
  real byte-level feedback.  Server processing is heuristically allocated the last 10%.
- **Folder structure preservation**: files uploaded via `<input webkitdirectory>` or
  directory drag are appended to FormData using `file.webkitRelativePath` as the filename,
  so the server-side `sanitizeRelPath` sees `folder/path/to/file.html` and can strip the
  top-level wrapper folder as designed in plan 008.
- **Zip upload**: a single `.zip` part named `zip` is sent; server unpacks and validates.
- **Site name slug**: the `slugify()` helper converts the dropped folder/file name to a
  valid lowercase-hyphenated slug pre-filled into the site name input.  Users can edit it.
- **Conflict flow**: on 409 the deploy button is disabled; a checkbox appears asking the
  user to confirm.  Checking the box re-enables Deploy, which re-sends the same payload
  with `confirm_overwrite=true` appended.
- **Page-level drag counter**: `dragenter`/`dragleave` fire for child elements too; a depth
  counter prevents the overlay from flickering as the mouse passes over child nodes.
- **`#new-site` button**: plan 009 shipped this button hidden
  (`display:none; aria-hidden="true"`).  This plan removes the inline hide style and sets
  the button as always-visible now that the deploy UI exists to back it.

---

## How to test

```bash
artifact serve   # dev mode — SQLite + local storage + fake login

# 1. Open http://localhost:3000
# 2. Drag a folder of static HTML onto the page
#    → overlay appears, modal opens, files listed
# 3. Enter a site name → Deploy button enables
# 4. Click Deploy → progress bar fills → success banner + link
# 5. Click the link → site loads from local storage

# Conflict
# 6. Drop the same folder again, same site name → 409 conflict warning
# 7. Check "Yes, overwrite" → deploy succeeds

# Source project guard
# 8. Drop a folder with package.json but no index.html
#    → 422 error: "This looks like a source project…"

# Zip upload
# 9. Drop or pick a .zip file → zip part is sent, server unpacks

# Verify /ui.css is used (no inline fallback)
# 10. Open DevTools → Sources → home.html should have no <style>
#     block with color hex codes (only the design-system link and page-scoped layout CSS)
```

---

## STOP conditions

- Do not modify the deploy endpoint or any Go code (plan 008 owns that)
- Do not add a build step; the page remains a single embedded HTML file
- Do not inject the deploy UI into user-deployed sites
- The `#new-site` button must remain in the DOM even if JS fails (progressive enhancement)
