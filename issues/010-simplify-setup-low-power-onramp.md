# 010: Make Artifact installable by a low-privilege champion — the two-stage adoption on-ramp

> Type: AFK · Priority: P0/P1 · Effort: L · Triage: ready-for-agent
> Glossary: Operator, Builder, Site, Deployment Profile, Native OIDC, Header-trust, Identity proxy,
> Trust mode, Governed mode (see `CONTEXT.md`).
> Relevant decisions: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md` (GCP + Okta is the one
> guaranteed profile), `docs/wildcard-tls-and-routing-options.md` (subdomains are load-bearing; unblock
> at the infra layer). Related issues: 001, 002, 005, 007, 008, 009.

## Problem Statement

Artifact spreads bottom-up: one curious engineer trials it, builds something, colleagues see it, and
*then* a platform team adopts it officially. That first engineer — the **champion** — is almost always
a low-privilege Operator. In the real deployment that produced this record, the champion could not
register an Okta application (no IdP admin) and could not create a Workload Identity binding
(`setIamPolicy` denied), so they fell back to header-trust and a long-lived storage key. They then hit
a wall of undocumented, non-batteries-included glue:

- There is **no published release** at all (0 git tags), so the one-line install silently falls back to
  building from Go source, the `ghcr…:v0.1.0` image referenced by Helm does not exist, and the Dockerfile
  pins an older Go than `go.mod` requires. The very first step of every path is broken.
- Header-trust "works" in code, but the Helm chart ships **no proxy-auth secret-injection middleware and
  no wildcard routing** for the ingress controllers a champion actually inherits (nginx, Traefik). The
  champion must hand-write `kubectl patch`es that `helm upgrade` then wipes — and a missing
  `X-Artifact-Proxy-Auth` injection causes a redirect loop.
- The chart only supports cloud storage via Workload Identity / IRSA, which the champion **cannot create**.
  There is no first-class **JSON-key / access-key storage fallback** toggle, forcing manual, drift-prone
  secret mounts.
- The GCP profile uses a Cloud SQL **socket DSN** but the chart **bundles no Cloud SQL Auth Proxy sidecar**,
  so the documented profile cannot actually connect to the database out of the box.
- The header-trust docs recommend `pass_request_headers`, which **does nothing** in oauth2-proxy's
  ForwardAuth mode, and the Traefik v3 host-routing syntax is undocumented.

The champion is the adoption engine. If that person cannot succeed end-to-end in an afternoon, the tool
never reaches the platform team that would harden and scale it. Today they cannot.

## Solution

Treat self-hosting as a **two-stage journey** and make both stages first-class in the same chart — a few
toggles, not two products and not a wall of 20 options:

- **Stage 1 — the low-power on-ramp (default).** A champion with one cloud project, **no IdP admin and no
  IAM-policy admin**, can stand up a working multi-site instance behind their company's existing SSO proxy
  in an afternoon. The chart ships the ForwardAuth proxy-auth middleware and wildcard routing for nginx and
  Traefik, a JSON-key / access-key storage fallback toggle, and a Cloud SQL Auth Proxy sidecar toggle —
  all surviving `helm upgrade`. A published release means the install and the chart image just work.
- **Stage 2 — the hardened upgrade (clearly labelled).** A platform team flips a few values to move from
  JSON-key → Workload Identity/IRSA, header-trust → Native OIDC, and single → multi-replica. Presented as a
  documented "Harden for production" checklist, not a separate install.

What the **Builder** sees never changes: Sites, SSO login, db/kv/files/ai/warehouse/ws/notify, the SDK and
the agent skill are identical across both stages. The entire trade is behind the scenes (credential
lifetime, upgrade-safety, HA, who owns the login).

Subdomains stay load-bearing (per `docs/wildcard-tls-and-routing-options.md`); path-based hosting is **not**
introduced. The cloud pattern is GCP-first but expressed so AWS/Azure differ only in service names
(GCS↔S3↔Blob, Workload Identity↔IRSA↔Managed Identity, Cloud SQL Auth Proxy↔RDS IAM↔equivalent).

## User Stories

1. As a champion Operator, I want a one-line install that fetches a real released binary, so that I am not
   silently forced to have a Go toolchain and build from source.
2. As a champion Operator, I want the Helm `image.tag` to reference a container image that actually exists,
   so that `helm install` pulls a running Artifact instead of failing to pull.
3. As an Operator, I want the published container image built with the Go version `go.mod` declares, so that
   the release image is consistent with what the project compiles against.
4. As a maintainer, I want pushing a `vX.Y.Z` tag to produce binaries for the supported OS/arch pairs and a
   published multi-arch image, so that every downstream install path resolves.
5. As a champion Operator without IdP admin, I want header-trust to be the default low-power on-ramp, so that
   I can ride my company's existing SSO proxy without registering a new IdP application.
6. As a champion Operator, I want the chart to render the proxy-auth secret-injection middleware for my
   ingress controller, so that every request reaching Artifact carries `X-Artifact-Proxy-Auth` and I do not
   hit the redirect loop.
7. As a champion Operator on nginx, I want batteries-included wildcard routing and middleware, so that
   `*.<domain>` and `admin.<domain>` reach Artifact without hand-written annotations.
8. As a champion Operator on Traefik, I want the chart to emit the correct v3 host-routing and a ForwardAuth
   middleware, so that subdomain routing and the proxy-auth handshake work without me reverse-engineering
   Traefik v3 syntax.
9. As an Operator, I want the ingress glue gated by a single `ingress.controller: nginx|traefik` choice, so
   that I pick my inherited controller and the chart does the rest.
10. As a champion Operator who cannot create a Workload Identity binding, I want a first-class JSON-key /
    access-key storage fallback toggle, so that the pod can read/write cloud storage using a mounted Secret.
11. As an Operator, I want the storage key to come from a Kubernetes Secret reference (never plaintext in
    values), so that I follow the project's secrets model.
12. As a champion Operator, I want any storage-credential choice to survive `helm upgrade`, so that I never
    have to re-apply a `kubectl patch` that an upgrade silently wipes.
13. As an Operator following the GCP profile, I want a Cloud SQL Auth Proxy sidecar toggle, so that the
    socket DSN in `values-gcp.yaml` can actually connect to the database.
14. As an Operator, I want the sidecar to be off by default and to use the same identity model as the rest of
    the pod, so that enabling it does not regress the existing direct-DSN path.
15. As an Operator weighing the trade-off, I want a clear, calibrated statement of what low-power gives up
    (long-lived credential, single replica, riding a shared proxy) and what it does not (outsiders are still
    bounced by SSO), so that I can decide whether it is acceptable for my data sensitivity.
16. As a platform-team Operator, I want a "Harden for production" checklist, so that I can upgrade JSON-key →
    Workload Identity/IRSA, header-trust → Native OIDC, and single → multi-replica as a documented set of
    value flips rather than a re-install.
17. As a platform-team Operator, I want native OIDC and header-trust documented as co-equal supported modes
    (OIDC the Stage-2 destination, header-trust the Stage-1 on-ramp), so that neither feels second-class.
18. As an Operator using a cloud-native identity layer (GCP IAP / AWS ALB OIDC / Pomerium), I want a one-page
    "header contract" doc, so that I can wire identity injection myself without chart support for my proxy.
19. As an Operator reading the header-trust docs, I want the broken `pass_request_headers` guidance replaced
    with the working middleware approach, so that I do not configure something that silently does nothing.
20. As an Operator on Traefik, I want the v3 `HostRegexp` wildcard syntax documented, so that I am not
    debugging v2-vs-v3 routing differences during setup.
21. As an Operator on AWS or Azure, I want the docs to map the GCP pattern onto S3/IRSA/RDS and Blob/Managed
    Identity, so that the low-power fallback and hardened upgrade exist on my cloud too.
22. As a security reviewer, I want the residual risks of low-power mode named explicitly (header spoofing
    inside a shared cluster, closed by the proxy-auth shared secret; broad blast radius of a leaked storage
    key), so that I can sign off on it for a trial and know exactly what Stage 2 fixes.
23. As an Operator, I want starter templates audited for hardcoded passwords (e.g. `changeme-rotate-immediately`)
    and namespace typos, so that I do not ship a footgun by copy-paste.
24. As a Builder, I want nothing about my deploy/SDK/login experience to change regardless of which stage the
    Operator chose, so that sites behave identically on a champion trial and a hardened rollout.

## Implementation Decisions

- **Modules touched:** the release pipeline (release workflow + GoReleaser config + Dockerfile + `install.sh`),
  the Helm chart (`deploy/helm/artifact/`), the GCP profile values, the header-trust and self-hosting docs,
  and the terraform/recipe starters. No change to the Builder-facing SDK, CLI deploy path, or `SiteFromHost`
  site-derivation invariant.
- **P0 — publish a release (universal unblocker).** Cut a `vX.Y.Z` tag so the tag-triggered release workflow
  produces binaries and a published container image. Align the Dockerfile and the release workflow's Go
  toolchain with the version declared in `go.mod` (currently 1.25.x; both pin 1.23 today). Reconcile the
  chart's `image.tag` and `install.sh` so the advertised version resolves to a real artifact. This is the
  single biggest win and must land first; the rest is untestable end-to-end without it.
- **Header-trust stays the documented Stage-1 default for low-power; Native OIDC stays the Stage-2/guaranteed
  default per ADR 0001.** Neither mode is demoted. The chart already injects the proxy *secret* and renders the
  `header_trust` config block (issue 007); this issue adds the missing **ingress glue**.
- **New chart input `ingress.controller`** with values `nginx | traefik` (plus an explicit "none/other" escape
  hatch for the header-contract path). When set, the chart renders, for that controller: (a) the ForwardAuth /
  proxy-auth middleware that injects `X-Artifact-Proxy-Auth` on every forwarded request, and (b) wildcard
  routing for `*.<domain>` and `admin.<domain>`. nginx and Traefik are the batteries-included set; everything
  else uses the header-contract doc.
- **JSON-key / access-key storage fallback becomes a first-class toggle** that mounts a cloud-storage
  credential from a Kubernetes Secret (GCS JSON key or S3/compatible access keys) when Workload Identity / IRSA
  is unavailable. Mutually exclusive with the Workload Identity annotation path; selecting it must not require
  any IAM-policy admin. Credentials come from a Secret reference, never plaintext values, consistent with the
  existing `externalS3` / `oidcSecret` / `headerTrustSecret` pattern. The result must persist across
  `helm upgrade` (no out-of-band `kubectl patch`).
- **Cloud SQL Auth Proxy sidecar toggle**, off by default, that runs alongside the app container so the GCP
  profile's socket DSN connects. It uses the same pod identity as storage (Workload Identity when hardened, or
  the mounted key in low-power). Enabling it must not change the existing direct-DSN/external-database path.
- **Two-stage framing in config and docs.** The low-power combination (header-trust + key-based storage +
  single replica + bundled ingress glue) is the default on-ramp; the hardened combination (Native OIDC +
  Workload Identity/IRSA + multi-replica via NATS) is a clearly-labelled upgrade. Same chart, a few toggles.
- **Docs corrections:** replace the non-functional `pass_request_headers` guidance in the header-trust doc with
  the middleware approach; document Traefik v3 `HostRegexp` wildcard syntax; add a "Harden for production"
  checklist; add a one-page header-contract doc for IAP / ALB OIDC / Pomerium; extend the GCP pattern to
  AWS/Azure equivalents in prose.
- **Subdomains are retained; no path-based mode** is introduced (per `docs/wildcard-tls-and-routing-options.md`).
- **Footgun sweep:** parameterize/secret any hardcoded passwords (e.g. `changeme-rotate-immediately`) and fix
  namespace typos in starter templates and recipes.
- **Cross-cloud:** the JSON-key/access-key fallback and the WI/IRSA hardened upgrade must each exist on GCP and
  be documented for AWS (S3 + IRSA + RDS) and Azure (Blob + Managed Identity); only service names differ.

## Testing Decisions

A good test asserts **external, observable behavior** — what the chart renders, what the release produces, what
a doc link resolves to — not internal template structure. Prior art for every seam already exists in this repo.

- **Helm chart (primary seam): `deploy/helm/artifact/render_test.go`** — Go tests that shell out to
  `helm template` and assert on the rendered manifest stream (skipping when `helm` is absent). This is the
  highest existing seam and already covers the GCS/WI, header-trust-secret, and cert paths (issues 007/008).
  Add behavior-level cases:
  - With `ingress.controller=nginx` (and `=traefik`), the render contains the proxy-auth middleware and the
    `*.<domain>` + `admin.<domain>` routing; with the controller unset/none, neither renders.
  - With the storage-key fallback enabled, the render mounts the storage credential from a Secret (via
    `secretKeyRef`/volume) and injects **no** Workload Identity annotation; the two paths are mutually exclusive.
  - With the Cloud SQL sidecar enabled, the render adds the proxy container; disabled (default) leaves the pod
    single-container and the existing external-database path unchanged (regression guard, mirroring
    `TestDefaultProfileStillRendersS3AndOIDC`).
  - `helm lint` / `helm template` pass with each new toggle on and off.
- **Release pipeline:** `goreleaser check` validates the config; a local `goreleaser release --snapshot --clean`
  (or equivalent) proves binaries build; a `docker build` against the Dockerfile proves the image builds on the
  `go.mod` Go version. A guard asserts the chart's `image.tag` corresponds to a real released tag (no dangling
  `v0.1.0`). Tag-triggered CI itself is verified by cutting the first real tag.
- **Terraform/recipes:** `terraform validate` (and `plan` given a project id) on any touched starter, matching
  issue 005's prior art.
- **Docs:** relative-link resolution and a content check that the header-trust doc no longer mentions
  `pass_request_headers` as the mechanism, mirroring the docs-only verification style of issues 001/002.
- **Build/code:** `go build ./...` and `go test ./...` stay green for any code touched (e.g. config validation).

## Out of Scope

- **Path-based hosting (`<domain>/<site>`).** Explicitly rejected; subdomains are the security boundary
  (`docs/wildcard-tls-and-routing-options.md`). Not built here.
- **Bundling our own identity proxy.** A bundled proxy needs its own IdP app, which re-creates the admin wall
  for the champion. Out.
- **Chart support for the long-tail proxies (GCP IAP / AWS ALB OIDC / Pomerium).** Covered by the one-page
  header-contract doc, not by rendered chart glue. Only nginx + Traefik get batteries-included middleware.
- **A guided installer / preflight (`artifact install`).** Deferred (see Further Notes) — this issue invests in
  a great Helm chart + release first.
- **New SDK/Builder features.** The Builder experience is intentionally unchanged.
- **Full AWS/Azure chart parity in code.** This issue ships the GCP toggles and *documents* the AWS/Azure
  equivalents; building IRSA/Managed-Identity chart paths end-to-end can follow.
- **Wildcard-TLS certificate issuance** beyond what issue 008 already shipped (the cert-manager `Certificate`
  exists, flag-gated). Cert-acquisition strategy stays in `docs/wildcard-tls-and-routing-options.md`.

## Further Notes

- **Is low-power actually insecure? Calibrated answer.** Outsiders are still bounced — the proxy rejects anyone
  without an SSO session. The residual risks are internal/accidental: (1) header spoofing inside a shared
  cluster, closed by the `X-Artifact-Proxy-Auth` shared secret; (2) a leaked storage key grants read+write to
  *all* Sites' files (blast radius = deface/delete everyone's sites). Verdict: fine for a trial / "any employee
  can see it" internal tool; a problem only with sensitive data, fine-grained access needs, or a company-wide
  rollout going through security review — exactly what the Stage-2 hardening fixes.
- **Two Operators, not two products.** The "champion" (limited cloud access, no IdP/IAM admin) and the
  "platform team" (full admin) are both Operators in the `CONTEXT.md` sense, at different privilege levels.
  Stage 1 feeds Stage 2; design the low-power path as the default on-ramp and the hardened path as the upgrade.
- **Overlap with existing issues:** 007 (chart GCP/header-trust *secret*) and 008 (wildcard TLS cert) have
  landed and are the foundation this builds on; 001 (header-trust groups), 002 (auth-mode selection guide), and
  009 (`/readyz`) are complementary. This issue adds the *missing low-power glue* (ingress middleware + routing,
  key fallback, Cloud SQL sidecar) plus the release and the hardening narrative.
- **Open / deferred:** the exact "champion succeeds in an afternoon" shortest-path step list; whether a guided
  installer/preflight (`artifact install`) earns its keep over a great chart; AWS/Azure parity timing.
- This PRD was synthesized from deployment lessons (records 0001–0005 / LR-0002…LR-0005) gathered while
  deploying `artifacts.airs-demo.com`.
