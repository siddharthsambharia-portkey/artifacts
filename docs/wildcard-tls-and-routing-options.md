# Routing & wildcard-TLS: options, trade-offs, and what we chose

> Status: **exploration / decision record** · Last updated: 2026-06-18
> Related: `docs/concepts.md`, `deploy/recipes/wildcard-tls-gcp.md`,
> `issues/008-wildcard-tls-and-cross-subdomain-sso.md`, ADR 0004 (cross-site imports).

This doc captures the design space around how Artifact addresses sites
(`my-site.<domain>` vs `<domain>/my-site`) and the operational cost that comes
with it (wildcard DNS + TLS). It exists because operators occasionally hit a wall:
their company won't let them obtain a `*.<domain>` certificate via the DNS-01 ACME
challenge, and they ask whether Artifact could be path-based instead.

The short version:

- **We chose subdomains, and we keep them.** The subdomain *is* the site name, and
  it is the only browser-enforced security boundary between sites.
- **The wildcard-TLS cost is real but almost always solvable at the infra layer**
  without changing the model — usually via an internal CA or CNAME-delegated DNS-01.
- **Path-based is a last resort.** It breaks the core "site is server-derived"
  invariant and is only viable in pure trust mode. We do not build it speculatively.

---

## 1. The core decision: subdomains vs paths

### Why subdomains

A browser's security boundary is the **origin** (scheme + host + port). Path is
invisible to it. So:

- `a.<domain>` and `b.<domain>` are **two origins** — isolated cookies, `localStorage`,
  `IndexedDB`, DOM, and `fetch` credentials.
- `<domain>/a` and `<domain>/b` are **one origin** — none of the above are isolated.

Artifact derives the site from the `Host` header server-side and never trusts the
client for it (`internal/config/config.go: SiteFromHost`). Every SDK subsystem
(db, kv, files, ai, notify, warehouse, realtime) keys off that. This is what lets
the platform expose a database / AI proxy / file storage to the browser with no API
keys: identity and site are stamped by the server, and sites cannot tamper with each
other because the browser keeps them in separate origins.

### Why path-based is hard (and unsafe)

| Browser primitive | Isolated by subdomain? | Isolated by path? |
|---|---|---|
| Cookies (incl. session) | Yes | **No** (cookie `Path` is not a security control) |
| `localStorage` / `IndexedDB` | Yes | **No** (per-origin) |
| DOM / `window` | Yes | **No** (same origin scripts across pages) |
| Who can call `/api/v1/*` | Yes | **No** |

