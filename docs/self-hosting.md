# Self-hosting

Artifact ships as a single static Go binary. The deployment unit is one process, one config file, one database (Postgres or SQLite), and one object storage bucket. This page covers five deployment paths: single VM, Docker Compose, Kubernetes/Helm, GCP Terraform starter, and AWS Terraform starter. Every path shares the same DNS requirements.

## DNS requirements

Every Artifact deployment needs two DNS records pointing to the same process:

| Record | Points to |
|--------|-----------|
| `*.<domain>` (wildcard) | Artifact server |
| `admin.<domain>` | Artifact server |

The wildcard handles per-site subdomains (`my-poll.artifact.corp.com`). The admin record routes `admin.<domain>` to the admin console instead of a site. Both must point to the same Artifact process.

> On GCP/AWS, use an internal Application Load Balancer with a wildcard HTTPS listener. On a single VM, a wildcard DNS A record pointing at the VM's private IP is enough.

---

## Sizing

A VM with **2 vCPU / 4 GB RAM** comfortably serves a company of ~5,000 employees under typical internal-tooling load. SQLite is sufficient at that scale. Switch to Postgres and add replicas when you need horizontal scale or higher write throughput.

---

## Deploy on a single VM

Copy the binary, write one config file, and run `artifact serve`.

### 1. Install the binary

```bash
curl -fsSL https://raw.githubusercontent.com/siddharthsambharia-portkey/artifacts/main/scripts/install.sh | sh
# or build from source (requires Go 1.25+):
git clone https://github.com/siddharthsambharia-portkey/artifacts && cd artifacts
make build   # produces ./bin/artifact
sudo cp bin/artifact /usr/local/bin/artifact
```

### 2. Write `artifact.yaml`

A minimal production config with Postgres and S3:

```yaml
domain: artifact.corp.example.com
listen: ":8443"
tls:
  mode: off   # terminate TLS at the corporate LB

auth:
  mode: oidc
  oidc:
    issuer: https://corp.okta.com
    client_id: 0oa...your_client_id
    client_secret_env: ARTIFACT_OIDC_SECRET

storage:
  driver: s3
  bucket: artifact-sites

database:
  driver: postgres
  url_env: ARTIFACT_DATABASE_URL

governance:
  mode: governed
```

### 3. Set secrets and start

```bash
export ARTIFACT_OIDC_SECRET=<your-oidc-client-secret>
export ARTIFACT_DATABASE_URL=postgres://artifact:pass@localhost:5432/artifact?sslmode=disable
artifact serve --config /etc/artifact/artifact.yaml
```

### 4. Run as a systemd service (recommended)

```ini
[Unit]
Description=Artifact
After=network.target

[Service]
User=artifact
EnvironmentFile=/etc/artifact/.env
ExecStart=/usr/local/bin/artifact serve --config /etc/artifact/artifact.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable --now artifact
```

---

## Run with Docker Compose

`deploy/docker-compose.yml` runs Artifact, Postgres 16, and MinIO (S3-compatible) as a three-container stack. Use it as a starting point for a VM-based deployment without external cloud services, or for local integration testing.

### Start the stack

```bash
cd deploy
docker compose up -d
```

The Compose file:

- **artifact** — built from `deploy/Dockerfile`; config mounted from
  `deploy/artifact.docker.yaml`; `ARTIFACT_DATABASE_URL` and MinIO credentials set via env.
- **postgres** — Postgres 16 Alpine; persisted to a named volume `pgdata`.
- **minio** — MinIO object store; persisted to `miniodata`; init container creates the
  `artifact-sites` bucket on first start.
- **minio-init** — one-shot container that creates the bucket, then exits.

Port `8443` is published to the host. MinIO's API is on `9000` and its console on `9001`.

### Customize auth and governance

