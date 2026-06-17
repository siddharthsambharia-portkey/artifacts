# ARTIFACT — Master Build Spec (single-sitting, complete launch)

You are building **Artifact**: an open-source internal hosting platform — the OSS, enterprise-portable version of Shopify's Quick. This spec is complete and self-contained. Build everything in it, in the order given, verifying each checkpoint before moving on. The end state is a public GitHub repository that any company (Uber, a bank, a 50-person startup) can clone and run in production behind their own SSO, on their own cloud, today.

**License:** Apache-2.0. **Language:** Go 1.23+ single static binary. **Repo:** `artifact` on the user's personal GitHub.

---

## 1. What Artifact is

Drop a folder of HTML into your company's trust bubble → get `mysite.artifact.yourcorp.com`, visible only to authenticated employees, with a zero-config browser API for: **database, file uploads, AI (chat + image gen), data warehouse queries, websockets/realtime, identity, and notifications.** No frameworks, no pipelines, no API keys in client code, no per-site config.

### Feature parity checklist vs Quick (all REQUIRED)
- [x] Static site hosting: subdomain → folder in object storage
- [x] `artifact deploy` — FTP-simple folder upload
- [x] Identity-aware access (every request = verified employee)
- [x] `artifact.db` — Firebase-style document collections + realtime subscribe
- [x] `artifact.files` — file uploads with served URLs
- [x] `artifact.ai.chat` — streaming LLM calls, keys server-side
- [x] `artifact.ai.image` — image generation passthrough
- [x] `artifact.warehouse.query` — read-only SQL against BigQuery/Postgres replicas
- [x] `artifact.ws` — websocket rooms, presence, multiplayer
- [x] `artifact.me` — identity API (name, email, title, team, Slack handle, avatar, groups)
- [x] `artifact.notify.slack` — post messages to Slack via server-held webhook/token
- [x] `artifact init` — drops agent skills (CLAUDE.md / AGENTS.md) so coding agents are instantly fluent
- [x] Home directory at apex domain — searchable list of all sites, recent deploys
- [x] Trust mode: all sites open to all employees, no owners, overwrite-to-update
- [x] Cross-site reads + script imports (the shared-library ecosystem)
- [x] Rate limiting + quotas
- Plus OSS/enterprise additions Quick doesn't need: pluggable auth (Okta/Entra/Google OIDC + header-trust), pluggable storage (S3/GCS/MinIO/local), governed mode toggle, audit log, admin console, Helm chart, MCP server for Claude Code.

### Hard non-goals (refuse forever; constraints are the product)
No per-site custom backends, no cron, no server-side functions, no build pipelines, no per-site env vars/secrets, no public-internet site exposure.

---

## 2. Architecture (one process)

```
*.artifact.corp.com ──▶ [ artifact binary ]
                        ├─ auth middleware (OIDC | header-trust | dev)
                        ├─ static serving ◀── object store (s3|gcs|minio|local)
                        ├─ /api/v1/* ◀──── Postgres (or SQLite in dev)
                        ├─ /api/v1/ai/* ─▶ OpenAI-compatible upstream (gateway)
                        ├─ /api/v1/warehouse ─▶ BigQuery|Postgres (read-only creds)
                        ├─ /api/v1/notify ─▶ Slack webhook (server-held)
                        └─ /ws ── in-process hub (NATS adapter optional for multi-replica)
```

- Subdomain `X.{domain}` → object-store prefix `sites/X/`. Apex serves home directory, docs, `/artifact.js`, admin at `admin.{domain}`.
- Deploys are atomic: upload files → write manifest → flip pointer. Never serve half a site.
- Dev mode (`artifact serve` with no config): SQLite + local-disk storage + fake login + `*.localhost` — works offline on a laptop.

### Locked tech choices
chi router · pgx (Postgres) + modernc.org/sqlite (pure Go, static binary) · minio-go for S3-compatible storage + native GCS adapter + local-disk driver · coreos/go-oidc · coder/websocket · cobra CLI · goreleaser · SDK in TypeScript via Vite, single file < 15 kB gz, no framework · admin console in Preact · embed home/admin/SDK assets in the binary.

