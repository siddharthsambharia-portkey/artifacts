# Okta OIDC Setup

1. Create an OIDC app in Okta (Web application)
2. Set redirect URI: `https://artifact.corp.example.com/auth/callback`
3. Configure `artifact.yaml`:

```yaml
auth:
  mode: oidc
  oidc:
    issuer: https://your-org.okta.com
    client_id: YOUR_CLIENT_ID
    client_secret_env: ARTIFACT_OIDC_SECRET
    groups_claim: groups
```

4. Set the secret: `export ARTIFACT_OIDC_SECRET=...`
5. Start: `artifact serve --config artifact.yaml`

Users visit your apex domain and are redirected to Okta login.
