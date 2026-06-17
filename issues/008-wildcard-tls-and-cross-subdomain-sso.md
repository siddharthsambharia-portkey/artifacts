# 008: Make the wildcard-subdomain story real — DNS-01 cert + cross-subdomain SSO

> Type: AFK · Priority: P2 · Effort: S/M
> Glossary: Site, Operator, Native OIDC, Identity proxy (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md`.

## What to build

Subdomain-per-site (`my-site.<domain>`) is the marquee feature, but the operator is left to figure
out the hardest part — the wildcard TLS certificate and cross-subdomain session — alone:

- The GCP terraform provisions **no certificate at all** (see issue 005).
- The docs only point at cert-manager generically and at HTTP-01-style setups. **HTTP-01 cannot issue
  a wildcard cert** (`*.<domain>`) — that requires a **DNS-01** challenge. This isn't documented.
- There's no guidance on making the session cookie valid across `*.<domain>`. Native OIDC already
  sets `cookie.Domain = "." + domain` (so Artifact's own SSO spans subdomains), but the oauth2-proxy
  path needs the equivalent `cookie-domain .<domain>` configured on the proxy, which is undocumented.

Close the gap so "subdomains on demand" works end-to-end:

- **Document the DNS-01 wildcard path**: a cert-manager `ClusterIssuer`/`Issuer` using a DNS-01
  solver (e.g. Cloud DNS for the GCP profile) plus a `Certificate` for `*.<domain>` and `<domain>`.
  Explain why HTTP-01 won't work for wildcards.
- **Ship it in Helm** (ideally): an optional cert-manager `Certificate` resource gated by a values
  flag, and the ingress wildcard host + TLS secret reference, so the chart can request the wildcard
  cert rather than leaving it manual.
- **Document cross-subdomain SSO**: confirm/keep Artifact's `cookie.Domain = ".<domain>"` for native
  OIDC, and document the oauth2-proxy `cookie-domain=.<domain>` (and `whitelist-domain`) settings so a
  session established on one subdomain is valid across all sites.

## Acceptance criteria

- [ ] Docs explain DNS-01 (not HTTP-01) is required for `*.<domain>`, with a concrete cert-manager
      `ClusterIssuer` + `Certificate` example for the GCP profile
- [ ] Helm can optionally render a cert-manager `Certificate` for `*.<domain>` + apex, wired to the
      ingress TLS secret (flag-gated; off by default)
- [ ] Cross-subdomain SSO is documented for both native OIDC (existing cookie domain) and oauth2-proxy
      (`cookie-domain`/`whitelist-domain`)
- [ ] `docs/self-hosting.md` wildcard section links the DNS-01 recipe instead of implying HTTP-01
- [ ] `helm lint` / `helm template` pass with the cert resource on and off

## Blocked by

- Soft: 007 (the Helm cert resource and ingress TLS live in the same chart work).