### Repo layout
```
artifact/
  cmd/artifact/                 # serve | deploy | init | dev | login | list | open | logs | mcp | version
  internal/{server,auth,sites,storage,db,realtime,ai,warehouse,files,notify,governance,admin,audit}
  sdk/                        # artifact.js (TS) + .d.ts
  web/{home,admin}
  skills/                     # CLAUDE.md + AGENTS.md templates dropped by `artifact init`
  examples/{guestbook,live-poll,team-dashboard,multiplayer-cursors,lunch-vote}
  deploy/{docker-compose.yml, Dockerfile, helm/artifact/, terraform/{gcp,aws}/, recipes/}
  docs/                       # markdown docs site, itself deployable as an Artifact site
  scripts/install.sh          # curl | sh installer
  .github/workflows/{ci.yml, release.yml}
  README.md LICENSE CONTRIBUTING.md SECURITY.md CODE_OF_CONDUCT.md CHANGELOG.md
  demo/demo.tape              # charmbracelet/vhs script that records the README GIF
```

### Config — single `artifact.yaml` (env-var overridable, ARTIFACT_*)
```yaml
branding: { name: Artifact, logo: "" }
domain: artifact.corp.example.com
listen: ":8443"
tls: { mode: auto|manual|off }            # off when behind corp LB/ZTNA

auth:
  mode: oidc | header-trust | dev
  oidc: { issuer: https://corp.okta.com, client_id: "", client_secret_env: ARTIFACT_OIDC_SECRET, groups_claim: groups }
  header_trust: { email_header: X-Auth-Request-Email, name_header: X-Auth-Request-User, proxy_secret_env: ARTIFACT_PROXY_SECRET }

storage: { driver: s3|gcs|local, bucket: artifact-sites, endpoint: "" }
database: { driver: postgres|sqlite, url_env: ARTIFACT_DATABASE_URL }

ai:
  upstream_url: https://gateway.corp.com/v1     # ANY OpenAI-compatible endpoint (OpenAI, Portkey, LiteLLM, Bedrock gw)
  api_key_env: ARTIFACT_AI_KEY
  image_model: ""                               # enables artifact.ai.image when set
  models_allowlist: []

warehouse:
  driver: bigquery|postgres|none                # READ-ONLY credentials only
  credentials_env: ARTIFACT_WAREHOUSE_CREDS
  allowed_datasets: []                          # empty = deny all; explicit allowlist required
  row_limit: 10000

notify:
  slack: { mode: webhook|off, secret_env: ARTIFACT_SLACK_SECRET, channel_allowlist: [] }

governance:
  mode: trust | governed                        # THE toggle
  quotas: { site_max_mb: 500, db_max_docs_per_site: 100000, upload_max_mb: 50, ai_daily_tokens_per_user: 0, warehouse_daily_queries_per_user: 200 }
```

---

## 3. The browser SDK — `artifact.js`

Loaded via `<script src="/artifact.js"></script>`. Same-origin calls to `/api/v1/*` with session cookie. Zero keys, zero config. Fully typed, every method documented with a copy-pasteable example.

```js
await artifact.ready();
artifact.me                                   // {email,name,title,team,slack,avatar,groups}

const posts = artifact.db.collection('posts');
await posts.create({title:'hi'});           // server stamps site, created_by, timestamps
await posts.update(id,{...}); await posts.delete(id);
await posts.list({where:{status:'draft'}, order:'-created_at', limit:50});
const off = posts.subscribe({onCreate,onUpdate,onDelete});   // realtime over ws, auto-reconnect+replay

await artifact.kv.set('k','v'); await artifact.kv.get('k');

const {url} = await artifact.files.upload(file); await artifact.files.list();

const r = await artifact.ai.chat([{role:'user',content:'...'}], {stream:true});
for await (const c of r) {...}
const img = await artifact.ai.image('a watercolor fox');        // returns served URL

const rows = await artifact.warehouse.query('SELECT region, sum(gmv) FROM sales.daily GROUP BY 1');

const room = artifact.ws.room('lobby');
room.on('message',fn); room.send({...});
room.presence.subscribe(users => ...);      // identity-attached presence

await artifact.notify.slack('#team-channel','Deploy is live 🎉');

artifact.db.collection('posts',{site:'team-blog'})              // cross-site read (trust mode)
```

Server rules: site namespace derived from Host/Origin — never client-supplied. Identity always server-stamped. Cross-site reads allowed in trust mode; cross-site writes require target site opt-in via optional `artifact.json` manifest. Warehouse queries: parse/deny anything but SELECT, enforce dataset allowlist + row limit + timeout. AI proxy: streaming passthrough only to the one configured upstream, attaches `x-artifact-user` and `x-artifact-site` headers for gateway-side cost attribution and guardrails. Slack: channel allowlist, rate-limited per user.

---

## 4. CLI + agent integration

