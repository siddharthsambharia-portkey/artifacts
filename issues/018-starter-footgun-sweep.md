# 018: Sweep starter templates for footguns (hardcoded passwords, namespace typos)

> Type: AFK · Priority: P3 · Effort: S · Triage: ready-for-agent
> Glossary: Operator, Deployment Profile (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md`.

## Parent

010 — Make Artifact installable by a low-privilege champion (the two-stage adoption on-ramp).

## What to build

The starter templates and recipes contain copy-paste footguns: hardcoded passwords (e.g.
`changeme-rotate-immediately`) and namespace typos that an Operator can ship by accident. Audit the chart
values, deploy recipes, terraform starters, and compose/config samples; parameterize or move secret values
to Secret references following the project's secrets model, and fix namespace typos.

End-to-end behavior:

- No starter ships a hardcoded credential as a working default; secret values are parameterized or sourced
  from a Secret reference.
- Namespace references in starters/recipes are consistent and correct.

## Acceptance criteria

- [ ] Hardcoded passwords (e.g. `changeme-rotate-immediately`) in starters are parameterized or moved to
      Secret references, never shipped as a usable default
- [ ] Namespace typos in starters/recipes are fixed and consistent
- [ ] `helm lint` / `helm template` pass; any touched terraform passes `terraform validate`
- [ ] Docs: any sample referencing the changed values is updated to match
- [ ] `go build ./...` and `go test ./...` green

## Blocked by

None — can start immediately.
