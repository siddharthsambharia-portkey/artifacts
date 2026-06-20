# 011: Publish a real release so every install path resolves

> Type: AFK · Priority: P0 · Effort: M · Triage: ready-for-agent
> Glossary: Operator, Deployment Profile (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md`.

## Parent

010 — Make Artifact installable by a low-privilege champion (the two-stage adoption on-ramp).

## What to build

There is no published release (0 git tags), so the first step of every path is broken: the one-line
installer silently falls back to building from Go source, the container image the Helm chart references
does not exist, and the advertised version resolves to nothing. Cut a real `vX.Y.Z` release and make the
tag-triggered pipeline produce binaries plus a published container image, then reconcile every consumer so
the advertised version points at a real artifact.

This is the universal unblocker — nothing else in 010 is verifiable end-to-end until it lands.

End-to-end behavior:

- Pushing a `vX.Y.Z` tag produces downloadable binaries for the supported OS/arch pairs and a published
  multi-arch container image at the expected registry path.
- The release toolchain and the container build use the Go version declared in `go.mod` (currently 1.25.x),
  not an older pin.
- The one-line install fetches the released binary instead of silently building from source.
- The Helm chart's image tag references the real published image (no dangling `v0.1.0`).

## Acceptance criteria

- [ ] A `vX.Y.Z` tag triggers a release that produces binaries and a published container image
- [ ] The container build and the release workflow use the Go version `go.mod` declares
- [ ] `install.sh` fetches the released binary for a real version (no silent source-build fallback when a
      release exists)
- [ ] The Helm chart `image.tag` resolves to a published image
- [ ] `goreleaser check` passes and a local snapshot build produces binaries
- [ ] A `docker build` against the Dockerfile succeeds on the `go.mod` Go version
- [ ] Docs: the quickstart / self-hosting install steps reference the real release flow and a valid version
- [ ] `go build ./...` and `go test ./...` green

## Blocked by

None — can start immediately.
