# 012: Batteries-included nginx ingress glue for header-trust (middleware + wildcard routing)

> Type: AFK · Priority: P1 · Effort: M · Triage: ready-for-agent
> Glossary: Operator, Site, Header-trust, Identity proxy (see `CONTEXT.md`).
> Relevant decisions: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md`,
> `docs/wildcard-tls-and-routing-options.md`.

## Parent

010 — Make Artifact installable by a low-privilege champion (the two-stage adoption on-ramp).

## What to build

A champion Operator without IdP admin rides their existing SSO proxy in header-trust mode, but the Helm
chart ships no glue for the ingress controller they inherit, so they hand-write `kubectl patch`es that
`helm upgrade` then wipes — and a missing `X-Artifact-Proxy-Auth` injection causes a redirect loop.

Introduce a chart input `ingress.controller` (this slice implements `nginx`) that, when set, renders the
proxy-auth secret-injection middleware and the wildcard routing for `*.<domain>` and `admin.<domain>` so
header-trust works out of the box on nginx and survives upgrades. This slice also fixes the header-trust
doc, which currently recommends `pass_request_headers` — a no-op in ForwardAuth mode.

End-to-end behavior:

- With `ingress.controller: nginx`, the chart renders the middleware that injects `X-Artifact-Proxy-Auth`
  on every forwarded request, plus routing for the wildcard and admin hosts.
- With the controller unset, none of this renders (no regression to the existing path).
- The header-trust doc describes the working middleware approach instead of `pass_request_headers`.

## Acceptance criteria

- [ ] `ingress.controller: nginx` renders the proxy-auth middleware injecting `X-Artifact-Proxy-Auth` and
      `*.<domain>` + `admin.<domain>` routing
- [ ] With the controller unset/none, neither the middleware nor the wildcard routing renders
- [ ] `render_test.go` covers both the nginx-on and the controller-off cases
- [ ] `helm lint` / `helm template` pass with the toggle on and off
- [ ] Docs: `docs/auth-header-trust.md` replaces `pass_request_headers` guidance with the middleware
      approach; `docs/self-hosting.md` documents the nginx low-power path
- [ ] `go build ./...` and `go test ./...` green

## Blocked by

None — can start immediately.
