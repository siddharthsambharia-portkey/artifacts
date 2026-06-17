# Auth guide: Okta (OIDC)

This guide walks through connecting Artifact to Okta as your identity provider. When done,
every employee who visits your Artifact domain will be redirected to Okta, and Artifact will
receive their email, name, and group memberships from the ID token.

See also: [Auth — Microsoft Entra ID](auth-entra.md) · [Auth — Google Workspace](auth-google.md) ·
[Auth — header-trust](auth-header-trust.md)

---

## Prerequisites

- An Okta org (developer org or production).
- A production Artifact deployment with a real domain. Terminate TLS at your load balancer or identity proxy. The callback URL must be reachable by your browser after Okta redirects back. (For local dev, use `auth: dev` instead.)
- The `artifact` binary on your `PATH` or `go run ./cmd/artifact`.

---

## Step 1: Create an Okta OIDC application

1. In the Okta Admin Console, go to **Applications → Applications → Create App Integration**.
2. Choose **OIDC – OpenID Connect** as the sign-in method, then **Web Application** as the
   application type.
3. Name it something like `Artifact (internal)`.
4. Under **Grant type**, make sure **Authorization Code** is checked.
5. Set the **Sign-in redirect URI** to:

   ```
   https://<your-domain>/auth/callback
   ```

   Replace `<your-domain>` with your Artifact domain (e.g. `artifact.corp.example.com`).
   This is the real callback route registered in Artifact (`GET /auth/callback`).

6. Leave the sign-out redirect URI blank unless you want a custom post-logout page.
7. Under **Assignments**, assign the app to the groups of employees who should have access, or
   choose "Allow everyone in your organization to access".
8. Click **Save**. Okta shows you the **Client ID** and lets you create a **Client secret** —
   copy both; you will need them in step 3.

**Find your issuer URL.** It is one of:

- **Org authorization server**: `https://<okta-domain>` (e.g. `https://yourco.okta.com`).
  Available in all orgs; the issuer is just your org URL with no path.
- **Custom authorization server**: `https://<okta-domain>/oauth2/<server-id>` (e.g.
  `.../oauth2/default`). Use this if you have a custom server; it lets you add custom claims
  without upgrading your Okta plan.

To find a custom server: **Security → API → Authorization Servers**. The `default` server's
issuer is shown in the table.

---

## Step 2: Configure the groups claim

Artifact reads group memberships from the `groups` claim in the ID token. By default, Okta
does **not** include groups — you need to add a claim.

### Using a custom authorization server (recommended)

1. Go to **Security → API → Authorization Servers** and open your server (e.g. `default`).
2. Click the **Claims** tab, then **Add Claim**.
3. Set:

   | Field | Value |
   |---|---|
   | Name | `groups` |
   | Include in token type | **ID Token** — **Always** |
   | Value type | **Groups** |
   | Filter | **Matches regex** · `.*` (all groups) or a more restrictive pattern |
   | Include in | Select **The following scopes** → `openid` (or leave "Any scope") |

4. Save. The next login will include `"groups": ["GroupA", "GroupB", ...]` in the ID token.

### Using the org authorization server

1. Go to **Applications → [your app] → Sign On → OpenID Connect ID Token**.
2. Set **Groups claim type** to `Filter` and **Groups claim filter** to
   `groups` + `Matches regex` + `.*`.
3. Save. The groups claim is added to the ID token for this app.

---

## Step 3: Configure Artifact

### Set the client secret

Artifact reads the client secret from an environment variable named by `client_secret_env`
(default: `ARTIFACT_OIDC_SECRET`). Set it before starting the server:

```bash
export ARTIFACT_OIDC_SECRET=<okta-client-secret>
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
    issuer: https://yourco.okta.com              # or https://yourco.okta.com/oauth2/default
    client_id: 0oaXXXXXXXXXXXXXXXX              # from the Okta app
    client_secret_env: ARTIFACT_OIDC_SECRET      # env var holding the client secret
    groups_claim: groups                         # matches the claim name you set in step 2
```

The fields map exactly to the `OIDC` struct in `internal/config/config.go`:

| YAML key | Go field | Default | Notes |
|---|---|---|---|
| `issuer` | `Issuer` | _(required)_ | Okta org URL or custom auth server URL |
| `client_id` | `ClientID` | _(required)_ | From the Okta app |
| `client_secret_env` | `ClientSecretEnv` | `ARTIFACT_OIDC_SECRET` | Name of the env var holding the secret |
| `groups_claim` | `GroupsClaim` | `groups` | Claim name in the ID token |

---

## How the sign-in flow works

1. Any unauthenticated request is redirected to `GET /login`.
2. `/login` generates a random CSRF state, stores it in a short-lived cookie, and redirects
   the browser to Okta's authorization endpoint.
3. After login, Okta redirects back to `GET /auth/callback` with a `code` and the CSRF
   `state`.
4. Artifact exchanges the code for tokens, verifies the ID token signature and claims, and
   extracts `email`, `name`, and `groups` from the ID token.
