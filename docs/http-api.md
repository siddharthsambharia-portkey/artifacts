# HTTP API

Endpoints beyond the browser SDK. All requests must come from an authenticated
employee (same session cookie the SDK uses); responses are JSON.

## POST /api/v1/deploy

Deploy a site from an HTTP client (the browser Drop-to-Deploy UI uses this; the
CLI does not yet). Rate limited.

**Request** — `multipart/form-data`:

| Field | Required | Notes |
|-------|----------|-------|
| `site` | yes | Target site name (subdomain). |
| `files` | one of | One or more file parts. The part filename carries the relative path, so nested folders are preserved. |
| `zip` | one of | A single `.zip` archive. Send **either** `files` **or** `zip`, never both. |
| `confirm_overwrite` | no | `"true"` to redeploy over an existing site. |

Limits: at most 10,000 files; total size is capped at the `site_max_mb` quota
(plus a small multipart overhead). A single top-level directory is unwrapped
automatically; if there is no `index.html`, Artifact either promotes the only
HTML file or returns a warning.

**200 OK** — deployed:

```json
{
  "site": "my-site",
  "url": "https://my-site.artifact.corp.com",
  "deploy_id": "20260613...",
  "file_count": 12,
  "total_bytes": 48213,
  "warnings": []
}
```

**409 Conflict** — site exists and `confirm_overwrite` was not `"true"`:

```json
{
  "error": "Site \"my-site\" already exists.",
  "exists": true,
  "last_deployed_by": "alice@corp.com",
  "owner": "alice@corp.com"
}
```

**422 Unprocessable Entity** — the upload isn't deployable static output, e.g. a
source project (`package.json` present) or no HTML at all:

```json
{ "error": "This looks like a source project (package.json found). Artifact hosts static files — run your build locally (e.g. npm run build) and drop the output folder (dist/, build/, or out/)." }
```

Other error responses share the `{ "error": "..." }` shape: **400** (missing
`site`, both/neither of `files`/`zip`, bad path, invalid zip), **403** (governed
mode ownership conflict), **413** (too many files or over quota).