```
artifact login        # OIDC device-code flow → token in OS keychain
artifact init [name]  # starter index.html + CLAUDE.md + AGENTS.md (full SDK reference, constraints, 3 worked examples)
artifact deploy       # content-hash diff upload; prints URL; overwrite confirm shows last deployer
artifact dev          # local server: live reload, SQLite, local storage, fake identity — full SDK parity
artifact list | open | logs --site X | version
artifact mcp          # stdio MCP server exposing tools: deploy_site, list_sites, read_logs, query_db
```

The MCP server means Claude Code / any MCP client can build AND ship Artifact sites without the human touching a terminal. The `init`-dropped skill file is the most important file in the project — write it first and dogfood it while building the examples.

DevEx acceptance bar (enforced by e2e tests in CI):
1. `install.sh` → deployed site in **< 60 seconds**.
2. `artifact dev` ↔ production parity: same binary, same SDK, zero "works locally only" gaps.
3. Every error message is a sentence with a fix.

---

## 5. Enterprise integration

**Auth:** (a) built-in OIDC — Okta/Entra/Google Workspace, session cookie scoped `*.{domain}`, groups ingested; (b) header-trust for IAP / Pomerium / oauth2-proxy / ZTNA (Prisma Access etc.) — **refuses to boot** without a proxy shared-secret or mTLS configured; (c) dev mode.

**Network (no Cloudflare needed):** wildcard internal DNS `*.artifact.corp.com` → internal LB → Artifact. TLS from internal CA or Let's Encrypt DNS-01, or `tls: off` behind the corp proxy. Never exposed to the public internet. Ship copy-paste recipes in `deploy/recipes/` for: GCP IAP, AWS ALB+OIDC, Pomerium, oauth2-proxy, Okta direct, plain VPN.

**Governance toggle:** trust mode = the Quick experience (all open, no owners, overwrite anything; audit log still silently records every deploy + destructive call). Governed mode = first deployer owns the site, collaborators/group-scoped visibility, deletion restricted, admin console exposes audit search, quotas, AI + warehouse spend per user/site, departed-employee site transfer. Implement governance as middleware over nullable columns; trust mode is governed mode with all checks returning allow.

**Admin console** (`admin.{domain}`, admin-group gated): sites + sizes + activity, users, audit log search, quota editor, AI usage dashboard, branding, mode toggle.

**Scale posture:** single 2-vCPU/4 GB VM comfortably serves a 5,000-person company (state it in the README — "one small VM" is the marketing). In-memory LRU for hot site manifests. Multi-replica: NATS pub/sub adapter + external Postgres + sticky sessions, documented in the Helm values.

---

## 6. Security non-negotiables
- Session cookies HttpOnly + Secure + SameSite=Lax, scoped to apex.
- Header-trust mode: hard-fail boot without proxy authentication.
- User-uploaded files served with `Content-Disposition: attachment` or from a sandboxed path with restrictive CSP — never executable as a "site" (stored-XSS prevention).
- Warehouse: read-only creds, SELECT-only parser, allowlist, limits. AI proxy: SSRF-proof — only the configured upstream URL, ever.
- Per-user token-bucket rate limits on every API family. Upload size/type limits.
- SBOM in releases, cosign-signed container images, `SECURITY.md` with disclosure policy.
- Migrations embedded, forward-only, run on boot.

---

## 7. Build order (single sitting — verify each checkpoint, then continue)