5. Artifact creates a session (see [Session cookies](#session-cookies-and-the-trust-bubble)).
6. Artifact redirects the browser to the page the user originally requested.

If no groups are found in the token (the claim is absent or empty), Artifact falls back to
`["employees"]` so the user is never locked out in trust mode.

---

## Session cookies and the trust bubble

This section explains the session model. The [Entra](auth-entra.md) and
[Google](auth-google.md) guides link here rather than repeating it.

### Cookie attributes

Artifact sets a single session cookie named `artifact_session` with the following attributes:

| Attribute | Value | Why |
|---|---|---|
| `HttpOnly` | true | JavaScript on the page cannot read or steal the cookie |
| `Secure` | true | Sent only over HTTPS (omitted only in local `domain: localhost` dev) |
| `SameSite` | `Lax` | Blocks cross-site POST forgery; allows top-level navigation (the OAuth redirect) |
| `Max-Age` | `86400` (24 h) | Sessions expire after a day; re-login is required |
| `Domain` | `.artifact.corp.example.com` | The leading dot makes the cookie valid for the apex **and all subdomains** |
| `Path` | `/` | Valid for all paths |

The cookie is set by `CallbackHandler` in `internal/auth/oidc.go`. The domain is set to
`"." + cfg.Domain` whenever `cfg.Domain != "localhost"`, which is what makes
`site-a.artifact.corp.example.com`, `site-b.artifact.corp.example.com`, and
`admin.artifact.corp.example.com` all receive the same session cookie.

### Why this is the trust bubble

Artifact's trust model requires every request to come from an authenticated employee. Because
the session cookie is scoped to `.<domain>`, a single login at the apex (or at any subdomain)
authenticates the user across every Artifact site without a second redirect. This is the
"trust bubble" described in [Concepts](concepts.md): once inside your company's Artifact
domain, every request carries a verified identity.

The flip side: do not put untrusted content under your Artifact domain. A malicious site
deployed by an insider could attempt to read cookies — but `HttpOnly` prevents JavaScript
from reading `artifact_session`, so the residual risk is the subdomain-cookie-sharing
described in the [backlog note in plans/README.md](../plans/README.md).

### Groups and governed mode

After login, `artifact.me.groups` in the browser SDK is populated with the `groups` array
from the session. In governed mode:

- **Visibility gating**: if a site has a `visibility_groups` set, only users whose
  `artifact.me.groups` overlaps that list can read the site.
- **Admin console**: `admin.<domain>` is accessible only to users whose `groups` claim includes a group named exactly `admins`. Create an `admins` group in Okta and include it in the groups claim. See [Governance & admin](governance-and-admin.md) for the full model.

---

## Step 4: Start Artifact

```bash
artifact serve --config artifact.yaml
```

On first boot you should see log lines like:

```
artifact starting version=0.1.0 listen=:8443 domain=artifact.corp.example.com auth=oidc governance=trust
```

If Artifact fails to connect to the OIDC issuer at startup, you will see:

```
connect to OIDC issuer https://...: ...
```

Check that the issuer URL is correct and that the server has outbound HTTPS to Okta.

---

## Verification checklist

Work through these in order — each step depends on the previous one succeeding.

- [ ] **Server boots**: no `connect to OIDC issuer` errors in the log.
- [ ] **`/login` redirects to Okta**: open `https://<domain>/login` in a fresh browser
  (incognito). You should land on the Okta sign-in page, not an error.
- [ ] **Callback succeeds**: complete the Okta login. You should be redirected back to
  `https://<domain>/` without an error page.
- [ ] **`artifact.me` is populated**: open the browser console on any deployed site and run:

  ```js
  await artifact.ready();
  console.log(artifact.me.email, artifact.me.groups);
  ```

  You should see your Okta email and the groups you belong to. If `groups` is `["employees"]`
  only, recheck the groups claim configuration in step 2.

- [ ] **Admin user reaches the admin console**: sign in with an account in the `admins` group. Navigate to `https://admin.<domain>`. You should see the admin console, not a 403.

---

## Troubleshooting

**"Invalid OAuth state. Try logging in again."**
The `artifact_oauth_state` cookie was missing or mismatched. This happens if the state cookie
expires (5-minute TTL), if a browser extension strips cookies, or if the callback URL in Okta
does not exactly match what Artifact constructs (`https://<domain>/auth/callback`).

**"OAuth exchange failed: … Check client_id and client_secret."**
The `ARTIFACT_OIDC_SECRET` env var is empty or wrong. Verify with
`echo $ARTIFACT_OIDC_SECRET` before starting the server.

**"No id_token in OAuth response. Ensure openid scope is granted."**
The `openid` scope was removed from the Okta app or the authorization server policy blocks it.
Confirm **openid** is listed under the app's **Allowed scopes**.

**Groups are missing or wrong.**
Re-read step 2. In Okta, the groups claim must be added explicitly; it is not included in the
token by default. After adding the claim, sign out and sign back in — the old session does not
update retroactively.