Edit `deploy/artifact.docker.yaml` for domain, auth, and governance settings. The Compose config defaults to `auth.mode: dev`. Change it to `oidc` or `header-trust` for production, and set the corresponding env vars in a `.env` file next to `docker-compose.yml`.

---

## Deploy on Kubernetes with Helm

The Helm chart in `deploy/helm/artifact/` packages the deployment, service, and ingress. It expects Postgres and S3-compatible object storage to be available externally (RDS, Cloud SQL, MinIO, etc.).

### Install the Helm chart

```bash
helm install artifact ./deploy/helm/artifact/ \
  --set config.domain=artifact.corp.example.com \
  --set config.auth.mode=oidc \
  --set externalDatabase.url=postgres://artifact:secret@rds-host:5432/artifact \
  --set externalS3.accessKey=<key> \
  --set-string externalS3.secretKey=<secret>
```

### Key `values.yaml` fields

| Key | Default | Description |
|-----|---------|-------------|
| `image.repository` | `ghcr.io/siddharthsambharia-portkey/artifacts` | Container image |
| `image.tag` | `v0.1.0` | Image tag to deploy |
| `replicaCount` | `1` | Number of replicas. See multi-replica note below. |
| `ingress.enabled` | `true` | Create an Ingress resource |
| `ingress.className` | `nginx` | Ingress class |
| `ingress.hosts[0].host` | `artifact.corp.example.com` | Ingress hostname |
| `config.*` | see values.yaml | Artifact config fields merged into the config map |
| `config.domain` | `artifact.corp.example.com` | Base domain |
| `config.auth.mode` | `oidc` | Auth mode (`oidc` \| `header-trust` \| `dev`) |
| `config.storage.driver` | `s3` | Storage driver (`s3` \| `gcs` \| `local`) |
| `config.storage.bucket` | `artifact-sites` | Bucket name (S3 or GCS) |
| `config.database.driver` | `postgres` | Database driver |
| `config.governance.mode` | `trust` | Governance mode |
| `serviceAccount.create` | `true` | Create a Kubernetes ServiceAccount for the pod |
| `serviceAccount.annotations` | `{}` | Annotations on the ServiceAccount — set `iam.gke.io/gcp-service-account` for Workload Identity (GCS) |
| `serviceAccount.automountServiceAccountToken` | `false` | Set to `true` on GKE when using Workload Identity (required for token projection) |
| `externalDatabase.enabled` | `true` | Mount a Postgres DSN into the pod |
| `externalDatabase.url` | — | Postgres DSN (plaintext — use `secretName` in production) |
| `externalDatabase.secretName` | `""` | Kubernetes Secret holding the DB DSN (recommended) |
| `externalS3.enabled` | `true` | Inject S3 credentials as env vars (only used when `storage.driver: s3`) |
| `externalS3.accessKey` | `""` | S3 access key ID |
| `externalS3.secretKey` | `""` | S3 secret (stored in a Kubernetes Secret) |
| `oidcSecret.secretName` | `""` | Kubernetes Secret holding the OIDC client secret (used when `auth.mode: oidc`) |
| `headerTrustSecret.secretName` | `""` | Kubernetes Secret holding the proxy shared secret (used when `auth.mode: header-trust`) |
| `certificate.enabled` | `false` | Render a cert-manager `Certificate` for `*.<domain>` + apex (requires cert-manager CRDs) |
| `certificate.clusterIssuer` | `letsencrypt-dns01` | Name of the cert-manager `ClusterIssuer` to use (must be DNS-01) |
| `certificate.secretName` | `<release>-wildcard-tls` | Kubernetes Secret cert-manager writes and the Ingress references |
| `nats.enabled` | `false` | Enable NATS adapter for multi-replica realtime |
| `nats.url` | `nats://nats:4222` | NATS server URL |
| `resources.limits` | 2 CPU / 4 Gi | Resource limits |
| `resources.requests` | 500m / 1 Gi | Resource requests |

### Enable multi-replica realtime

