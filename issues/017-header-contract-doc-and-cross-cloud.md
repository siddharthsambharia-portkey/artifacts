# 017: Header-contract doc for long-tail proxies + AWS/Azure mapping

> Type: AFK · Priority: P2 · Effort: S · Triage: ready-for-agent
> Glossary: Operator, Header-trust, Identity proxy, Deployment Profile (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md`.

## Parent

010 — Make Artifact installable by a low-privilege champion (the two-stage adoption on-ramp).

## What to build

The chart ships batteries-included glue only for nginx and Traefik (issues 012–013). For the cloud-native
tail that injects identity itself — GCP IAP, AWS ALB OIDC, Pomerium — provide a single one-page "header
contract" doc: which headers Artifact expects and the shared-secret (`X-Artifact-Proxy-Auth`) requirement,
so an Operator can wire identity injection themselves without chart support for their proxy.

Also map the GCP pattern onto the other clouds in prose, since the model is cloud-interoperable and only
service names change: storage (GCS ↔ S3 ↔ Blob), identity (Workload Identity ↔ IRSA ↔ Managed Identity), and
managed database (Cloud SQL Auth Proxy ↔ RDS/IAM ↔ equivalent). State that the low-power key/access-key
fallback and the hardened WI/IRSA upgrade conceptually exist on each.

Docs-only; no behavior change.

## Acceptance criteria

- [ ] A one-page header-contract doc lists the identity headers Artifact reads and the
      `X-Artifact-Proxy-Auth` shared-secret requirement, with setup notes for IAP / ALB OIDC / Pomerium
- [ ] The GCP pattern is mapped onto AWS (S3 / IRSA / RDS) and Azure (Blob / Managed Identity) in prose
- [ ] The doc is cross-linked from `docs/auth-header-trust.md` and `docs/self-hosting.md`
- [ ] All relative doc links resolve
- [ ] No code behavior change (docs-only)

## Blocked by

None — can start immediately. (Soft: 012/013 — reference the chart-supported controllers vs. the
header-contract path consistently.)