1. **Scaffold + dev mode boot.** Repo layout, config loader, slog logging, health endpoint, SQLite+local-storage drivers, fake auth, `*.localhost` routing. ✔ `go run ./cmd/artifact serve` → `anything.localhost:8443` shows "no such site" page with dev identity.
2. **Static hosting + deploy loop.** Object-store interface (local first, then s3/gcs), manifest-pointer atomic deploys, MIME/ETag/caching, upload API, `artifact deploy|init|list|open`. ✔ deploy examples/guestbook folder → live URL; overwrite works; e2e test green.
3. **Auth.** OIDC mode + header-trust mode + sessions. ✔ e2e with mock OIDC provider (dex in docker for tests).
4. **Skill files.** Write `skills/CLAUDE.md` + `AGENTS.md` with the complete SDK reference and constraints. ✔ used to build every example below.
5. **SDK core: identity, db, kv, files.** Handlers + artifact.js + types. Quotas + rate limits + friendly errors. ✔ guestbook example fully works with zero site config.
6. **Realtime.** WS hub, rooms, presence, db subscribe wired to write-path events, SDK auto-reconnect/replay. ✔ multiplayer-cursors example: two browsers see each other.
7. **AI.** Streaming chat passthrough + image gen + usage rows + per-user quota. ✔ works against any OpenAI-compatible endpoint (test with a stub server).
8. **Warehouse + notify.** SELECT-only query API with allowlist; Slack webhook notify with channel allowlist. ✔ team-dashboard example pulls warehouse rows; lunch-vote pings Slack.
9. **Home + admin.** Apex site directory (search, recent deploys) built as a static Artifact site using the SDK (dogfood). Admin console with audit search, quotas, usage, mode toggle. ✔ flipping to governed mode enforces ownership live, nothing breaks.
10. **MCP server.** `artifact mcp` stdio with deploy/list/logs/db tools. ✔ Claude Code can ship a site end-to-end.
11. **Packaging.** Dockerfile (distroless), docker-compose (artifact+postgres+minio), Helm chart (external pg/s3 values, ingress examples, 2-replica + NATS values), Terraform examples (GCP: GCS+CloudSQL+ILB; AWS: S3+RDS+ALB), `install.sh`, goreleaser (darwin/linux, amd64/arm64), Homebrew tap formula. ✔ `docker compose up` quickstart green; `helm install` on kind + minio passes e2e.
12. **CI/CD.** ci.yml: lint (golangci-lint), unit, e2e (incl. the 60-second-deploy timing test), SDK build. release.yml: tag → goreleaser → binaries + ghcr image + SBOM + cosign.
13. **Docs + launch assets** (see §8). Record demo GIF via `vhs demo/demo.tape`.
14. **Release.** Tag `v0.1.0`, publish, verify install.sh from the public repo on a clean machine.

---

## 8. Launch manifest (everything the repo ships with on day one)

**README.md** — hero GIF (vhs-generated: init → agent builds a poll → deploy → live URL), one-paragraph pitch, 60-second quickstart (`curl -fsSL .../install.sh | sh && artifact serve`), feature grid, architecture diagram (mermaid), "Deploy at your company" section linking the recipes, comparison note ("inspired by Shopify's Quick; built so everyone else can have it"), badges (CI, release, license, Go report).

**docs/** — quickstart · concepts (sites, trust bubble, constraints) · SDK reference (every method, runnable) · CLI reference · self-hosting guides: GCP / AWS / Kubernetes / single-VM / on-prem MinIO · auth guides: Okta step-by-step (with screenshots placeholders), Entra, Google Workspace, header-trust recipes · AI gateway setup (OpenAI direct, Portkey, LiteLLM, Bedrock) · warehouse setup · governance & admin guide · architecture deep-dive ("how it's built") · FAQ ("why no custom backends" — the philosophy doc) · MIGRATION-FROM-NOTHING.md is not a thing; keep it lean.

**examples/** — guestbook (db+identity) · live-poll (db+subscribe) · team-dashboard (warehouse+charts) · multiplayer-cursors (ws+presence) · lunch-vote (db+ws+notify.slack). Each has its own mini README and is deployable with one command.

**Community/legal:** LICENSE (Apache-2.0), CONTRIBUTING.md (dev setup = `make dev`, test, PR rules), CODE_OF_CONDUCT.md (Contributor Covenant), SECURITY.md, CHANGELOG.md (keep-a-changelog), issue/PR templates, `good-first-issue` seed list (10 items).

**Launch comms (write as files in `launch/`, don't publish automatically):** Show HN post draft ("Show HN: Artifact — open-source version of Shopify's internal Quick platform; run your company's trust bubble on Okta + your own cloud") · X thread draft (8 tweets mirroring the Quick thread structure but OSS-angled) · blog post draft ("Every company deserves a Quick") · r/selfhosted + r/devops post drafts.

**Definition of DONE for v0.1.0:** a stranger at any company can go README → running behind their Okta in < 30 min; their coding agent (via init skills or MCP) builds a working realtime app with no human reading docs; `docker compose up` demo works on a clean machine; all e2e tests green in public CI.

---

## 9. Engineering standards (verbatim rules for the agent)
- Table-driven tests for every handler; one e2e per subsystem driving the real CLI + chromedp for SDK paths.
- Interfaces only at seams (storage, db, pubsub, auth, warehouse, notify). No speculative abstraction.
- SQL written compatible with SQLite + Postgres; dialect shims isolated in one file.
- Never trust client-supplied identity or site fields. Stamp server-side from session + Host.
- Static file p50 < 20 ms warm; binary must run the whole product on 2 vCPU / 4 GB.
- Conventional commits; SemVer; CLI warns on server/CLI version mismatch.
- Every public function in the SDK has a doc comment + example. Docs are written for agents first, humans second.