The fatal problem: under one shared origin, the server **cannot honestly attribute an
API call to a site** — every site's JS is the same origin with the same cookies and
the same `Origin`/`Referer`. To route the call you'd have to trust a client-supplied
site value, which violates the load-bearing invariant ("site is server-derived, never
client-supplied"). That regresses per-site AI quota / notify / warehouse attribution
even in trust mode, and is outright broken in **governed mode** (a private site and a
general site sharing an origin means one can read the other's session-authorized data).

**Precedent:** Shopify's "Quick" (the system Artifact is modeled on) chose subdomains
via a wildcard NGINX config — primarily for implementation simplicity and web-native
ergonomics. Notably Quick runs *pure trust mode* with no owners, so it didn't strictly
need origin isolation; it got it for free. Artifact adds governed mode, which turns
that free bonus into a hard requirement.

---

## 2. The cost this creates: wildcard DNS + TLS

Subdomain-per-site needs two things at the infra layer (the **app code does not care
how TLS is terminated** — it only reads `Host`):

1. **A wildcard DNS record** — `*.<domain> → load balancer`. One-time, low-risk, static.
2. **A TLS cert covering the subdomains** — typically a wildcard `*.<domain>` cert.

The usual pain is the cert. The documented path uses ACME **DNS-01** (HTTP-01 *cannot*
issue wildcards, per RFC 8555). DNS-01 requires giving cert-manager API credentials to
write `_acme-challenge` TXT records in the DNS zone — and that is exactly the permission
many corporate IT teams refuse.

> **Untangle the two asks.** "I can't do wildcard DNS" usually means *"I can't get
> DNS-01 credentials"* (#2), **not** *"I can't add a `*` record"* (#1). They have very
> different fixes. Always confirm which wall the operator actually hit.

---

## 3. Options for getting the cert WITHOUT DNS-01 (all keep subdomains)

These preserve the security boundary. Listed best-first.

### Option A — Use the company's internal CA  ✅ usually best
Artifact is internal-only by design, so a private CA is the natural fit. Most orgs that
block DNS-01 have one (Vault PKI, AD CS, Venafi, step-ca). Issue a `*.<domain>` cert
directly, drop it in the ingress TLS secret. No ACME, no DNS challenge, no Let's Encrypt.

- **Pros:** No DNS-API access needed; fits the internal-only model; simplest operationally.
- **Cons:** Requires an internal CA and a (often manual) renewal process; the root must
  be trusted by client devices (usually already true via MDM).

### Option B — DNS-01 via CNAME delegation (`acme-dns`)
Keep cert-manager/Let's Encrypt, but IT only adds **one static CNAME**:
`_acme-challenge.<domain> → <id>.acme-dns.<domain>` pointing at a tiny acme-dns server
you control. cert-manager then solves the challenge against *your* server, not their zone.

- **Pros:** Converts "give me DNS API keys" (refused) into "add one CNAME once" (usually
  fine); keeps automated ACME renewal.
- **Cons:** One more small component (acme-dns) to run; initial setup understanding.

### Option C — Reuse an existing org wildcard / terminate TLS upstream
Artifact already sits behind an identity proxy / LB (the trust bubble requires it). If the
org already has a `*.<domain>` cert, or the cloud LB can mint a managed wildcard
(GCP-managed certs, AWS ACM), terminate TLS there and send plain HTTP to Artifact.

- **Pros:** Zero new cert machinery; reuses existing, already-approved infra.
- **Cons:** Depends on an existing wildcard or a managed-cert-capable LB; TLS lives outside
  the Helm chart.

### Option D — Per-host certs instead of a wildcard
You don't strictly need `*` — you need each subdomain covered. cert-manager can issue a
per-site cert as sites are created.

- **Pros:** No wildcard cert at all; subdomains preserved.
- **Cons:** Needs an **internal ACME CA** (public HTTP-01 can't reach an internal-only host);
  only scales to tens/low-hundreds of sites; cert churn as sites come and go.

### Option E — Single-host (right-size the tool)
If the need is one tool, not a platform, deploy to a single subdomain and use one ordinary
single-host cert from the normal corporate process. No wildcard anywhere.

- **Pros:** Trivial; no wildcard DNS or cert.
- **Cons:** Not a multi-site platform; each new tool is a new manual hostname + cert.

---

## 4. The path-based option (documented for completeness — not recommended)

Build a mode where sites live at `<domain>/<site>/` instead of `<site>.<domain>`.

- **Pros:** No wildcard DNS, no wildcard cert — a single hostname/cert serves everything.
- **Cons:**
  - **Breaks the core invariant.** Under one origin the server can't attribute
    `/api/v1/*` calls to a site without trusting a client-sent value.
  - **Unsafe in governed mode** (no origin isolation between private/public sites);
    only tolerable in pure trust mode.
  - **Expensive.** Touches every `SiteFromHost` caller (db, kv, files, ai, notify,
    warehouse, realtime, static handler), the SDK's site derivation, session/cookie
    handling, and the cross-site CORS model — weeks of security-sensitive work.
  - Contradicts the project's "say no until it's real" discipline (cf. custom backends,
    cron, hosted MCP in `docs/faq.md`).
- **If we ever build it:** gate it to `governance.mode: trust` only, refuse to start in
  governed mode, and document it explicitly as a security downgrade.

---

## 5. Comparison at a glance

| Option | Keeps subdomains? | Needs DNS-01? | Needs wildcard cert? | Governed-mode safe? | Effort | Verdict |
|---|---|---|---|---|---|---|
| A. Internal CA | Yes | No | Yes (from CA) | Yes | Low | **Preferred** |
| B. acme-dns delegation | Yes | "1 CNAME" only | Yes | Yes | Low–Med | **Strong** |
| C. Reuse wildcard / upstream TLS | Yes | No | Reuses existing | Yes | Low | Strong (if available) |
| D. Per-host certs | Yes | No (needs internal ACME) | No | Yes | Med | Situational (small scale) |
| E. Single-host | N/A (1 site) | No | No | Yes | Very low | Good for small needs |
| F. Path-based mode | **No** | No | No | **No** | High | **Avoid** unless recurring demand |

---

## 6. Decision

- **Keep subdomains.** The origin boundary is load-bearing, especially for governed mode.
- **Unblock cert-constrained operators at the infra layer**, in priority order:
  internal CA (A) → acme-dns delegation (B) → reuse/upstream TLS (C) → per-host (D) →
  single-host (E) for small needs.
- **Defer path-based (F).** Treat it as a "no until it's real": build only if multiple
  operators are permanently stuck in pure trust mode AND none of A–E can satisfy them.
  Even then, ship it trust-mode-only and clearly labeled as a downgrade.

### Follow-ups worth doing regardless
- Document options A–E in `deploy/recipes/` (the current recipe only covers GCP DNS-01).
- Add an internal-CA recipe (drop a pre-issued wildcard into the ingress TLS secret).
- Add an acme-dns / CNAME-delegation recipe.
- Make `docs/self-hosting.md` point operators at this menu before assuming DNS-01.
