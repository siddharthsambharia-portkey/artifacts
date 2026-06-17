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
    groups_header: X-Auth-Request-Groups  # default; comma-separated groups forwarded by the proxy
    proxy_secret_env: ARTIFACT_PROXY_SECRET  # name of the env var holding the shared secret
```

| Field | Default | What it does |
|---|---|---|
| `email_header` | `X-Auth-Request-Email` | Header the proxy sends with the authenticated user's email. Required. |
| `name_header` | `X-Auth-Request-User` | Header with the user's display name. If missing or empty, Artifact falls back to the local-part of the email. |
| `groups_header` | `X-Auth-Request-Groups` | Header carrying a comma-separated list of groups the proxy assigns to the user. If absent or empty, falls back to `["employees"]`. |
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

## Groups and admin access

Artifact reads the user's groups from the header named by `groups_header` (default:
`X-Auth-Request-Groups`). The header value is a comma-separated list of group names:

```
X-Auth-Request-Groups: admins,employees
```

Artifact splits, trims, and stamps those groups onto the signed-in user. The groups are
then used for:

- **Admin console gating** — users in the `admins` group can reach `admin.<domain>`.
- **Governed-mode group-scoped visibility** — sites with `visibility: group` check whether
  the requesting user is in the specified groups.

If the header is absent or empty, Artifact falls back to `["employees"]`, keeping all
authenticated users in the default employee group and preventing lockout in deployments
where groups are not yet configured.

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
    groups_header: X-Pomerium-Claim-Groups
    proxy_secret_env: ARTIFACT_PROXY_SECRET
```

Configure Pomerium to send `X-Artifact-Proxy-Auth: <secret>` on every forwarded request and
set `ARTIFACT_PROXY_SECRET` to the same value in Artifact's environment. Pomerium forwards
the `groups` claim from the identity provider as `X-Pomerium-Claim-Groups` when the IdP
includes it in the token.

### oauth2-proxy

oauth2-proxy forwards user identity in `X-Auth-Request-Email` and `X-Auth-Request-User` by
default, which match Artifact's defaults. To also forward groups, enable `set_xauthrequest`
and add the `groups` scope:

```yaml
# artifact.yaml — oauth2-proxy defaults already match email/name/groups headers:
auth:
  mode: header-trust
  header_trust:
    proxy_secret_env: ARTIFACT_PROXY_SECRET
```

In `oauth2-proxy.cfg`:

```ini
# oauth2-proxy.cfg
set_xauthrequest = true          # emits X-Auth-Request-Email, X-Auth-Request-User, X-Auth-Request-Groups
scope = openid email profile groups
pass_request_headers = X-Artifact-Proxy-Auth=changeme-proxy-shared-secret
```

With `set_xauthrequest = true` and the `groups` scope, oauth2-proxy populates
`X-Auth-Request-Groups` with a comma-separated list of the user's IdP groups. Artifact
reads that header automatically with its default `groups_header` setting.

#### Cross-subdomain SSO with oauth2-proxy

Artifact serves each site on its own subdomain (`my-site.<domain>`). For a single sign-on
session to span all subdomains, the oauth2-proxy session cookie must be scoped to the parent
domain. Add these settings to `oauth2-proxy.cfg`:

```ini
# Cross-subdomain session — required for Artifact's per-site subdomain model
cookie_domain    = .artifact.corp.example.com   # leading dot covers all subdomains
whitelist_domain = .artifact.corp.example.com   # allow post-login redirects to any subdomain
```

| Setting | Value | Purpose |
|---------|-------|---------|
| `cookie_domain` | `.<domain>` (leading dot) | Sets the `Domain` attribute on the oauth2-proxy session cookie so it is sent to all subdomains of `<domain>` |
| `whitelist_domain` | `.<domain>` | Permits oauth2-proxy to redirect back to any `<domain>` subdomain after login completes |

> **Why the leading dot matters:** a cookie set for `artifact.corp.example.com` (no dot) is
> only sent to that exact hostname. A cookie set for `.artifact.corp.example.com` (with dot) is
> sent to `artifact.corp.example.com` and every subdomain — which is what Artifact's
> subdomain-per-site model requires.

Replace `artifact.corp.example.com` with your actual `config.domain` value.

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

- [ ] **Admin console is reachable for admins-group users.** Sign in through the proxy as a
  user whose groups header includes `admins` and confirm `https://admin.artifact.corp.example.com`
  loads. A user whose groups header does not include `admins` should receive a 403.

- [ ] **Cross-subdomain session works.** If using oauth2-proxy, confirm that `cookie_domain`
  is set to `.<domain>` and that navigating from one site subdomain to another does not
  re-prompt for login.

## Cross-subdomain SSO: native OIDC vs header-trust

For reference, here is how each auth mode handles session cookies across `*.<domain>`:

| Auth mode | How cross-subdomain session is achieved |
|-----------|-----------------------------------------|
| Native OIDC (`auth.mode: oidc`) | Artifact sets `cookie.Domain = "." + domain` automatically in `CallbackHandler`. No operator action needed. |
| Header-trust + oauth2-proxy | Set `cookie_domain = .<domain>` and `whitelist_domain = .<domain>` in `oauth2-proxy.cfg` (see above). |
| Header-trust + Pomerium | Pomerium handles its own session; the forwarded identity headers arrive fresh on every request — no separate cookie-domain config needed. |
| Header-trust + Google IAP | IAP manages its own session centrally; no per-subdomain cookie config is required. |

For the wildcard TLS certificate that makes `*.<domain>` HTTPS work, see
[deploy/recipes/wildcard-tls-gcp.md](../deploy/recipes/wildcard-tls-gcp.md).
