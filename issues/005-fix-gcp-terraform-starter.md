# 005: Fix the GCP Terraform starter (HCL syntax bug + wire the resources)

> Type: AFK · Priority: P1 · Effort: S
> Glossary: Operator, deployment profile (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md` (GCP + Okta is the
> guaranteed profile — the starter terraform must at least plan).

## What to build

`deploy/terraform/gcp/main.tf` does not pass `terraform validate` and provisions resources the app
is never told about. Make the starter actually plan, and connect what it creates to the running
Artifact instance so an Operator following the guaranteed GCP + Okta profile gets a working baseline
rather than orphaned infrastructure.

Two distinct problems:

- **HCL syntax error.** `variable "region" { type = string default = "us-central1" }` puts two
  attributes on one line with no separator; `terraform validate` rejects it. Each attribute needs
  its own line (or a comma). The `project_id`/`region` variables should be valid HCL.
- **Orphaned resources.** It creates a GCS bucket and a Cloud SQL Postgres instance but wires
  neither into Artifact: no service account for the app to read the bucket (Workload Identity), no
  Cloud SQL connection name surfaced, no load balancer, no outputs. Expose the pieces the deploy
  needs as `output`s (bucket name, Cloud SQL connection name / instance, and a service-account email
  if one is created) so the Helm chart (issue 007) can consume them, and add a short comment block
  pointing at the Helm values they map to. A full LB/VPC build-out stays out of scope — it remains a
  starter — but it must be honest about being a starter and must plan cleanly.

Keep the "this is a starter, customize for your VPC" framing; just make it correct.

## Acceptance criteria

- [ ] `terraform validate` passes on `deploy/terraform/gcp/` (the `region` variable and all blocks
      are valid HCL)
- [ ] `terraform plan` runs without syntax/config errors (given a `project_id`)
- [ ] Bucket name, Cloud SQL connection name/instance, and any created service-account email are
      exposed as `output`s
- [ ] A comment maps each output to the Helm value it feeds (storage bucket, database URL, WI SA)
- [ ] `docs/self-hosting.md` GCP section notes what the starter does and does not wire (kept honest)

## Blocked by

None — can start immediately.
