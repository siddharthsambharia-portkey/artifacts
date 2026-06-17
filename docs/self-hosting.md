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
| `config.auth.mode` | `oidc` | Auth mode |
| `config.storage.driver` | `s3` | Storage driver |
| `config.storage.bucket` | `artifact-sites` | S3 bucket name |
| `config.database.driver` | `postgres` | Database driver |
| `config.governance.mode` | `trust` | Governance mode |
| `externalDatabase.enabled` | `true` | Mount DB URL from `externalDatabase.url` |
| `externalDatabase.url` | — | Postgres DSN |
| `externalS3.enabled` | `true` | Pass S3 credentials as env vars |
| `externalS3.accessKey` | `""` | S3 access key ID |
| `externalS3.secretKey` | `""` | S3 secret (stored in a Kubernetes Secret) |
| `nats.enabled` | `false` | Enable NATS adapter for multi-replica realtime |
| `nats.url` | `nats://nats:4222` | NATS server URL |
| `resources.limits` | 2 CPU / 4 Gi | Resource limits |
| `resources.requests` | 500m / 1 Gi | Resource requests |

### Enable multi-replica realtime

A single replica uses an in-process WebSocket hub. Realtime events (DB subscriptions, presence) are not fanned out across pods. To run more than one replica, deploy a NATS server and set `nats.enabled: true`. The hub then uses NATS as a pub/sub bus between replicas.

> **Ingress note:** wildcard subdomain routing (`*.<domain>`) requires your Ingress
> controller to support wildcard hosts. nginx-ingress does. Configure a wildcard TLS
> certificate (e.g. via cert-manager) at the ingress layer to match.

---

## Provision GCP infrastructure with Terraform

`deploy/terraform/gcp/main.tf` is a **starter template** — not a production-ready module.
It provisions a GCS bucket for site storage and a Cloud SQL Postgres 16 instance. A comment
in the file points you to deploy Artifact on a VM or GKE cluster using the Helm chart,
pointing at those resources.

**What the starter template does not wire:** an internal Application Load Balancer, Identity
Aware Proxy (IAP), a wildcard TLS certificate, or VPC peering. Add these before going to
production.

```bash
cd deploy/terraform/gcp
terraform init
terraform apply -var project_id=my-corp-project
```

After `apply`, point your `artifact.yaml` at the Cloud SQL DSN and GCS bucket:

```yaml
storage:
  driver: gcs
  bucket: my-corp-project-artifact-sites

database:
  driver: postgres
  url_env: ARTIFACT_DATABASE_URL
```

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
