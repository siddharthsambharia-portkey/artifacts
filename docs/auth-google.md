# Auth guide: Google Workspace (OIDC)

This guide walks through connecting Artifact to Google as your identity provider using an
OAuth 2.0 Web Client ID. When done, employees will sign in with their Google Workspace
accounts and Artifact will receive their email and name.

> **Groups caveat — read before you start.** Google's standard OIDC ID token does **not**
> include group membership. If you need governed-mode group-scoped visibility or group-gated
> admin access, see [Groups and your options](#groups-and-your-options) below before
> proceeding. The rest of this guide covers the common case where you want all Workspace
> members to have equal access (trust mode or domain-restricted).

See also: [Auth — Okta](auth-okta.md) · [Auth — Microsoft Entra ID](auth-entra.md) ·
[Auth — header-trust](auth-header-trust.md)

---

## Prerequisites

- A Google Workspace organization (or a GCP project tied to one).
- A production Artifact deployment with a real domain. Terminate TLS at your load balancer or identity proxy.
- Access to [Google Cloud Console](https://console.cloud.google.com) to create OAuth
  credentials.

---

## Step 1: Create an OAuth 2.0 client ID in Google Cloud

1. Open the [Google Cloud Console](https://console.cloud.google.com) and select (or create) a
   project. The project should belong to your Workspace organization.
2. Go to **APIs & Services → OAuth consent screen**.
   - Choose **Internal** as the user type if you want to restrict access to your Workspace
     org only (strongly recommended for an internal tool like Artifact).
   - Fill in the **App name**, **User support email**, and **Developer contact information**.
   - Under **Scopes**, add `openid`, `email`, and `profile`. No extra scopes are needed for
     the basic (no-groups) setup.
   - Save and continue through the remaining screens.
3. Go to **APIs & Services → Credentials → Create credentials → OAuth client ID**.
4. Choose **Web application** as the application type.
5. Set a name (e.g. `Artifact internal`).
6. Under **Authorized redirect URIs**, add:

   ```
   https://<your-domain>/auth/callback
   ```

   Replace `<your-domain>` with your Artifact domain (e.g. `artifact.corp.example.com`). This
   is the real callback route registered in Artifact (`GET /auth/callback`).

7. Click **Create**. Google shows you the **Client ID** and **Client Secret** — download the
   JSON or copy both values immediately.

---

## Step 2: Configure Artifact

### Set the client secret

```bash
export ARTIFACT_OIDC_SECRET=<google-client-secret>
```

Or add it to your `.env` file (see `.env.example` at the repo root). Never commit the secret
to `artifact.yaml`.

### Write your `artifact.yaml`

```yaml
domain: artifact.corp.example.com   # your apex domain

tls:
  mode: off  # terminate TLS at your load balancer or identity proxy

auth:
  mode: oidc
  oidc:
    issuer: https://accounts.google.com         # Google's OIDC issuer — fixed value
    client_id: <your-client-id>.apps.googleusercontent.com
    client_secret_env: ARTIFACT_OIDC_SECRET     # env var holding the secret
    groups_claim: groups                        # see "Groups and your options" below
```

The fields map exactly to the `OIDC` struct in `internal/config/config.go`:

| YAML key | Go field | Default | Notes |
|---|---|---|---|
| `issuer` | `Issuer` | _(required)_ | Always `https://accounts.google.com` for Google |
| `client_id` | `ClientID` | _(required)_ | From the OAuth 2.0 client ID (ends in `.apps.googleusercontent.com`) |
| `client_secret_env` | `ClientSecretEnv` | `ARTIFACT_OIDC_SECRET` | Name of the env var holding the secret |
| `groups_claim` | `GroupsClaim` | `groups` | See below — this claim is not populated by Google by default |

---

## Groups and your options

Google's standard OIDC ID token carries `email`, `name`, `picture`, `sub`, and (for Workspace
accounts) the `hd` (hosted domain) claim. It does **not** include group membership. This
means that out of the box, `artifact.me.groups` will be `["employees"]` for every user
(Artifact's fallback when the groups claim is absent or empty).

Depending on what you need, pick one of the options below.

### Option A: Domain-restricted trust mode (simplest)

If all Workspace members should have equal access to Artifact and you do not need per-site
visibility gating or group-gated admin access, this is the recommended approach:

1. Use the configuration in step 2 as-is.
2. Set `governance.mode: trust` in `artifact.yaml` (this is the default).
3. Every user who authenticates with your Workspace domain gets in; all sites are visible to
   all employees.

If you want to prevent users from _other_ Google accounts (personal Gmail, partner Workspace
tenants) from logging in, keep the OAuth consent screen set to **Internal** (step 1 item 2),
which restricts the client to your Workspace org automatically.

The `hd` claim in the Google ID token contains the user's hosted domain (e.g. `corp.example.com`).
Artifact does not currently enforce `hd` at the middleware layer — the access boundary is
controlled by the OAuth consent screen's **Internal** setting and your Okta/Entra alternative
if you need finer control.

### Option B: Cloud Identity groups via an identity proxy

If you need group-scoped governed mode (visibility groups, admin groups), you need a mechanism
that injects a `groups` claim that Google's OIDC token does not provide. Two practical paths:

**B1 — Identity proxy (oauth2-proxy / Pomerium / GCP IAP with header forwarding)**

Run an identity proxy in front of Artifact that reads Google Workspace group membership from
the [Cloud Identity Groups API](https://cloud.google.com/identity/docs/reference/rest/v1/groups)
and injects it as a trusted header. Switch Artifact to `auth: header-trust` mode, which reads
identity from the proxy's headers rather than performing OIDC itself.

See [Auth — header-trust](auth-header-trust.md) for how to configure Artifact in that mode.

**B2 — Custom OIDC token enrichment**

Use a small server-side token broker that fetches group membership from the
[Admin SDK Directory API](https://developers.google.com/admin-sdk/directory/reference/rest/v1/groups/list)
and mints a new JWT with a `groups` claim before forwarding to Artifact. This is more
engineering work but keeps the OIDC flow in place. Point `oidc.issuer` at your broker's
discovery endpoint rather than `https://accounts.google.com`.

Neither B1 nor B2 is provided out of the box; they are separate infrastructure components.
If you are starting from scratch, the Okta or Entra guides are simpler paths to governed-mode
groups.

---

## Session cookies and the trust bubble

Session handling is identical to the Okta setup. See the
[Session cookies and the trust bubble](auth-okta.md#session-cookies-and-the-trust-bubble)
section in the Okta guide for the full explanation of cookie attributes, the `.<domain>`
subdomain scope, and how groups flow into governed-mode visibility and the admin console.

---

## Step 3: Start Artifact

```bash
artifact serve --config artifact.yaml
```

Expected startup log:

```
artifact starting version=0.1.0 listen=:8443 domain=artifact.corp.example.com auth=oidc governance=trust
```

---

## Verification checklist

- [ ] **Server boots**: no `connect to OIDC issuer` errors in the log.
- [ ] **`/login` redirects to Google**: open `https://<domain>/login` in a fresh browser
  (incognito). You should land on the Google sign-in page, not an error.
- [ ] **Callback succeeds**: complete the Google login. You should land back on
  `https://<domain>/` without an error page.
- [ ] **`artifact.me` is populated**: in the browser console on any deployed site:

  ```js
  await artifact.ready();
  console.log(artifact.me.email, artifact.me.groups);
  ```

  You should see your Workspace email. `groups` will be `["employees"]` unless you implemented
  option B above.

- [ ] **Non-Workspace account is blocked** (if you chose Internal consent screen): try signing
  in with a personal Gmail account. It should be rejected by Google before the callback.

---

## Troubleshooting

**"OAuth exchange failed"**
Check that `ARTIFACT_OIDC_SECRET` matches the client secret in Google Cloud Console. Secrets
can be regenerated in **APIs & Services → Credentials → [your client] → Edit**.

**"redirect_uri_mismatch"** (Google error page)
The redirect URI in the OAuth client must exactly match `https://<your-domain>/auth/callback`.
Check for trailing slashes or port numbers in the Google Cloud Console configuration.

**"Access blocked: This app's request is invalid"**
The OAuth consent screen is in **Testing** status and your account is not listed as a test
user. Either publish the consent screen (for Internal apps this requires no review) or add
your account to the test users list.

**Users from other domains can log in**
Your OAuth consent screen user type may be **External** instead of **Internal**. Change it to
**Internal** to restrict access to your Workspace org. (Note: changing from External to
Internal is not always possible without recreating the consent screen — check the GCP docs for
your situation.)

**`groups` is always `["employees"]`**
This is expected with a plain Google OIDC setup. Google does not put group membership in the
ID token. See [Groups and your options](#groups-and-your-options) for the available paths.
