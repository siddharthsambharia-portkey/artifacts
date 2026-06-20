# 014: First-class JSON-key / access-key storage fallback toggle

> Type: AFK · Priority: P1 · Effort: M · Triage: ready-for-agent
> Glossary: Operator, Site, Deployment Profile (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md`.

## Parent

010 — Make Artifact installable by a low-privilege champion (the two-stage adoption on-ramp).

## What to build

The chart only reads cloud storage via Workload Identity / IRSA, which a champion cannot create
(`setIamPolicy` denied). Add a first-class chart toggle that mounts a cloud-storage credential from a
Kubernetes Secret — a GCS JSON key or S3/compatible access keys — so the pod can read and write storage
with no IAM-policy admin. This removes the manual, drift-prone `kubectl patch` the champion resorts to
today, and the result must survive `helm upgrade`.

End-to-end behavior:

- A values toggle selects key-based storage credentials sourced from a Kubernetes Secret reference (never
  plaintext in values).
- The key-based path and the Workload Identity annotation path are mutually exclusive; selecting the
  fallback injects no WI annotation.
- The configuration persists across `helm upgrade` (no out-of-band patching).

## Acceptance criteria

- [ ] A storage-key fallback toggle mounts the GCS-key / S3-key credential from a Kubernetes Secret
- [ ] Enabling the fallback injects no Workload Identity annotation (the two paths are mutually exclusive)
- [ ] Credentials come from a Secret reference, not plaintext values (consistent with `externalS3` /
      `oidcSecret` / `headerTrustSecret`)
- [ ] `render_test.go` asserts the Secret-mounted credential renders and the WI annotation is absent when
      the fallback is on; the WI path still renders when it is off
- [ ] `helm lint` / `helm template` pass with the toggle on and off
- [ ] Docs: `docs/self-hosting.md` documents the low-power storage fallback and that it survives
      `helm upgrade`
- [ ] `go build ./...` and `go test ./...` green

## Blocked by

None — can start immediately.
