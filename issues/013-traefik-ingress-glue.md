# 013: Batteries-included Traefik ingress glue (ForwardAuth + v3 wildcard routing)

> Type: AFK · Priority: P1 · Effort: M · Triage: ready-for-agent
> Glossary: Operator, Site, Header-trust, Identity proxy (see `CONTEXT.md`).
> Relevant decisions: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md`,
> `docs/wildcard-tls-and-routing-options.md`.

## Parent

010 — Make Artifact installable by a low-privilege champion (the two-stage adoption on-ramp).

## What to build

Extend the `ingress.controller` input introduced for nginx (issue 012) to cover Traefik — the other
controller a champion commonly inherits. With `ingress.controller: traefik`, the chart renders a
ForwardAuth / proxy-auth middleware that injects `X-Artifact-Proxy-Auth`, plus wildcard routing for
`*.<domain>` and `admin.<domain>` using Traefik v3 `HostRegexp` syntax. The v3 routing syntax differs from
v2 and is undocumented today.

End-to-end behavior:

- With `ingress.controller: traefik`, the chart renders the Traefik middleware and v3 `HostRegexp` wildcard
  routing for the site and admin hosts.
- The existing nginx and controller-off paths are unchanged.
- The Traefik v3 wildcard host syntax is documented.

## Acceptance criteria

- [ ] `ingress.controller: traefik` renders the ForwardAuth proxy-auth middleware and v3 `HostRegexp`
      wildcard routing for `*.<domain>` + `admin.<domain>`
- [ ] The nginx path (issue 012) and the controller-off path still render unchanged (no regression)
- [ ] `render_test.go` covers the Traefik-on case
- [ ] `helm lint` / `helm template` pass with the Traefik toggle on and off
- [ ] Docs: Traefik v3 `HostRegexp` wildcard routing documented (in `docs/auth-header-trust.md` /
      `docs/self-hosting.md` or `docs/wildcard-tls-and-routing-options.md`)
- [ ] `go build ./...` and `go test ./...` green

## Blocked by

- 012 — reuses the `ingress.controller` input and the render-test harness it introduces.
