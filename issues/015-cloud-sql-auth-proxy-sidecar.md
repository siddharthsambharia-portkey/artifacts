# 015: Cloud SQL Auth Proxy sidecar toggle for the GCP profile

> Type: AFK · Priority: P1 · Effort: M · Triage: ready-for-agent
> Glossary: Operator, Deployment Profile (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md` (Cloud SQL Postgres is part
> of the guaranteed profile).

## Parent

010 — Make Artifact installable by a low-privilege champion (the two-stage adoption on-ramp).

## What to build

The GCP profile uses a Cloud SQL **socket DSN** (`host=/cloudsql/<connection>`), but the chart bundles no
Cloud SQL Auth Proxy, so the documented profile cannot actually connect to the database. Add an
off-by-default sidecar toggle that runs the Cloud SQL Auth Proxy alongside the app container, using the same
pod identity as storage (Workload Identity when hardened, or the mounted key in the low-power fallback from
issue 014). Enabling it must not change the existing direct-DSN / external-database path.

End-to-end behavior:

- A values toggle adds the Cloud SQL Auth Proxy sidecar so the socket DSN connects.
- The sidecar is off by default; with it off the pod is single-container and the existing external-database
  path is unchanged.
- The sidecar uses the pod's existing identity (no separate credential mechanism introduced).

## Acceptance criteria

- [ ] A values toggle renders the Cloud SQL Auth Proxy sidecar container
- [ ] The sidecar is off by default; the default render stays single-container (regression guard, mirroring
      `TestDefaultProfileStillRendersS3AndOIDC`)
- [ ] The sidecar uses the pod's existing identity (Workload Identity or the issue-014 key mount)
- [ ] `render_test.go` covers the sidecar-on render and the off-by-default render
- [ ] `helm lint` / `helm template` pass with the toggle on and off
- [ ] Docs: `docs/self-hosting.md` GCP section documents enabling the sidecar for the socket DSN
- [ ] `go build ./...` and `go test ./...` green

## Blocked by

None — can start immediately. (Soft: 014 — shares the pod-identity model for the low-power path; reconcile
field names if both are in flight.)
