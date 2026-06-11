# X Thread Draft (8 tweets)

**1/8** Shopify built Quick — 50k internal sites on one $200/mo VM. Anyone drops HTML, gets a URL with database, AI, websockets baked in. They can't open-source it (welded to GCP+IAP). We built Artifact — same idea, any cloud, Apache-2.0.

**2/8** `artifact deploy` — FTP-simple folder upload → live internal URL behind your SSO. No CI, no tickets, no platform team.

**3/8** Browser SDK, zero API keys: `artifact.db.collection('posts').create({...})` with realtime subscribe. Firebase for employees.

**4/8** `artifact.ai.chat()` — keys server-side, proxied through your AI gateway with per-user cost attribution.

**5/8** `artifact.ws.room('lobby')` — multiplayer, presence, live polls. One line.

**6/8** `artifact init` drops a skill file so your AI assistant builds and ships sites without reading docs. `artifact mcp` exposes deploy as an MCP tool.

**7/8** Pluggable: Okta OIDC, IAP/Pomerium header-trust, S3/GCS/MinIO, Postgres, governed mode toggle, admin console.

**8/8** `curl -fsSL .../install.sh | sh && artifact dev` — running in 60 seconds. github.com/siddharthsambharia-portkey/artifacts