A single replica uses an in-process WebSocket hub. Realtime events (DB subscriptions, presence) are not fanned out across pods. To run more than one replica, deploy a NATS server and set `nats.enabled: true`. The hub then uses NATS as a pub/sub bus between replicas.

### Wildcard TLS for `*.<domain>`

Artifact's per-site subdomain model (`my-site.<domain>`) requires a wildcard TLS certificate.
**HTTP-01 ACME challenges cannot issue wildcard certificates** — only DNS-01 can, because it
proves zone ownership rather than per-hostname reachability.

For the GCP profile, use cert-manager with a Cloud DNS DNS-01 solver. See the full walkthrough:
[deploy/recipes/wildcard-tls-gcp.md](../deploy/recipes/wildcard-tls-gcp.md).

The Helm chart can optionally render the `cert-manager.io/v1 Certificate` resource for you
(covers `*.<domain>` + apex), wired to the Ingress TLS secret:

```bash
helm upgrade --install artifact ./deploy/helm/artifact/ \
  -f deploy/helm/artifact/values-gcp.yaml \
  --set certificate.enabled=true \
  --set certificate.clusterIssuer=letsencrypt-clouddns \
  --set certificate.secretName=artifact-wildcard-tls
```

Set `certificate.enabled=false` (the default) if you manage the certificate outside the chart
or if cert-manager CRDs are not installed in the cluster.

> **Ingress note:** wildcard subdomain routing (`*.<domain>`) requires your Ingress controller
> to support wildcard hosts. nginx-ingress does. Add a `*.<domain>` entry to `ingress.hosts`
> and reference the TLS secret in `ingress.tls` to enable HTTPS on all site subdomains.

---

## Provision GCP infrastructure with Terraform

`deploy/terraform/gcp/main.tf` is a **starter template** — not a production-ready module.
It provisions three things for the guaranteed GCP + Okta profile:

| Resource | Purpose |
|----------|---------|
| GCS bucket | Site storage (`config.storage.driver: gcs`) |
| Cloud SQL Postgres 16 | Application database (`db-f1-micro`) |
| GCP service account | Workload Identity principal — the pod reads GCS via ADC with no JSON key |

**What the starter template does not wire:** an internal Application Load Balancer, Identity
Aware Proxy (IAP), a wildcard TLS certificate, or VPC peering. Add these before going to
production. For the wildcard TLS certificate, follow the DNS-01 recipe in
[deploy/recipes/wildcard-tls-gcp.md](../deploy/recipes/wildcard-tls-gcp.md).

```bash
cd deploy/terraform/gcp
terraform init
terraform apply -var project_id=my-corp-project
```

`apply` prints three outputs that feed directly into the Helm chart:

| Output | Helm value |
|--------|-----------|
| `storage_bucket` | `config.storage.bucket` |
| `cloudsql_connection_name` | Build `externalDatabase.url` — `postgres://artifact:<pw>@/<db>?host=/cloudsql/<connection_name>` |
| `workload_identity_sa_email` | `serviceAccount.annotations."iam.gke.io/gcp-service-account"` |

After `apply`, create the required Kubernetes Secrets and then install using the
[`values-gcp.yaml`](../deploy/helm/artifact/values-gcp.yaml) example profile:

```bash
BUCKET=$(terraform output -raw storage_bucket)
CONN=$(terraform output -raw cloudsql_connection_name)
WI_SA=$(terraform output -raw workload_identity_sa_email)

# Pre-create secrets (run once; never put DSNs or OIDC secrets in values files)
kubectl create secret generic artifact-db \
  --from-literal=database-url="postgres://artifact:<password>@/artifact?host=/cloudsql/$CONN"
kubectl create secret generic artifact-oidc \
  --from-literal=client-secret=<your-okta-client-secret>

helm install artifact ./deploy/helm/artifact/ \
  -f deploy/helm/artifact/values-gcp.yaml \
  --set config.domain=artifact.corp.example.com \
  --set config.storage.bucket="$BUCKET" \
  --set serviceAccount.annotations."iam\.gke\.io/gcp-service-account"="$WI_SA"
```

