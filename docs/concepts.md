# Concepts

Artifact is small on purpose. Five ideas explain the whole system.

## 1. A site is a folder

A *site* is a folder of static files (HTML, CSS, JS, images). You deploy it and it becomes
available at `https://<site>.<domain>` — the subdomain *is* the site name. There are no
frameworks, no build step, and no server-side code you write. The subdomain maps to a prefix
in object storage (`sites/<site>/`). The apex domain serves the home directory and
`/artifact.js`. `admin.<domain>` serves the admin console.

Deploys are **atomic**: files are uploaded, a manifest is written, then a pointer is flipped.
A half-uploaded site is never served, and overwriting a site is how you update it.

## 2. The trust bubble

This is the load-bearing idea. Normally, exposing a database, an AI proxy, and file storage
directly to the browser with no API keys would be reckless. Inside a company it is safe.
Every request reaching Artifact has already been authenticated as an employee — by
Artifact's own OIDC login or by an identity proxy in front of it.

That single guarantee — "every caller is a trusted employee" — lets Artifact eliminate the
hard 90% of a backend platform. There are no API keys in client code, no per-site auth, no
signup, and no multi-tenant isolation. The browser SDK calls `/api/v1/*` with the session
cookie, and the server stamps identity and the site name from the request itself.

The trade-off is explicit: **Artifact is for internal apps, never the public internet.** See
[the FAQ](faq.md) for when this is and isn't a fit.

## 3. Identity and site are server-derived, never client-supplied

Two values are never trusted from the client:

- **Who you are** (`artifact.me`) comes from your session, stamped by the server on every
  write as `created_by` / `updated_by`.
- **Which site you are** comes from the request `Host` (the subdomain). The SDK never sends a
  `site` field on writes; the server derives it.

Your site contains no auth code. A site cannot impersonate a user or write data as another
site. (Cross-site *reads* are a separate, deliberate feature — see below.)

## 4. The zero-config browser API

Loading `<script src="/artifact.js"></script>` gives every site the same backend:

| SDK | Purpose | Backed by |
|---|---|---|
| `artifact.me` | The signed-in employee | session / SSO |
| `artifact.db` | Document collections + realtime subscribe | Postgres or SQLite |
| `artifact.kv` | Per-site key/value | Postgres or SQLite |
| `artifact.files` | Uploads with served URLs | object storage |
| `artifact.ai` | Streaming chat + image generation | your AI gateway (keys server-side) |
| `artifact.warehouse` | Read-only SQL | BigQuery / Snowflake / Postgres |
| `artifact.ws` | Rooms, messages, presence | in-process hub (NATS optional) |
| `artifact.notify` | Slack messages | server-held webhook/token |

Full method-by-method details are in the [SDK reference](sdk-reference.md).

### Cross-site reads and the shared-library idea

In trust mode, a site can read another site's data:
`artifact.db.collection('posts', { site: 'team-blog' })`. This lets sites compose into a
small internal ecosystem (a dashboard reading another tool's collection, shared script
imports). Cross-site **writes** are not allowed by default. In governed mode, cross-site
reads are subject to the target site's visibility.

## 5. Trust mode vs governed mode

Governance is a single toggle (`governance.mode`):

- **Trust mode** (default): the quick experience. All sites are open to all employees, there
  are no owners, and anyone can overwrite anything. The audit log still records every deploy
  and destructive call.
- **Governed mode**: the first deployer owns the site, visibility can be scoped to groups,
  deletion is restricted, and the admin console exposes audit search, quotas, and usage.

Governance is middleware over nullable columns. Trust mode is governed mode with every check
returning "allow." See [Governance & admin](governance-and-admin.md).

## Hard non-goals

These are refused on purpose; the constraints are the product:

- No per-site custom backends or server-side functions
- No cron / scheduled jobs
- No build pipelines
- No per-site environment variables or secrets
- No public-internet exposure of sites

If you need these, Artifact is the wrong tool — and that is by design. See the
[FAQ](faq.md).
