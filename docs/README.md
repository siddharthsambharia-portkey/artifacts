# Artifact Documentation

Artifact is an **open-source internal hosting platform**. Drop a folder of HTML and get a
live internal URL with a zero-config browser backend — database, realtime, files, AI,
warehouse queries, websockets, and Slack — behind your own SSO, on your own cloud, in one
Go binary.

Start with **[Concepts](concepts.md)** to understand the trust bubble at the core of
Artifact, then follow the **[Quickstart](quickstart.md)** to have a site running in about a
minute.

## Build a site on Artifact

| Doc | What it covers |
|---|---|
| [Quickstart](quickstart.md) | Install, run a local server, deploy your first site in ~60 seconds |
| [Concepts](concepts.md) | Sites, the trust bubble, constraints, trust vs governed mode |
| [SDK reference](sdk-reference.md) | Every `artifact.*` method, the HTTP endpoint behind it, runnable examples |
| [CLI reference](cli-reference.md) | Every `artifact` subcommand and what it actually does today |
| [FAQ / philosophy](faq.md) | "Why no custom backends?", "Is this a fit for me?", common questions |

> Building a site with a coding agent? Point the agent at the skill in
> [`skills/`](../skills/) (`AGENTS.md` / `CLAUDE.md`). `artifact init` drops it into every
> new site automatically.

## Deploy Artifact at your company

| Doc | What it covers |
|---|---|
| [Configuration](configuration.md) | Every `artifact.yaml` field and environment variable, with defaults |
| [Self-hosting](self-hosting.md) | Single VM, Docker Compose, Kubernetes/Helm, GCP, AWS |
| [Auth overview — choosing a mode](auth-overview.md) | Native OIDC vs header-trust, the guaranteed profile, GCP-without-Okta demo path |
| [Auth — Okta](auth-okta.md) | OIDC setup with Okta, step by step (guaranteed profile) |
| [Auth — Microsoft Entra ID](auth-entra.md) | OIDC setup with Entra ID (Azure AD) |
| [Auth — Google Workspace](auth-google.md) | OIDC setup with Google Workspace |
| [Auth — header-trust (IAP / Pomerium / oauth2-proxy)](auth-header-trust.md) | Run behind an existing identity proxy |
| [AI gateway](ai-gateway.md) | Wire `artifact.ai` to any OpenAI-compatible upstream |
| [Warehouse](warehouse.md) | Read-only SQL against BigQuery / Postgres |
| [Governance & admin](governance-and-admin.md) | Trust vs governed mode, quotas, audit log, admin console |
| [Architecture](architecture.md) | How the binary is built — request flow, packages, storage, realtime |

> Deploying with a coding agent? Point the agent at the operator skill at the repo root
> ([`AGENTS.md`](../AGENTS.md) / [`CLAUDE.md`](../CLAUDE.md)).

## Conventions in these docs

- Commands assume the `artifact` binary is on your `PATH`. In a clone, substitute
  `go run ./cmd/artifact` for `artifact`.
- The default development domain is `localhost`, so sites are reachable at
  `http://<site>.localhost:8443`. In production it is your `domain`, e.g.
  `https://<site>.artifact.corp.com`.
- Where current behavior differs from the long-term design, the docs say so in a
  **Current behavior** note rather than describing the aspiration.
