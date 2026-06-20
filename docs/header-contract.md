# Header contract for identity proxies

This page documents the full header protocol Artifact uses in `header-trust` mode: which
headers Artifact reads, the shared-secret requirement, and setup notes for the three most
common proxies (GCP IAP, AWS ALB + OIDC, and Pomerium). It also maps the GCP infrastructure
pattern onto AWS and Azure so you can adapt the starter templates to your cloud of choice.

For the full `header-trust` configuration reference, see
[Auth — header-trust](auth-header-trust.md).

---

## Identity headers Artifact reads

Artifact reads three headers on every request in header-trust mode. The header names are
configurable; defaults are listed below.

| Header (default name) | Config field | Required | Description |
|---|---|---|---|
| `X-Auth-Request-Email` | `auth.header_trust.email_header` | Yes | The authenticated user's email address. Artifact rejects the request with 401 if this header is missing or empty after the proxy-auth check passes. |
| `X-Auth-Request-User` | `auth.header_trust.name_header` | No | Display name. If absent or empty, Artifact falls back to the local-part of the email. |
| `X-Auth-Request-Groups` | `auth.header_trust.groups_header` | No | Comma-separated group names (e.g. `admins,employees`). If absent or empty, falls back to `["employees"]`. |

All three header names can be overridden per-proxy in `artifact.yaml`. See
[proxy-specific header names](#proxy-specific-header-names) below.

---

## `X-Artifact-Proxy-Auth` shared-secret requirement

Every request the proxy forwards to Artifact must carry:

```
X-Artifact-Proxy-Auth: <shared-secret>
```

Artifact checks this header before reading any identity header. If the value does not match
the secret in `ARTIFACT_PROXY_SECRET` (or the env var named by `proxy_secret_env`), the
request is rejected with 401 — regardless of what is in the identity headers.

### Configuration

```yaml
# artifact.yaml
auth:
  mode: header-trust
  header_trust:
    proxy_secret_env: ARTIFACT_PROXY_SECRET   # name of the env var, not the value
```

```bash
# .env  (never commit this file)
ARTIFACT_PROXY_SECRET=<random-high-entropy-value>
```

Set `ARTIFACT_PROXY_SECRET` to the same value in Artifact's environment and in your proxy's
outbound header configuration.

### Hard-fail boot rule

Artifact **refuses to start** in header-trust mode unless:

1. `proxy_secret_env` is non-empty in `artifact.yaml`.
2. The environment variable it names is non-empty at boot time.

This is enforced at startup rather than at request time because without the secret any
workload that can reach Artifact's port could forge identity headers. Network isolation alone
is not sufficient if other workloads share the same network segment.

---

## Proxy-specific header names

Different proxies use different header names for the same identity fields. Override the
defaults in `artifact.yaml` to match your proxy.

| Proxy | Email header | Name header | Groups header |
|---|---|---|---|
| oauth2-proxy (default) | `X-Auth-Request-Email` | `X-Auth-Request-User` | `X-Auth-Request-Groups` |
| Pomerium | `X-Pomerium-Claim-Email` | `X-Pomerium-Claim-Name` | `X-Pomerium-Claim-Groups` |
| GCP IAP | `X-Goog-Authenticated-User-Email` ¹ | — (not forwarded natively) | — |
| AWS ALB + OIDC | `X-Amzn-Oidc-Data` (JWT) ² | — | — |

¹ IAP prefixes the value with `accounts.google.com:`. Strip the prefix with a sidecar or
Nginx `sub_filter` before the request reaches Artifact.

² ALB puts the full OIDC token in `X-Amzn-Oidc-Data`. Use a Lambda authorizer or a thin
reverse proxy (Nginx, Envoy) to extract the email into a plain header. When using claim
forwarding, plain `X-Auth-Request-Email` is available directly.

### GCP IAP

```yaml
# artifact.yaml
auth:
  mode: header-trust
  header_trust:
    email_header: X-Goog-Authenticated-User-Email   # strip "accounts.google.com:" prefix first
    proxy_secret_env: ARTIFACT_PROXY_SECRET
```

Configure IAP's backend service (or a sidecar) to:

1. Strip the `accounts.google.com:` prefix from `X-Goog-Authenticated-User-Email`.
2. Inject `X-Artifact-Proxy-Auth: <secret>` on every forwarded request.

IAP manages its own session centrally — no per-subdomain cookie configuration is needed.

### AWS ALB + OIDC

```yaml
# artifact.yaml
auth:
  mode: header-trust
  header_trust:
    email_header: X-Auth-Request-Email   # set by your extraction layer
    proxy_secret_env: ARTIFACT_PROXY_SECRET
```

Configure your ALB + extraction layer to:

1. Enable OIDC authentication on the ALB listener.
2. Use a Lambda authorizer or a small Nginx/Envoy sidecar to extract the email claim from
   `X-Amzn-Oidc-Data` into `X-Auth-Request-Email`. If your IdP supports claim forwarding,
   enable it on the ALB so the claim arrives as a plain header.
3. Inject `X-Artifact-Proxy-Auth: <secret>` from the extraction layer.

### Pomerium

```yaml
# artifact.yaml
auth:
  mode: header-trust
  header_trust:
    email_header:  X-Pomerium-Claim-Email
    name_header:   X-Pomerium-Claim-Name
    groups_header: X-Pomerium-Claim-Groups
    proxy_secret_env: ARTIFACT_PROXY_SECRET
```

In your Pomerium policy, add a `set_request_headers` entry to inject
`X-Artifact-Proxy-Auth` on every request Pomerium forwards to Artifact. Pomerium forwards
the `groups` claim from the IdP token as `X-Pomerium-Claim-Groups` when the IdP includes it.

See the full recipe at [`deploy/recipes/pomerium.md`](../deploy/recipes/pomerium.md).

---

## Cross-cloud infrastructure mapping

The GCP Terraform starter (`deploy/terraform/gcp/main.tf`) uses GCS, Workload Identity, and
Cloud SQL. The AWS and Azure patterns follow the same structure with cloud-native equivalents.

### Storage

| Layer | GCP | AWS | Azure |
|---|---|---|---|
| Object storage | GCS (`storage.driver: gcs`) | S3 (`storage.driver: s3`) | Azure Blob Storage — use `storage.driver: s3` with the Blob S3-compatible endpoint (`<account>.blob.core.windows.net`) |
| Bucket config | `config.storage.bucket: <gcs-bucket>` | `config.storage.bucket: <s3-bucket>` | `config.storage.bucket: <container-name>` |
| Terraform starter | `deploy/terraform/gcp/main.tf` | `deploy/terraform/aws/main.tf` | *(not yet provided; create a storage account and container manually or via your org's Terraform modules)* |

### Identity (no-key path)

On each cloud the recommended path is workload identity federation — the pod authenticates
to storage using a projected token rather than a long-lived key.

| Layer | GCP (GKE) | AWS (EKS) | Azure (AKS) |
|---|---|---|---|
| Mechanism | Workload Identity | IRSA (IAM Roles for Service Accounts) | Managed Identity (Workload Identity Federation) |
| ServiceAccount annotation | `iam.gke.io/gcp-service-account: <sa>@<project>.iam.gserviceaccount.com` | `eks.amazonaws.com/role-arn: arn:aws:iam::<account>:role/<role>` | `azure.workload.identity/client-id: <managed-identity-client-id>` |
| `automountServiceAccountToken` | `true` (required for token projection) | `true` (required for token projection) | `true` (required for token projection) |
| IAM grant | `roles/storage.objectAdmin` on the bucket | `s3:GetObject`, `s3:PutObject`, `s3:DeleteObject` on the bucket | `Storage Blob Data Contributor` on the container |

In all three cases, set the annotation on the Kubernetes ServiceAccount via the Helm chart:

```bash
# GKE
--set serviceAccount.annotations."iam\.gke\.io/gcp-service-account"="<sa>@<project>.iam.gserviceaccount.com"
--set serviceAccount.automountServiceAccountToken=true

# EKS
--set serviceAccount.annotations."eks\.amazonaws\.com/role-arn"="arn:aws:iam::<account>:role/<role>"
--set serviceAccount.automountServiceAccountToken=true

# AKS
--set serviceAccount.annotations."azure\.workload\.identity/client-id"="<managed-identity-client-id>"
--set serviceAccount.automountServiceAccountToken=true
```

### Low-power key fallback

When the workload identity binding cannot be created (e.g. insufficient IAM permissions in
the project or account), each cloud has an equivalent key-based fallback that the Helm chart
supports via `storageKeyFallback`:

| Cloud | Key type | Helm values |
|---|---|---|
| GCP | GCS service-account JSON key | `storageKeyFallback.enabled=true`, `storageKeyFallback.secretName=<secret>`, `storageKeyFallback.gcsKeyFileKey=key.json` |
| AWS | S3 access key + secret | `storageKeyFallback.enabled=true`, `storageKeyFallback.secretName=<secret>`, `storageKeyFallback.s3AccessKeyKey=access-key`, `storageKeyFallback.s3SecretKeyKey=secret-key` |
| Azure | Storage account key (injected as `AZURE_STORAGE_KEY` env var) | Mount the key from a Kubernetes Secret; the S3-compatible driver reads standard AWS env vars — set `AWS_ACCESS_KEY_ID` to the account name and `AWS_SECRET_ACCESS_KEY` to the key |

The low-power fallback is a stepping-stone. Migrate to workload identity federation as soon
as the necessary IAM permissions are available — key rotation and secret sprawl add
operational overhead that the no-key path avoids.

### Managed database

| Layer | GCP | AWS | Azure |
|---|---|---|---|
| Managed Postgres | Cloud SQL Postgres 16 | RDS Postgres 16 | Azure Database for PostgreSQL (flexible server) |
| Connection method | Cloud SQL Auth Proxy sidecar (socket DSN: `host=/cloudsql/<connection_name>`) | Direct TCP DSN (`host=<rds-endpoint>`) or RDS Proxy | Direct TCP DSN (`host=<flexible-server-host>`) |
| IAM authentication | Workload Identity token via Auth Proxy | RDS IAM authentication (optional; set `rds_iam=true` on the RDS instance) | Azure AD authentication (optional; enable on the flexible server) |
| Helm sidecar toggle | `cloudSqlProxy.enabled=true` | *(not needed — direct TCP)* | *(not needed — direct TCP)* |
| Terraform starter | `deploy/terraform/gcp/main.tf` (Cloud SQL `db-f1-micro`) | `deploy/terraform/aws/main.tf` (RDS `db.t3.micro`) | *(not yet provided)* |

On GCP, enable the Cloud SQL Auth Proxy sidecar before attempting to connect (see
[Self-hosting — Enabling the Cloud SQL Auth Proxy sidecar](self-hosting.md)). On AWS and Azure,
Artifact connects directly over TCP; set `database.url_env` to the DSN in a Kubernetes Secret.

### Summary table

| Concern | GCP | AWS | Azure |
|---|---|---|---|
| Storage driver | `gcs` | `s3` | `s3` (Blob S3-compatible endpoint) |
| Identity (preferred) | Workload Identity (GKE) | IRSA (EKS) | Managed Identity (AKS) |
| Identity (fallback) | GCS JSON key | S3 access key | Storage account key |
| Managed DB | Cloud SQL + Auth Proxy sidecar | RDS (direct TCP) | Azure PostgreSQL flexible server (direct TCP) |
| Terraform starter | `deploy/terraform/gcp/` | `deploy/terraform/aws/` | *(manual or org modules)* |
