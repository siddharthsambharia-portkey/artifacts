# Auth guide: Microsoft Entra ID (OIDC)

This guide walks through connecting Artifact to Microsoft Entra ID (formerly Azure Active
Directory) as your identity provider. When done, employees will sign in with their Microsoft
work accounts and Artifact will receive their email, name, and group memberships.

See also: [Auth — Okta](auth-okta.md) · [Auth — Google Workspace](auth-google.md) ·
[Auth — header-trust](auth-header-trust.md)

---

## Prerequisites

- An Azure subscription with at least one Entra ID tenant.
- A production Artifact deployment with a real domain. Terminate TLS at your load balancer or identity proxy.
- The app registration requires **Application Administrator** or **Global Administrator**
  rights in Entra ID, or the ability to ask your Microsoft 365 admin to create the app.

---

## Step 1: Register an application in Entra ID

1. Open the [Azure portal](https://portal.azure.com) and navigate to
   **Microsoft Entra ID → App registrations → New registration**.
2. Set the **Name** to something recognizable, e.g. `Artifact (internal)`.
3. Under **Supported account types**, choose **Accounts in this organizational directory only
   (Single tenant)** unless you run a multi-tenant Entra setup.
4. Under **Redirect URI**, select **Web** and enter:

   ```
   https://<your-domain>/auth/callback
   ```

   Replace `<your-domain>` with your Artifact domain (e.g. `artifact.corp.example.com`). This
   is the real callback route registered in Artifact (`GET /auth/callback`).

5. Click **Register**. Note the **Application (client) ID** and **Directory (tenant) ID** —
   you need both.

---

## Step 2: Create a client secret

1. In your new app registration, go to **Certificates & secrets → Client secrets → New client
   secret**.
2. Add a description (e.g. `artifact-production`) and choose an expiry that fits your rotation
   policy (1 or 2 years is common).
3. Click **Add**. Copy the **Value** immediately — Entra only shows it once.

---

## Step 3: Configure the groups claim

> **Important — Entra emits group object IDs, not group names.**
> When you enable the groups claim, Entra includes each group as a UUID
> (`"f4b4c8e2-1234-…"`) in the token, not a display name like `"platform-team"`. For
> visibility groups, configure the UUID strings in your site settings, not human-readable names.
> Admin access requires the groups claim to include the literal string `admins`; because Entra
> emits UUIDs, the standard groups claim does not grant admin access.

### Add an optional groups claim

1. In your app registration, go to **Token configuration → Add optional claim**.
2. Select **ID** as the token type, then check **groups**, and click **Add**.
3. If prompted to add `profile`, `email`, and `openid` scopes via **Microsoft Graph**, accept.
4. Under **Groups**, choose the scope of groups to emit:
   - **All groups**: every group the user belongs to (can be large for deeply nested tenants).
   - **Security groups** only: recommended for most Artifact deployments.
   - **Groups assigned to the application**: only groups explicitly assigned to this app
     registration — the most predictable option.

   Groups will appear in the `groups` claim in the ID token as an array of object ID strings.

### Find a group's object ID

You need the object ID to configure admin or visibility groups. In the Azure portal:

1. Go to **Microsoft Entra ID → Groups** and search for the group by name.
2. Open the group. The **Object ID** is shown on the overview page (e.g.
   `f4b4c8e2-1234-5678-abcd-ef0123456789`).
3. Copy it exactly — this is what goes into your Artifact group configuration.

---

## Step 4: Configure Artifact

### Set the client secret

```bash
export ARTIFACT_OIDC_SECRET=<entra-client-secret>
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
    issuer: https://login.microsoftonline.com/<tenant-id>/v2.0   # your tenant ID
    client_id: <application-client-id>                           # from the app registration
    client_secret_env: ARTIFACT_OIDC_SECRET                      # env var holding the secret
    groups_claim: groups                                         # Entra emits group object IDs here
```

Replace `<tenant-id>` with the **Directory (tenant) ID** from step 1 and `<application-client-id>`
with the **Application (client) ID**.

The fields map exactly to the `OIDC` struct in `internal/config/config.go`:

| YAML key | Go field | Default | Notes |
|---|---|---|---|
| `issuer` | `Issuer` | _(required)_ | Must use the `/v2.0` endpoint for Entra |
| `client_id` | `ClientID` | _(required)_ | Application (client) ID |
| `client_secret_env` | `ClientSecretEnv` | `ARTIFACT_OIDC_SECRET` | Name of the env var holding the secret |
| `groups_claim` | `GroupsClaim` | `groups` | Entra writes group object IDs to this claim |

> **Issuer must use v2.0.** The URL `https://login.microsoftonline.com/<tenant>/v2.0` is the
> OIDC v2.0 endpoint. The v1.0 endpoint (`/oauth2/token`, no `/v2.0`) uses a different token
> format and will not verify correctly against Artifact's OIDC library.

---

## Session cookies and the trust bubble

Session handling is identical to the Okta setup. See the
[Session cookies and the trust bubble](auth-okta.md#session-cookies-and-the-trust-bubble)
section in the Okta guide for the full explanation of cookie attributes, the `.<domain>`
subdomain scope, and how groups flow into governed-mode visibility and the admin console.

The short version: after login, `artifact.me.groups` in the browser SDK will contain the
array of group object IDs from the Entra token. For visibility groups, configure the UUID
strings in your site settings, not display names. Admin access requires the literal group name
`admins` in the claim; because Entra emits UUIDs, admin access is not available with the
standard groups claim configuration.

---

## Step 5: Start Artifact

```bash
artifact serve --config artifact.yaml
```

Expected startup log:

```
artifact starting version=0.1.0 listen=:8443 domain=artifact.corp.example.com auth=oidc governance=trust
```

---

## Verification checklist

- [ ] **Server boots**: no `connect to OIDC issuer` errors in the log. If you see one, verify
  that the issuer URL ends with `/v2.0` and that the tenant ID is correct.
- [ ] **`/login` redirects to Microsoft**: open `https://<domain>/login` in a fresh browser
  (incognito). You should land on the Microsoft sign-in page.
- [ ] **Callback succeeds**: complete the login. You should land back on `https://<domain>/`
  without an error page.
- [ ] **`artifact.me` is populated**: in the browser console on any deployed site:

  ```js
  await artifact.ready();
  console.log(artifact.me.email, artifact.me.groups);
  ```

  `groups` should be an array of UUID strings. If it is `["employees"]` only, the groups claim
  was not added (recheck step 3).

- [ ] **Admin access** (if needed): admin access requires the `groups` claim to include the
  literal string `admins`. Because Entra emits UUIDs, the standard groups claim does not grant
  admin access. `https://admin.<domain>` will return 403 for all users in the default setup.

---

## Troubleshooting

**"OAuth exchange failed"**
Double-check that `ARTIFACT_OIDC_SECRET` is set to the client secret value (not the secret
ID) and that the secret has not expired in Entra.

**Groups claim is missing (`["employees"]` only)**
Ensure you completed step 3 and that the token configuration was saved. Sign out and back in
after the change — existing sessions are not refreshed retroactively.

**Groups contain UUIDs but your site config has display names**
Entra always emits group object IDs in the token. Update your visibility group configuration
to use the UUIDs you found in step 3.

**"AADSTS50011: The redirect URI … does not match"**
The redirect URI in Entra must exactly match `https://<your-domain>/auth/callback`. Check for
trailing slashes, HTTP vs HTTPS, and port numbers.

**"connect to OIDC issuer … 401 Unauthorized"**
The tenant ID in the issuer URL is wrong, or the tenant has conditional-access policies that
block OIDC discovery. Confirm with `curl https://login.microsoftonline.com/<tenant-id>/v2.0/.well-known/openid-configuration`.
