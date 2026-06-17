# Artifact — Agent Skill (AGENTS.md)

You are building a site on **Artifact**, an internal hosting platform. Every request is from an authenticated employee. No API keys, no auth code, no backend — just HTML + the Artifact SDK.

---

## Publish / retrieve / change workflow

### Publish a new site or change an existing one

```bash
artifact deploy          # upload folder → live URL (prompts for confirmation if site exists)
artifact deploy --yes    # overwrite without confirmation — use this to change/update
```

**Overwriting redeploys all files atomically.** To change a site: edit files locally, run `artifact deploy --yes`.

**No-CLI path:** drag a folder onto the Artifact home page, or send a multipart `POST /api/v1/deploy` (`site`, `files[]` or `zip`, optional `confirm_overwrite=true`).

### Retrieve / inspect

```bash
artifact list                  # all deployed sites: name, date, deployer, size
artifact open --site <name>    # print (or open) the live URL
artifact logs --site <name>    # audit log: deploys, SDK calls, errors
```

---

## Constraints (never violate)

- **No custom backends.** Use only Artifact SDK primitives.
- **No API keys in client code.** AI, Slack, and warehouse credentials are server-held.
- **No per-site env vars or secrets.**
- **Site namespace is server-derived** from the subdomain — never send `site` in write payloads.
- **Warehouse is SELECT-only**, allowlisted datasets.
- **Trust mode:** all employees can read/write all sites. Cross-site reads: `artifact.db.collection('posts', {site:'other-site'})`.

---

## Setup

```html
<script src="/artifact.js"></script>
<script>
  await artifact.ready();
  console.log(artifact.me); // {email, name, title?, team?, slack?, avatar?, groups?}
</script>
```

Always call `artifact.ready()` before any SDK method. TypeScript types: `sdk/artifact.d.ts`.

---

## SDK Reference

### Identity — `artifact.me`

Server-stamped from SSO. Available after `artifact.ready()`. Read-only.

```js
artifact.me.email   // "alice@company.com"
artifact.me.name    // "Alice"
artifact.me.title   // optional string
artifact.me.team    // optional string
artifact.me.slack   // optional string
artifact.me.avatar  // optional URL string
artifact.me.groups  // optional string[]
```

---

### Database — `artifact.db.collection(name, {site?})`

Returns a collection handle. The server stamps `site`, `created_by`, `updated_by`, and timestamps on every write — never include them in your payload.

```js
const posts = artifact.db.collection('posts');

const doc  = await posts.create({ title: 'Hello', body: 'World' });
const doc2 = await posts.update(doc.id, { title: 'Updated' });
await posts.delete(doc.id);

// list opts: where, order, limit (max 1000, default 50), site
const list     = await posts.list({ order: '-created_at', limit: 50 });
const filtered = await posts.list({ where: { status: 'published' } });

// realtime — returns an unsubscribe function
const off = posts.subscribe({ onCreate, onUpdate, onDelete });
// later: off();
```

A document looks like `{ id, site, collection, data, created_by, updated_by, created_at, updated_at }`. Your fields live under `data`.

**Cross-site reads (trust mode):**

```js
const remote = artifact.db.collection('announcements', { site: 'team-blog' });
const all    = await remote.list();
```

Cross-site writes are not permitted.

---

### Key-Value — `artifact.kv`

Per-site string store.

```js
await artifact.kv.set('theme', 'dark');
const theme = await artifact.kv.get('theme'); // string | null
```

---

### Files — `artifact.files`

```js
const { id, url, filename, size } = await artifact.files.upload(fileInput.files[0]);
const all = await artifact.files.list();
```

`url` is ready to use in `<img src>` or links. Served with `Content-Disposition: attachment` — uploaded files are data, not executable sites.

---

### AI — `artifact.ai` (keys server-side)

```js
// Streaming chat
const resp = await artifact.ai.chat(
  [{ role: 'user', content: 'Summarize this page in 3 bullets' }],
  { stream: true, model: 'gpt-4o-mini' }  // model optional, subject to server allowlist
);
for await (const chunk of resp) { /* SSE chunks */ }

// Non-streaming
const result = await artifact.ai.chat([{ role: 'user', content: 'Hello' }]);

// Image generation (requires image_model configured on server)
const { url } = await artifact.ai.image('a watercolor fox');
```

---

### Warehouse — `artifact.warehouse` (SELECT only)

```js
const { rows } = await artifact.warehouse.query(
  'SELECT region, sum(gmv) AS gmv FROM sales.daily GROUP BY region'
);
```

Server enforces SELECT-only, dataset allowlist, and row caps. Unavailable if no warehouse driver is configured.

---

### WebSockets — `artifact.ws`

Realtime rooms with presence. The same hub delivers `db.subscribe` events.

```js
const room = artifact.ws.room('lobby');

room.on('message', (msg) => console.log(msg.from, msg.payload));
room.send({ action: 'move', x: 10, y: 20 });
room.presence.subscribe((users) => console.log('online:', users));
```

---

### Slack — `artifact.notify`

```js
await artifact.notify.slack('#team-channel', 'Deploy is live!');
```

Posts via a server-held incoming webhook. Requires `notify.slack.mode: webhook` configured on the server.

---

## Worked examples

### 1. Guestbook (db + identity)

```js
const entries = artifact.db.collection('entries');
await entries.create({ message: 'Hello!', author: artifact.me.name });
const all = await entries.list({ order: '-created_at' });
entries.subscribe({ onCreate: (e) => appendToDOM(e) });
```

### 2. Live poll (db + subscribe)

```js
const votes = artifact.db.collection('votes');
await votes.create({ option: 'Pizza', voter: artifact.me.email });
votes.subscribe({ onCreate: () => refreshChart() });
```

### 3. Multiplayer cursors (ws + presence)

```js
const room = artifact.ws.room('game');
document.onmousemove = (e) => room.send({ x: e.clientX, y: e.clientY });
room.on('message', (m) => drawCursor(m.from.name, m.payload));
room.presence.subscribe((online) => showAvatarRow(online));
```

### 4. File upload and display

```js
fileInput.onchange = async () => {
  const { url } = await artifact.files.upload(fileInput.files[0]);
  document.getElementById('preview').src = url;
};
```

### 5. Warehouse dashboard

```js
const { rows } = await artifact.warehouse.query(
  'SELECT dept, count(*) AS headcount FROM hr.employees GROUP BY dept ORDER BY 2 DESC'
);
rows.forEach(r => renderRow(r.dept, r.headcount));
```

---

## CLI quick reference

```
artifact init [name]      # create site folder with index.html + AGENTS.md + CLAUDE.md
artifact deploy           # publish or update (confirm if site exists)
artifact deploy --yes     # overwrite/change without prompt
artifact dev              # local dev server at http://<site>.localhost:8443
artifact list             # list all deployed sites
artifact open --site <n>  # print URL for site
artifact logs --site <n>  # recent audit log
```

## MCP tools (for Claude Code / AI clients)

`artifact mcp` exposes: `deploy_site`, `list_sites`, `read_logs`, `query_db`.
