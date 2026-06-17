# Choosing an auth mode

Artifact supports two production auth modes. Pick one before you deploy; the choice is
a single line in `artifact.yaml`.

```yaml
auth:
  mode: oidc          # native OIDC — recommended default
  # mode: header-trust  # proxy-forwarded identity
```

---

## Quick comparison

| | Native OIDC (`auth.mode: oidc`) | Header-trust (`auth.mode: header-trust`) |
|---|---|---|
| **How auth works** | Artifact is the OIDC client — runs the login/callback flow, reads the ID token | An identity proxy (oauth2-proxy, Pomerium, IAP) authenticates and forwards identity in headers |
| **Groups / admins** | Read from the ID token `groups_claim` — governed mode, group-scoped visibility, and the admin console all work out of the box | Read from a configurable `groups_header` (default `X-Auth-Request-Groups`) — admin console and group visibility work once the proxy is configured to forward groups |
| **Session cookie** | Yes — Artifact manages session lifecycle in the database | No — the proxy is the session; Artifact re-validates the proxy-auth secret on every request |
| **When to pick this** | Default choice. Recommended if you are starting fresh or if the IdP admin can register an OIDC application | Pick this if you already operate an org-wide identity proxy (Pomerium, IAP, ZTNA) and want Artifact inside that boundary without registering a second OIDC app |
| **Guaranteed profile** | GCP + Okta — documented, tested, kept honest per [ADR 0001](adr/0001-guaranteed-deployment-profile-gcp-okta.md) | Supported, but outside the guaranteed profile; gaps are features not blockers |

---

## Native OIDC — recommended default

In native OIDC mode, Artifact handles the full OIDC authorization-code flow:

1. Unauthenticated requests are redirected to `/login`.
2. `/login` redirects the browser to the identity provider's authorization endpoint.
3. The identity provider redirects back to `/auth/callback` with an authorization code.
4. Artifact exchanges the code for an ID token, reads the user's email, name, and groups
   from the token claims, and sets a session cookie.

All governed-mode features — group-scoped visibility, the admin console, first-deployer
ownership — work without any extra configuration once the IdP returns groups in the
token's `groups_claim`.

**Pick this by default.** The guaranteed profile is GCP + Okta; other OIDC providers
(Entra ID, Google Workspace, any standards-compliant IdP) work the same way.

### Per-IdP setup guides

- [Auth — Okta](auth-okta.md)
- [Auth — Microsoft Entra ID](auth-entra.md)
- [Auth — Google Workspace](auth-google.md)

---

## Header-trust — proxy-fronted deployments

In header-trust mode, Artifact trusts identity forwarded in HTTP headers by a proxy that
has already authenticated the user. On every request Artifact:

1. Validates the `X-Artifact-Proxy-Auth` header against a shared secret (boot fails if
   this is not configured — a hard safety check).
2. Reads the user's email, display name, and groups from the configured headers.
3. Falls back gracefully when optional headers are absent (name → email local-part;
   groups → `["employees"]`).

There is no session cookie. The proxy is the session.

**Pick this if:**
- Your organisation already runs an org-wide identity proxy (Pomerium, Google IAP,
  AWS ALB + OIDC, Cloudflare Access, or any ZTNA gateway) and you want Artifact behind
  it without registering a second OAuth application.
- You are on GCP but don't have Okta admin access and want a quick demo — see the
  GCP-without-Okta path below.

### GCP demo path: oauth2-proxy + Google OAuth

If you're on GCP but don't have an Okta tenant, you can run oauth2-proxy in front of
Artifact using a Google OAuth client you create in your own GCP project:

1. **Create a Google OAuth 2.0 client** in [console.cloud.google.com](https://console.cloud.google.com)
   under APIs & Services → Credentials. Set the authorized redirect URI to
   `https://oauth.<your-domain>/oauth2/callback`.

2. **Run oauth2-proxy** with the Google provider:

   ```ini
   # oauth2-proxy.cfg
   provider = google
   client_id = <your-google-client-id>
   client_secret = <your-google-client-secret>
   redirect_url = https://oauth.artifact.corp.example.com/oauth2/callback
   email_domains = corp.example.com          # restrict to your company domain
   set_xauthrequest = true                   # emit X-Auth-Request-Email/User/Groups
   scope = openid email profile groups
   pass_request_headers = X-Artifact-Proxy-Auth=<shared-secret>
   upstream = http://artifact:8443
   ```

3. **Configure Artifact** in `artifact.yaml`:

   ```yaml
   auth:
     mode: header-trust
     header_trust:
       proxy_secret_env: ARTIFACT_PROXY_SECRET  # set to <shared-secret>
   ```

No Okta admin access is required. Groups come from Google Workspace directory sync
if your account is a Google Workspace account and the `groups` scope is approved.
For personal Google accounts, `X-Auth-Request-Groups` will be absent and users will
land in the default `["employees"]` group.

### Header-trust setup guide

- [Auth — header-trust (IAP / Pomerium / oauth2-proxy)](auth-header-trust.md)

---

## The guaranteed profile

The one configuration combination Artifact guarantees works end-to-end — documented,
tested, and kept honest — is **GCP + Okta**:

| Component | Guaranteed value |
|---|---|
| `auth.mode` | `oidc` (Okta) |
| `storage.driver` | `gcs` |
| `database.driver` | `postgres` (Cloud SQL) |
| `warehouse.driver` | `bigquery` |
| `notify.slack.mode` | `webhook` |

Everything outside this profile is explicitly supported (header-trust, S3, SQLite, local
storage) but is not subject to the same end-to-end verification. See
[ADR 0001](adr/0001-guaranteed-deployment-profile-gcp-okta.md) for the decision record.

---

## `dev` mode

There is a third mode, `auth.mode: dev`, which returns `dev@localhost` for every request
with no credentials check. It is safe only on a laptop for local development:

```bash
artifact dev    # starts the server in dev mode automatically
```

Never set `auth.mode: dev` in a production config.
