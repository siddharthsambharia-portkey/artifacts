# Auth — header-trust (IAP / Pomerium / oauth2-proxy / ZTNA)

In header-trust mode, an identity proxy — Google IAP, AWS ALB + OIDC, Pomerium, oauth2-proxy,
or any ZTNA gateway — sits in front of Artifact and authenticates the user. Artifact trusts
the identity the proxy forwards in HTTP headers instead of running its own OIDC flow.

Use this mode when you have an existing identity proxy and want Artifact inside that trust
boundary without registering a second OIDC application.

## Network model

```
employees (internal network or VPN)
        │
        ▼
identity proxy (Google IAP / Pomerium / oauth2-proxy / ZTNA)
  ├─ authenticates the user
  ├─ forwards X-Auth-Request-Email (and X-Artifact-Proxy-Auth)
  └─ wildcard route: *.artifact.corp.example.com → http://artifact:8443
        │
        ▼
Artifact (listens on :8443, tls: off)
  ├─ *.artifact.corp.example.com  → site serving
  └─ admin.artifact.corp.example.com → admin console
```

Key properties:

- Wildcard DNS `*.<domain>` points to the proxy, not directly to Artifact.
- `admin.<domain>` is routed the same way and is gated to the `admins` group (see
  [Governance & admin](governance-and-admin.md)).
- TLS terminates at the proxy or load balancer; set `tls: { mode: off }` in Artifact.
- Artifact is **never** exposed to the public internet — only the proxy is.

## Configuration

```yaml
# artifact.yaml
auth:
  mode: header-trust
  header_trust:
    email_header: X-Auth-Request-Email    # default; header the proxy sends with the user's email
    name_header: X-Auth-Request-User      # default; header with the user's display name (optional)
    proxy_secret_env: ARTIFACT_PROXY_SECRET  # name of the env var holding the shared secret
```

| Field | Default | What it does |
|---|---|---|
| `email_header` | `X-Auth-Request-Email` | Header the proxy sends with the authenticated user's email. Required. |
| `name_header` | `X-Auth-Request-User` | Header with the user's display name. If missing or empty, Artifact falls back to the local-part of the email. |
| `proxy_secret_env` | *(none)* | Name of the environment variable that holds the shared secret. **Must be set.** |

The actual secret value goes in the environment variable named by `proxy_secret_env`, not
directly in the config. The `.env.example` at the repo root uses `ARTIFACT_PROXY_SECRET`:

```bash
ARTIFACT_PROXY_SECRET=changeme-proxy-shared-secret
```

The proxy must send this value in the `X-Artifact-Proxy-Auth` request header on every
request it forwards to Artifact.

## Hard-fail boot rule

Artifact **refuses to start** in header-trust mode unless both conditions hold:

1. `proxy_secret_env` is present in the config (i.e. the field is non-empty).
2. The environment variable it names is set to a non-empty value at boot time.

From `config.Validate()`:

```
if proxy_secret_env == ""                    → error: refuses to boot (field missing)
if os.Getenv(proxy_secret_env) == ""         → error: refuses to boot (env var not set)
```

**Why this is enforced at boot rather than at runtime:** without a shared secret any process
that can reach Artifact's port could forge the identity headers and authenticate as any user.
The proxy creates the trust bubble; Artifact protects that bubble with the secret. Skipping
it would mean the entire trust model relies on network isolation alone, which is not
sufficient if any other workload shares the same network segment.

If Artifact fails to start with a message like:

```
header-trust mode requires ARTIFACT_PROXY_SECRET to be set — configure your identity proxy shared secret
```

set the environment variable and restart. If the error is:

```
header-trust mode requires proxy_secret_env in config — Artifact refuses to boot without proxy authentication
```

add `proxy_secret_env: ARTIFACT_PROXY_SECRET` (or your chosen name) to `artifact.yaml`.

## Current limitation — groups are hardcoded to `["employees"]`

**Current behavior:** in header-trust mode, every authenticated user is assigned the group
`employees` regardless of what the identity proxy knows about group membership. There is no
way to pass groups via a header today.

Practical consequences:

- No user will be in the `admins` group, so the admin console at `admin.<domain>` will be
  inaccessible (all requests return 403).
- Governed-mode group-scoped visibility (`visibility: group`) cannot be used — no user will
  match a non-employee group.

