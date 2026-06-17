# 002: Auth-mode selection guide ("support both") + reflect ADR 0001 in docs

> Type: AFK · Priority: P2 · Effort: S
> Glossary: Operator, Builder, trust mode, governed mode (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md`.

## What to build

Both auth modes already work in code; what's missing is **guidance so an Operator picks the
right one**, plus reflecting the settled stance: **native OIDC is the recommended default**
(it's the only mode that delivers governed mode / admins today), and **header-trust is fully
supported** for shops that already front their apps with a proxy — and for GCP-without-Okta
demos via oauth2-proxy + Google.

This is primarily a **docs** slice. No auth behavior changes are required (issue 001 handles the
header-trust groups gap separately). If, while writing, you find the scaffolding/defaults
contradict "native OIDC is the default," fix that small inconsistency here.

Content:

- A short "Choosing an auth mode" section (in `docs/README.md` and/or a dedicated
  `docs/auth-overview.md`) with a plain comparison:
  - **native OIDC** — Artifact is the OIDC client; groups/admins/governed mode work; pick this
    by default.
  - **header-trust** — a proxy (oauth2-proxy/Pomerium/IAP) authenticates; pick this if you
    already run an org-wide identity proxy or lack IdP-admin access. Note the GCP+Google
    demo path (oauth2-proxy with a Google OAuth client you create in your own GCP project —
    no Okta admin needed).
- Cross-link the existing per-IdP guides (okta/entra/google/header-trust) from that section.
- State the guaranteed profile (GCP + Okta native OIDC) per ADR 0001 so docs and ADR agree.

## Acceptance criteria

- [ ] Docs have a "choosing an auth mode" guide that contrasts native OIDC vs header-trust and
      gives a clear default (native OIDC)
- [ ] The GCP-without-Okta demo path (oauth2-proxy + Google) is documented
- [ ] Existing auth guides are cross-linked from the overview
- [ ] Docs and ADR 0001 agree on the guaranteed profile
- [ ] All relative doc links resolve; no code behavior change required (or only a defaults
      inconsistency fix if found)

## Blocked by

- Soft: 001 (header-trust configurable groups) — once merged, update the comparison so
  header-trust no longer carries the "no admins" caveat. Can be written before 001 lands with a
  forward-looking note.
