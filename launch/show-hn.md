# Show HN Draft

**Title:** Show HN: Artifact – open-source version of Shopify's internal Quick platform

**Body:**

Shopify built Quick — an internal platform where anyone drops a folder of HTML and gets a live URL with database, AI, websockets, and identity baked in. 50,000+ sites on a single $200/month VM. They can't open-source it because it's welded to their GCP + IAP stack.

We built Artifact: the same idea, rebuilt on pluggable parts so any company can run it.

- One Go binary: static hosting + SDK API + auth + AI proxy + warehouse + websockets
- `artifact deploy` — FTP-simple folder upload
- Browser SDK with zero API keys: `artifact.db`, `artifact.ai`, `artifact.ws`, `artifact.me`
- Pluggable auth: Okta/Entra OIDC or header-trust for IAP/Pomerium
- Pluggable storage: S3/GCS/MinIO/local
- `artifact init` drops agent skills; `artifact mcp` lets your AI assistant deploy sites
- Apache-2.0

```bash
curl -fsSL .../install.sh | sh && artifact dev
artifact init poll && cd poll && artifact deploy
```

GitHub: https://github.com/siddharthsambharia-portkey/artifacts

Would love feedback from folks running internal platforms or building with agents.
