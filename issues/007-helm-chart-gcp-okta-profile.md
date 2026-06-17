# 007: Helm chart can express the guaranteed GCP + Okta profile

> Type: AFK · Priority: P1 · Effort: M
> Glossary: Operator, deployment profile, Native OIDC, header-trust (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md` (GCP + Okta is the
> one profile guaranteed end-to-end).

## What to build

The Helm chart only models an S3/MinIO + native-OIDC deployment, so it **cannot express the profile
the project guarantees** (GCP + Okta: `storage.driver: gcs`, Cloud SQL Postgres, Workload Identity).
`values.yaml` hardcodes `storage.driver: s3` with `endpoint: minio...` and an `externalS3` block, and
has no way to configure the `gcs` driver, Workload Identity, or header-trust. Bring the chart to
parity with the documented config surface so an Operator can `helm install` the guaranteed profile.

Add to the chart:

- **`gcs` storage driver** — `config.storage.driver: gcs` with `bucket`, no S3 access/secret keys
  required. The S3 block becomes one option among several, not the only one.
- **Workload Identity** — a `serviceAccount` block that can set the
  `iam.gke.io/gcp-service-account` annotation (and `automountServiceAccountToken` as needed) so the
  pod reads GCS via ADC/WI with no JSON key mounted. (The GCS driver already uses ADC — this just
  lets the chart wire the K8s SA → GCP SA binding.)
- **header-trust auth** — `config.auth.mode: header-trust` with `email_header` / `name_header` /
  `groups_header` / `proxy_secret_env`, and a way to inject the `ARTIFACT_PROXY_SECRET` env from a
  Secret. Today only `auth.mode: oidc` is representable.
- Keep S3/MinIO + OIDC working as-is (don't regress the existing path).

This is the consumer side of issue 005's terraform outputs (bucket name, Cloud SQL connection,
WI SA email).

## Acceptance criteria

- [ ] `values.yaml` supports `storage.driver: gcs` (with `bucket`) and `helm template` renders a
      valid deployment for it with no S3 keys required
- [ ] A `serviceAccount` block can set the Workload Identity annotation; `helm template` shows it on
      the pod's ServiceAccount
- [ ] `auth.mode: header-trust` is representable with all four header-trust fields, and the proxy
      secret is injected from a Secret (not plaintext in values)
- [ ] Existing S3/MinIO + native-OIDC path still renders unchanged (no regression)
- [ ] `helm lint` passes; a documented `values-gcp.yaml` example (or a docs snippet) shows the full
      GCP + Okta profile
- [ ] `docs/self-hosting.md` references the GCP profile values

## Blocked by

- Soft: 005 (terraform outputs feed these values). Can proceed in parallel; reconcile field names
  when both land.
