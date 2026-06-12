# FAQ

## Why no custom backends?

Custom backends mean per-site servers, env vars, on-call ownership, and security review for every throwaway tool. Artifact's bet: a small fixed set of primitives covers 95% of internal apps when you're already inside the trust bubble.

## How is this different from Quick?

Shopify's Quick is welded to their GCP + IAP stack. Artifact is the same idea with pluggable auth (Okta/Entra/header-trust), storage (S3/GCS/MinIO), and governance — Apache-2.0, self-hostable.

## Can I expose a site to the public internet?

No. Artifact is internal-only by design. Never put it on a public ingress.

## What does one VM serve?

A 2 vCPU / 4 GB instance comfortably serves a 5,000-person company for static hosting + SDK API. Add Postgres and object storage externally for production.

## What can't Artifact do?

Artifact is a fixed set of primitives, not a backend. Some things are deliberately out of scope:

- **Sending email** — there is no mail primitive. Use `artifact.notify.slack()` to ping a channel instead.
- **Scheduled / cron jobs** — Artifact only runs code in response to a browser request. Run a separate service for anything time-triggered.
- **Calling internal APIs with secret tokens** — there are no per-site secrets or env vars, so a site can't hold a credential to call another system. Not a fit — by design.

This is the same "no custom backends, no cron, no per-site secrets" constraint described in [Concepts → Constraints](concepts.md) and the "Why no custom backends?" answer above. When something feels missing, combine two SDK primitives instead.
