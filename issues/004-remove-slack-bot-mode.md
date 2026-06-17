# 004: Remove the unimplemented Slack `bot` mode

> Type: AFK · Priority: P2 · Effort: S
> Glossary: Operator, deployment profile (see `CONTEXT.md`).
> Relevant decision: `docs/adr/0001-guaranteed-deployment-profile-gcp-okta.md` (delete the lies).

## What to build

`notify.slack.mode` advertises `webhook | bot`, but the handler only ever POSTs to a webhook
URL — `bot` silently behaves like `webhook` (and the channel override doesn't work for classic
webhooks). Collapse the advertised options to `off | webhook` so config matches behavior. Real
Slack bot-token support (proper channels, `chat.postMessage`) can return later as an explicit
feature.

Scope:

- Treat `notify.slack.mode` as `off | webhook` only; reject/clarify `bot` (config validation
  error or documented removal). Keep the existing webhook send path unchanged.
- Update docs and examples that mention `bot`: the Slack/notify feature guide,
  `docs/configuration.md`, `artifact.yaml`, agent skill / build-spec references. Update
  `CHANGELOG.md`. If docs imply per-channel delivery via classic webhooks, correct that.

## Acceptance criteria

- [ ] `notify.slack.mode` accepts only `off` and `webhook`; `bot` is no longer advertised
- [ ] No remaining reference to a Slack `bot` mode in code, config, or docs (grep)
- [ ] Webhook notifications still work; `go build ./...` and `go test ./...` green
- [ ] ADR 0001's "trims decided but not yet executed" note can drop Slack `bot`

## Blocked by

None — can start immediately.
