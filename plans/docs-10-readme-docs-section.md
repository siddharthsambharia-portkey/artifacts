# docs-10: README documentation section + fix broken doc links

> Vertical slice from the docs-and-agent-files initiative. Independently
> verifiable: README has a Documentation section and every doc link in README
> resolves. When done, update the row in `plans/README.md`.

## Status

- **Priority**: P2
- **Effort**: S
- **Type**: AFK
- **Category**: docs
- **Depends on**: docs-02, docs-04

## What to build

A minimal edit to the top-level `README.md` (no rewrite):

- Add a short **Documentation** section linking `docs/README.md` as the entry point, and
  optionally the most-used pages (quickstart, sdk-reference, self-hosting, governance).
- Ensure the README's existing doc links resolve. Today it links `docs/auth-okta.md` (created
  in docs-04) and `docs/faq.md` (created in docs-02); confirm both exist. `deploy/recipes/
  pomerium.md` already exists.
- Do not change the hero, feature table, architecture diagram, or other sections.

## Acceptance criteria

- [ ] README has a Documentation section linking `docs/README.md`
- [ ] Every relative doc link in README resolves (`docs/auth-okta.md`, `docs/faq.md`, etc.)
- [ ] No other README sections changed

## Blocked by

- docs-02 (provides `docs/faq.md` + `docs/README.md`)
- docs-04 (provides `docs/auth-okta.md`)