**If you need admin access or group-scoped visibility**, use OIDC mode instead (see
[Auth — Okta](auth-okta.md), [Auth — Entra ID](auth-entra.md), or
[Auth — Google Workspace](auth-google.md)). OIDC mode reads group membership from the ID
token's `groups_claim`, which makes admin gating work correctly.

This limitation is tracked in the backlog and will be resolved when a configurable
groups header is added to the header-trust authenticator.

## Proxy-specific setup

### Pomerium

See the full recipe at [`deploy/recipes/pomerium.md`](../deploy/recipes/pomerium.md).

The short version — Pomerium forwards identity via `X-Pomerium-Claim-*` headers:

```yaml
# artifact.yaml
auth:
  mode: header-trust
  header_trust:
    email_header: X-Pomerium-Claim-Email
    name_header: X-Pomerium-Claim-Name
    proxy_secret_env: ARTIFACT_PROXY_SECRET
```

Configure Pomerium to send `X-Artifact-Proxy-Auth: <secret>` on every forwarded request and
set `ARTIFACT_PROXY_SECRET` to the same value in Artifact's environment.

### oauth2-proxy

oauth2-proxy forwards user identity in `X-Auth-Request-Email` and `X-Auth-Request-User` by
default, which match Artifact's defaults. The only required config change is adding the
proxy-auth header:

```yaml
# artifact.yaml — oauth2-proxy defaults already match; just add:
auth:
  mode: header-trust
  header_trust:
    proxy_secret_env: ARTIFACT_PROXY_SECRET
```

In `oauth2-proxy.cfg`, inject the shared secret:

```ini
# oauth2-proxy.cfg
set_xauthrequest = true
pass_request_headers = X-Artifact-Proxy-Auth=changeme-proxy-shared-secret
```

### Google Cloud IAP

IAP forwards the user's email in `X-Goog-Authenticated-User-Email` as
`accounts.google.com:<email>`. You will need to strip the prefix, which IAP does not do
natively. A lightweight Cloud Run sidecar or Nginx `sub_filter` is the usual approach.

Once the prefix is stripped, configure:

```yaml
auth:
  mode: header-trust
  header_trust:
    email_header: X-Goog-Authenticated-User-Email
    proxy_secret_env: ARTIFACT_PROXY_SECRET
```

Inject `X-Artifact-Proxy-Auth` from the sidecar that strips the prefix.

### AWS ALB + OIDC

ALB with OIDC authentication forwards the user's email in `X-Amzn-Oidc-Data` (a JWT) or,
when claim forwarding is enabled, in a plain header such as `X-Auth-Request-Email`. Use an
ALB Lambda authorizer or a small reverse proxy (Nginx, Envoy) to extract the email into a
plain header and inject `X-Artifact-Proxy-Auth`:

```yaml
auth:
  mode: header-trust
  header_trust:
    email_header: X-Auth-Request-Email    # set by your extraction layer
    proxy_secret_env: ARTIFACT_PROXY_SECRET
```

## Verification checklist

After deploying, verify the following:

- [ ] **Boot fails without the secret.** Temporarily unset `ARTIFACT_PROXY_SECRET` and
  restart Artifact. It should refuse to start with an error message. Restore the variable
  and restart before continuing.

- [ ] **Authenticated request lands as the correct user.** Send a request through the proxy
  with the identity headers and confirm that `artifact.me.email` reflects the proxied user,
  not a fallback.

  ```bash
  # Through the proxy (authenticated)
  curl -s https://my-site.artifact.corp.example.com/api/v1/sites | jq .
  # Should succeed and return the site list.
  ```

- [ ] **Request without the proxy-auth header is rejected.** Send directly to Artifact
  (bypassing the proxy) and confirm a 401:

  ```bash
  # Direct to Artifact port (no proxy-auth header)
  curl -s http://localhost:8443/api/v1/sites
  # Expected: {"error":"Missing or invalid proxy authentication. ..."}
  ```

- [ ] **Request with the proxy-auth header but no email header is rejected.** Send with
  `X-Artifact-Proxy-Auth` set but omit the email header and confirm a 401.

- [ ] **Admin console returns 403 for all users** (expected until groups support is added).
  Visit `https://admin.artifact.corp.example.com` and confirm the 403 response — this is
  correct behavior in header-trust mode.
