# 016: "Harden for production" checklist + co-equal auth framing

> Type: AFK · Priority: P2 · Effort: S · Triage: ready-for-agent
> Glossary: Operator, Native OIDC, Header-trust, Trust mode, Governed mode (see `CONTEXT.md`).
> Relevant decisions: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md`, issue 002 (auth-mode
> selection guide).

## Parent

010 — Make Artifact installable by a low-privilege champion (the two-stage adoption on-ramp).

## What to build

Once the low-power toggles exist (issues 012–015), document the Stage-2 upgrade so a platform team can move
from a champion trial to a hardened rollout by flipping a few values rather than re-installing. Present it as
a "Harden for production" checklist and include the calibrated statement of what low-power trades.

The checklist covers:

- JSON-key / access-key storage → Workload Identity / IRSA.
- Header-trust → Native OIDC.
- Single replica → multi-replica (NATS).
- The calibrated risk statement: outsiders are bounced by SSO; residual risks are internal/accidental
  (header spoofing inside a shared cluster, closed by the proxy-auth shared secret; broad blast radius of a
  leaked storage key) — fine for a trial, fixed by hardening for sensitive data / company-wide rollout.

Reaffirm that Native OIDC and header-trust are co-equal supported modes (OIDC the Stage-2 destination,
header-trust the Stage-1 on-ramp), consistent with issue 002 and ADR 0001. Docs-only; no behavior change.

## Acceptance criteria

- [ ] A "Harden for production" checklist documents JSON-key→WI/IRSA, header-trust→Native OIDC, and
      single→multi-replica as a set of value flips (not a re-install)
- [ ] The calibrated low-power risk statement is included and matches the parent PRD
- [ ] Native OIDC and header-trust are presented as co-equal supported modes (no demotion), consistent with
      issue 002 / ADR 0001
- [ ] All relative doc links resolve
- [ ] No code behavior change (docs-only)

## Blocked by

- 012, 013, 014, 015 — the checklist documents the upgrade away from the toggles those slices add.
