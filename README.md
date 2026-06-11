# Artifact

**Open-source internal hosting platform** — drop a folder of HTML, get a live internal URL with database, AI, files, websockets, warehouse queries, and Slack notifications. No API keys in client code. One binary.

Inspired by [Shopify's Quick](https://shopify.engineering/quick); built so every company can run their own trust bubble on Okta + their own cloud.

![demo](demo/demo.gif)

## 60-second quickstart

```bash
curl -fsSL https://raw.githubusercontent.com/siddharthsambharia-portkey/artifacts/main/scripts/install.sh | sh
artifact dev
```

In another terminal:

```bash
artifact init my-poll
cd my-poll
# edit index.html — or let your agent do it
artifact deploy
open http://my-poll.localhost:8443
```

## Features

| Capability | API |
|---|---|
| Static hosting | `artifact deploy` → `mysite.artifact.corp.com` |
| Database + realtime | `artifact.db.collection('x').create/subscribe` |
| File uploads | `artifact.files.upload(file)` |
| AI chat + images | `artifact.ai.chat()` / `artifact.ai.image()` |
| Warehouse SQL | `artifact.warehouse.query('SELECT …')` |
| WebSockets | `artifact.ws.room('lobby')` |
| Identity | `artifact.me` |
| Slack | `artifact.notify.slack('#channel', 'msg')` |
| Agent skills | `artifact init` drops CLAUDE.md |
| MCP server | `artifact mcp` |

## Architecture

```mermaid
flowchart LR
  subgraph clients [Employees]
    Browser[Browser + artifact.js]
  end
  subgraph artifact [Artifact binary]
    Auth[Auth middleware]
    Static[Static serving]
    API["/api/v1/*"]
    WS[WebSocket hub]
  end
  subgraph storage [Storage]
    S3[S3 / GCS / MinIO / local]
    PG[(Postgres / SQLite)]
  end
  Browser --> Auth
  Auth --> Static
  Auth --> API
  Auth --> WS
  Static --> S3
  API --> PG
```

One Go binary. Wildcard DNS `*.artifact.corp.com` → Artifact. A **2 vCPU / 4 GB VM** comfortably serves a 5,000-person company.

## Deploy at your company

| Recipe | Path |
|---|---|
| Docker Compose (dev/demo) | `deploy/docker-compose.yml` |
| Kubernetes + Helm | `deploy/helm/artifact/` |
| GCP (GCS + Cloud SQL + ILB) | `deploy/terraform/gcp/` |
| AWS (S3 + RDS + ALB) | `deploy/terraform/aws/` |
| Okta OIDC | `docs/auth-okta.md` |
| Header-trust (IAP, Pomerium) | `deploy/recipes/` |

## Development

```bash
make dev      # local server, SQLite, fake auth
make build    # compile binary
make test     # unit tests
make sdk      # build artifact.js
```

## Examples

| Example | Features |
|---|---|
| [guestbook](examples/guestbook/) | db + identity |
| [live-poll](examples/live-poll/) | db + subscribe |
| [team-dashboard](examples/team-dashboard/) | warehouse |
| [multiplayer-cursors](examples/multiplayer-cursors/) | ws + presence |
| [lunch-vote](examples/lunch-vote/) | db + ws + slack |

## License

Apache-2.0 — see [LICENSE](LICENSE).
