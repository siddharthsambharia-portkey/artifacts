# Pomerium + Artifact

Configure Pomerium to forward identity headers and proxy auth:

```yaml
# pomerium policy
- from: https://*.artifact.corp.example.com
  to: http://artifact:8443
  allowed_users:
    - domain: corp.example.com
```

Artifact config:

```yaml
auth:
  mode: header-trust
  header_trust:
    email_header: X-Pomerium-Claim-Email
    name_header: X-Pomerium-Claim-Name
    proxy_secret_env: ARTIFACT_PROXY_SECRET
```

Set `ARTIFACT_PROXY_SECRET` and configure Pomerium to send `X-Artifact-Proxy-Auth` with the same value.
