# Plan 009 — Design System: Geist-inspired tokens + Artifact page redesign

**Priority:** P1  **Effort:** M  **Depends on:** —  **Status:** DONE

---

## Problem

The Artifact home page (`home.html`) and admin console (`admin.html`) are functional but
visually inconsistent: each page carries its own hard-coded CSS with no shared tokens or
component language.  Adding new Artifact-owned pages (e.g. the Drop-to-Deploy modal from
plan 010) requires duplicating and re-drifting styles.  The existing look is also noticeably
rougher than what internal users expect from a 2026 platform.

---

## Goals

1. **Single source of truth for design tokens** — one CSS file (`/ui.css`) served from the
   binary defines all colors, typography, spacing, radii, shadows, and transitions.
2. **Geist-inspired visual language** — clean, neutral palette; Inter/system-ui font stack;
   4 pt spacing grid; consistent radius scale; subtle shadows.
3. **Redesigned home page** — uses `/ui.css` component classes; responsive grid; `#new-site`
   button shipped hidden for plan 010 to unhide.
4. **Redesigned admin console** — uses `/ui.css` component classes; stats strip, panel/table
   layout, badge variants.
5. **No user-site blast radius** — `/ui.css` is served on `*.artifact.corp` only (not
   injected into user-deployed sites).

---

## Non-goals

- CSS framework / build pipeline for user sites
- Dark mode (can be added later via `prefers-color-scheme`)
- Changes to the TypeScript SDK (`artifact.js`)

---

## Changes

### `internal/server/static/ui.css` (new)

Shared design system stylesheet, embedded via `//go:embed static/*` and served at `/ui.css`:

| Layer | Contents |
|-------|----------|
| **Tokens** | `--color-*`, `--font-*`, `--text-*`, `--space-*`, `--radius-*`, `--shadow-*`, `--transition-*` |
| **Reset** | Minimal box-sizing + font-smoothing reset |
| **Layout** | `.container`, `.container--wide` |
| **Navigation** | `.nav`, `.nav__logo`, `.nav__user` |
| **Page header** | `.page-header`, `.page-header__title`, `.page-header__sub` |
| **Card** | `.card`, `.card--interactive`, `.card__title`, `.card__meta` |
| **Grid** | `.grid`, `.grid--2`, `.grid--3`, `.grid--auto` |
| **Input** | `.input` |
| **Button** | `.btn`, `.btn--primary`, `.btn--secondary` |
| **Badge** | `.badge`, `.badge--neutral`, `.badge--success` |
| **Panel** | `.panel`, `.panel__header`, `.panel__title`, `.panel__body` |
| **Stat card** | `.stat-card`, `.stat-card__value`, `.stat-card__label` |
| **Table** | `.table` |
| **Code block** | `.codeblock` |
| **Empty state** | `.empty-state`, `.empty-state__icon`, `.empty-state__title`, etc. |
| **Search row** | `.search-row` |

### `internal/server/static/home.html` (updated)

- `<link rel="stylesheet" href="/ui.css">` replaces inline `<style>` block
- Nav bar with Artifact logo SVG and authenticated user name
- Page header with site count badge
- Responsive auto-fill card grid; each card shows initial, name, deployer, time-ago, size, subdomain URL
- `timeAgo()` helper replaces `toLocaleDateString()`
- `#new-site` button added but hidden (`display:none; aria-hidden="true"`) — plan 010 unhides it and wires the drop event

### `internal/server/static/admin.html` (updated)

- `<link rel="stylesheet" href="/ui.css">` replaces inline `<style>` block
- Nav with "Admin" badge
- Stats strip using `.stat-card` inside a `.panel`
- Quotas panel uses `.codeblock`
- Sites / AI Usage / Audit tables use `.table` inside `.panel`
- Audit action column uses `.badge--neutral`
- Empty rows rendered for tables with no data

### `internal/server/server.go` (updated)

Added `serveUICSS` handler and `/ui.css` route (no-auth, cache 1 h — same treatment as `/artifact.js`):

```go
r.Get("/ui.css", s.serveUICSS)
```

---

## How to test

```bash
# Start the server in dev mode
artifact serve

# Visit the home page at http://localhost:3000
# Visit the admin console at http://admin.localhost:3000

# Verify /ui.css is served with correct Content-Type
curl -I http://localhost:3000/ui.css
# → Content-Type: text/css

# Verify user-deployed sites are unaffected
curl -s http://mysite.localhost:3000/ | grep ui.css
# → (empty — no injection into user sites)
```

---

## Design token reference

```
Color — background
  --color-bg:             #ffffff
  --color-surface:        #fafafa
  --color-surface-hover:  #f5f5f5

Color — border
  --color-border:         #e5e5e5
  --color-border-subtle:  #f0f0f0

Color — text
  --color-text:           #111111
  --color-text-secondary: #666666
  --color-text-tertiary:  #999999

Color — accent
  --color-accent:         #000000
  --color-accent-hover:   #333333
  --color-accent-fg:      #ffffff

Typography
  --font-sans: Inter, system-ui, -apple-system, …
  --font-mono: JetBrains Mono, Fira Code, ui-monospace, …
  --text-xs   12px  --text-sm  13px  --text-base 14px
  --text-md   16px  --text-lg  18px  --text-xl   20px  --text-2xl 24px

Spacing (4-pt grid)
  --space-1 4px  --space-2 8px  --space-3 12px  --space-4 16px
  --space-6 24px --space-8 32px

Radii
  --radius-sm 4px  --radius-md 6px  --radius-lg 8px  --radius-xl 12px
```

---

## STOP conditions

- Do **not** inject `/ui.css` into user-site responses (static serving must remain unchanged)
- Do **not** add a build step — the file is plain CSS embedded directly
- Do **not** remove the `#new-site` button even if it's hidden; plan 010 depends on its presence