`values-gcp.yaml` sets `config.storage.driver: gcs`, enables Workload Identity on the
pod's ServiceAccount, and pulls the database URL and OIDC client secret from the Secrets
created above. No S3 access keys are required — the GCS driver authenticates via
Application Default Credentials through the Workload Identity binding.

#### Key `values-gcp.yaml` fields

| Field | Purpose |
|-------|---------|
| `config.storage.driver: gcs` | Use GCS instead of S3/MinIO |
| `config.storage.bucket` | Bucket name from `storage_bucket` terraform output |
| `serviceAccount.create: true` | Create the K8s ServiceAccount |
| `serviceAccount.annotations["iam.gke.io/gcp-service-account"]` | Bind to GCP SA for Workload Identity |
| `serviceAccount.automountServiceAccountToken: true` | Required for WI token projection |
| `externalDatabase.secretName / secretKey` | Read Cloud SQL DSN from a Kubernetes Secret |
| `oidcSecret.secretName / secretKey` | Read Okta client secret from a Kubernetes Secret |

---

## Provision AWS infrastructure with Terraform

`deploy/terraform/aws/main.tf` is likewise a **starter template**. It provisions an S3
bucket and an RDS Postgres 16 instance (`db.t3.micro`). A comment in the file points you
to add an ALB with an OIDC auth action and deploy Artifact on ECS, EC2, or a Kubernetes
cluster.

**What the starter template does not wire:** an Application Load Balancer, OIDC
authentication on the ALB, a wildcard TLS certificate, security groups, or VPC configuration.
Add these before going to production.

```bash
cd deploy/terraform/aws
terraform init
terraform apply -var db_password=<secret>
```

After `apply`, point your `artifact.yaml` at the RDS DSN and S3 bucket:

```yaml
storage:
  driver: s3
  bucket: artifact-sites

database:
  driver: postgres
  url_env: ARTIFACT_DATABASE_URL
```

---

## Health endpoints

Artifact exposes two HTTP endpoints for use by load balancers and Kubernetes probes:

| Endpoint | Purpose | Checks |
|----------|---------|--------|
| `GET /healthz` | **Liveness** — is the process alive? | None; always returns `200 ok` as long as the Go runtime is up. |
| `GET /readyz` | **Readiness** — is the process ready to serve traffic? | DB ping with a 2-second timeout. Returns `200 ok` when the database is reachable; `503 Service Unavailable` otherwise. |

Both responses include an `X-Artifact-Version` header on success.

Use them as separate probe targets to avoid the Cloud-SQL-not-ready race on cold starts and rolling updates:

```yaml
# Kubernetes / Helm (deployment.yaml)
livenessProbe:
  httpGet:
    path: /healthz
    port: 8443

readinessProbe:
  httpGet:
    path: /readyz
    port: 8443
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3
```

The Helm chart in `deploy/helm/artifact/` is already wired this way. When deploying outside Kubernetes (e.g. behind an ALB or a Pomerium gateway), point your health-check at `/readyz` and your keep-alive check at `/healthz`.

---

## Auth modes

Every production deployment needs a real auth mode. The two main options:

- **OIDC** — Artifact handles the login/callback flow itself. Works with Okta, Entra ID,
  Google Workspace, or any OIDC provider. See [Auth — Okta](auth-okta.md) for a step-by-step
  example.
- **Header-trust** — run Artifact behind an identity proxy (Pomerium, oauth2-proxy, GCP IAP)
  and let the proxy stamp identity headers. See
  [Auth — header-trust](auth-header-trust.md). The recipe in
  `deploy/recipes/pomerium.md` shows a complete Pomerium config.

Never use `auth.mode: dev` in production. It signs every request in as `dev@localhost` with
no authentication.
