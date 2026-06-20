# 019: Verify all docs end-to-end once the on-ramp work lands

> Type: AFK · Priority: P2 · Effort: S · Triage: ready-for-agent
> Glossary: Operator, Builder, Site, Native OIDC, Header-trust, Deployment Profile (see `CONTEXT.md`).
> Relevant decisions: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md`,
> `docs/wildcard-tls-and-routing-options.md`.

## Parent

010 — Make Artifact installable by a low-privilege champion (the two-stage adoption on-ramp).

## What to build

After issues 011–018 land, do a final documentation pass so the docs match the shipped behavior and a
champion can follow them end-to-end. Each prior slice updated its own docs; this slice is the gate that
re-verifies them together, catches drift between slices, and confirms the "champion succeeds in an
afternoon" path is documented and reproducible.

Verify across the docs set:

- The install flow references the real release (issue 011) — no source-build fallback implied, no dangling
  `v0.1.0`.
- The header-trust low-power path (nginx and Traefik glue, issues 012–013) is documented and the
  `pass_request_headers` guidance is gone everywhere.
- The JSON-key storage fallback (014) and Cloud SQL Auth Proxy sidecar (015) are documented and consistent
  with the chart toggles.
- The "Harden for production" checklist (016) and header-contract / cross-cloud doc (017) exist and
  cross-link correctly.
- No footgun values (018) survive in any sample.
- A reader following only the docs can stand up the low-power Stage-1 instance and later perform the Stage-2
  upgrade.

## Acceptance criteria

- [ ] All relative links across `docs/`, `README.md`, recipes, and chart value comments resolve
- [ ] No doc mentions `pass_request_headers` as the header-trust mechanism, and no doc references a
      non-existent `v0.1.0` release/image
- [ ] The low-power Stage-1 path (release → header-trust + ingress glue → key storage → Cloud SQL sidecar)
      is documented end-to-end and internally consistent
- [ ] The Stage-2 "Harden for production" checklist and the header-contract / cross-cloud doc are present
      and cross-linked
- [ ] Docs and ADR 0001 still agree on the guaranteed profile; no stale guidance contradicting issues
      011–018 remains
- [ ] The Builder-facing experience (deploy / SDK / login) is documented as unchanged across both stages

## Blocked by

- 011, 012, 013, 014, 015, 016, 017, 018 — this verifies the docs those slices produce.
