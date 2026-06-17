# Governance & admin

Governance is a single toggle that determines how much control is placed over who can deploy,
read, and delete sites. The audit log records everything regardless of mode, and the admin
console is always present — but most of its power only matters in governed mode.

## Trust mode vs governed mode

```yaml
# artifact.yaml
governance:
  mode: trust      # trust (default) | governed
```

| | Trust mode | Governed mode |
|---|---|---|
| Site ownership | None — any employee can deploy or overwrite any site | The first deployer owns the site |
| Redeploy restriction | Anyone can overwrite any site | Only the owner or admins can redeploy |
| DB write restriction | Anyone can write to any site's collections | Only the owner or admins can write |
| Site visibility | All sites visible to all employees | Configurable: private, group-scoped, or public |
| Deletion | No restrictions | Restricted to owner or admins |
| Audit log | Recorded | Recorded |
| Admin console | Present; requires membership in the `admins` group (see Admin console below) | Present and useful |

**Trust mode** is the quick-start experience. It is the right choice for small teams where
everyone knows everyone and you want zero friction. It is not suitable for large organizations
where you need to prevent accidental overwrites or enforce data visibility.

**Governed mode** adds ownership and access controls without changing the SDK or the site
development experience.

### Enabling governed mode

Set the toggle in `artifact.yaml`:

```yaml
governance:
  mode: governed
```

Or use the environment variable override (useful for rolling out without a config deploy):

```bash
ARTIFACT_GOVERNANCE_MODE=governed
```

The environment variable takes precedence over `artifact.yaml`. See `.env.example` for all
overridable variables.

## What governed mode adds

### First-deployer ownership

When a site is deployed for the first time in governed mode, the deploying user's email is
recorded as the site owner. Subsequent deploys by other users are rejected with an error
explaining who owns the site. Admins can always redeploy.

### Group-scoped visibility

Each site has a `visibility` setting that controls who can read it:

| Visibility | Who can access |
|---|---|
| `private` | Site owner and admins only |
| `group` | Members of specified groups, plus owner and admins |
| `public` | All authenticated employees |

In trust mode every site behaves as `public`. In governed mode the default for new sites is
`public`, but admins can change it via the admin console.

Group membership comes from:
- **OIDC mode** — the `groups` claim in the user's ID token, configured via `oidc.groups_claim`.
- **Header-trust mode** — the `groups_header` forwarded by the identity proxy (default:
  `X-Auth-Request-Groups`). See [Auth — header-trust](auth-header-trust.md) for proxy-specific setup.

### Restricted deletion

In governed mode, deleting or overwriting a site is restricted to the site owner and admins.

## Quotas

Quotas are enforced in both trust and governed mode. Set them under `governance.quotas`:

```yaml
governance:
  mode: trust
  quotas:
    site_max_mb: 500                    # max total size of a single deployed site
    db_max_docs_per_site: 100000        # max documents per site across all collections
    upload_max_mb: 50                   # max size of a single file upload
    ai_daily_calls_per_user: 0          # max AI calls (chat + image) per user per day; 0 = unlimited
    warehouse_daily_queries_per_user: 200  # max warehouse queries per user per day; 0 = unlimited
```

| Field | Default | What it enforces |
|---|---|---|
| `site_max_mb` | `500` | Maximum total size of a deployed site. Enforced at deploy time. |
| `db_max_docs_per_site` | `100000` | Maximum number of documents across all collections for a single site. |
| `upload_max_mb` | `50` | Maximum size of a single file uploaded via `artifact.files`. |
| `ai_daily_calls_per_user` | `0` | Maximum AI requests (chat + image combined) per user per rolling 24-hour window. `0` disables the quota. |
| `warehouse_daily_queries_per_user` | `200` | Maximum warehouse queries per user per rolling 24-hour window. `0` disables the quota. |

Quotas apply regardless of governance mode. The AI and warehouse quotas produce 429 responses
when exceeded; upload and DB quotas produce 413/422.

## Audit log

Every deploy, destructive action, and warehouse query is recorded in the audit log,
regardless of governance mode. The log is written to the same database as the rest of Artifact
data.

Recorded actions include:

| Action | When recorded |
|---|---|
| `deploy` | Site deployed or updated |
| `delete` | Site deleted |
| `warehouse_query` | Warehouse query executed (first 200 chars of SQL) |
| `visibility_change` | Site visibility changed by an admin |

The audit log is append-only. Admins can query it via the admin console (see below).

## Admin console

The admin console lives at `admin.<domain>` (e.g. `https://admin.artifact.corp.example.com`).
It is gated to users in the `admins` group — a user who is not in `admins` receives 403.

Admin gating checks the `admins` group from the user's groups. In OIDC mode, groups come from
the `oidc.groups_claim` JWT claim. In header-trust mode, groups come from the `groups_header`
forwarded by the identity proxy — configure your proxy to include `admins` in that header for
users who should have admin access. See [Auth — header-trust](auth-header-trust.md) for
proxy-specific setup.

### Admin API endpoints

All admin endpoints are under `/api/v1/admin/` and require the user to be in the `admins`
group. All routes are served from the same Artifact binary on the admin host.

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/v1/admin/audit` | Search the audit log. Query params: `site=<name>`, `user=<email>`, `limit=<n>` (default 100). |
| `GET` | `/api/v1/admin/usage` | AI usage summary — top users by call count over the last 100 records. |
| `GET` | `/api/v1/admin/config` | Current server configuration snapshot: governance mode, quotas, domain, auth mode, storage driver. |
| `GET` | `/api/v1/admin/stats` | Aggregate stats: total site count, total bytes stored, governance mode. |
| `PUT` | `/api/v1/admin/sites/{site}/visibility` | Change a site's visibility. Body: `{"visibility":"private"|"group"|"public","groups":["eng","design"]}`. `groups` is required when `visibility` is `group`. Records an audit entry. |

#### Example: search the audit log

```bash
curl -s "https://admin.artifact.corp.example.com/api/v1/admin/audit?site=team-blog&limit=20" | jq .
```

#### Example: change site visibility

```bash
curl -X PUT \
  "https://admin.artifact.corp.example.com/api/v1/admin/sites/team-blog/visibility" \
  -H "Content-Type: application/json" \
  -d '{"visibility":"group","groups":["eng","design"]}'
```

The `PUT /visibility` call records a `visibility_change` entry in the audit log with the
new visibility and groups.

## Choose a governance mode

| Your situation | Recommended mode |
|---|---|
| Small team, high trust, want zero friction | Trust |
| Multiple teams, need to prevent accidental overwrites | Governed |
| Need per-group data visibility | Governed + OIDC or header-trust with groups header |
| Need admin console access | Any auth mode with `admins` group forwarded to Artifact |
| Production deployment at a large company | Governed + OIDC auth |

Even in trust mode, the audit log gives you a full history of deploys and destructive actions
— the difference is that nothing is *blocked*.
