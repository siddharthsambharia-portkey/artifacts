# Artifact — Agent Skill File

You are building a site on **Artifact**, an internal hosting platform. Every request is from an authenticated employee. No API keys, no auth code, no backend — just HTML + the Artifact SDK.

## Constraints (never violate)

- **No custom backends.** Use only Artifact SDK primitives.
- **No API keys in client code.** AI, Slack, warehouse creds are server-held.
- **No per-site env vars or secrets.**
- **Site namespace is server-derived** from the subdomain — never send `site` in write payloads.
- **Warehouse is SELECT-only**, allowlisted datasets.
- **Trust mode:** all employees can read/write all sites. Cross-site reads: `artifact.db.collection('posts', {site:'other-site'})`.

## Setup

```html
<script src="/artifact.js"></script>
<script>
  await artifact.ready();
  console.log(artifact.me); // {email, name, title, team, slack, avatar, groups}
</script>
```

Deploy: `artifact deploy` from the site folder. Overwrite to update.

## SDK Reference

### Identity — `artifact.me`
Server-stamped from SSO. Available after `artifact.ready()`.

### Database — `artifact.db.collection(name, {site?})`

```js
const posts = artifact.db.collection('posts');
await posts.create({ title: 'Hello', body: 'World' });  // server stamps site, created_by, timestamps
await posts.update(id, { title: 'Updated' });
await posts.delete(id);
const list = await posts.list({ order: '-created_at', limit: 50 });
const off = posts.subscribe({ onCreate, onUpdate, onDelete }); // realtime via WebSocket
```

### Key-Value — `artifact.kv`

```js
await artifact.kv.set('theme', 'dark');
const theme = await artifact.kv.get('theme');
```

### Files — `artifact.files`

```js
const { url } = await artifact.files.upload(fileInput.files[0]);
```

### AI — `artifact.ai` (keys server-side)

```js
const resp = await artifact.ai.chat([{ role: 'user', content: 'Summarize this' }], { stream: true });
for await (const chunk of resp) { /* SSE chunks */ }
const img = await artifact.ai.image('a watercolor fox');
```

### Warehouse — `artifact.warehouse` (SELECT only)

```js
const { rows } = await artifact.warehouse.query('SELECT region, sum(gmv) FROM sales.daily GROUP BY 1');
```

### WebSockets — `artifact.ws`

```js
const room = artifact.ws.room('lobby');
room.on('message', (msg) => console.log(msg.from, msg.payload));
room.send({ action: 'move', x: 10, y: 20 });
room.presence.subscribe((users) => console.log('online:', users));
```

### Slack — `artifact.notify`

```js
await artifact.notify.slack('#team-channel', 'Deploy is live 🎉');
```

## Worked Examples

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

### 3. Multiplayer (ws + presence)
```js
const room = artifact.ws.room('game');
document.onmousemove = (e) => room.send({ x: e.clientX, y: e.clientY });
room.on('message', (m) => drawCursor(m.from.name, m.payload));
```

## CLI

```
artifact init [name]   # creates index.html + this skill file
artifact deploy        # upload folder → live URL
artifact dev           # local server at <site>.localhost:8443
artifact list          # all deployed sites
```

## MCP Tools (for Claude Code)

`artifact mcp` exposes: `deploy_site`, `list_sites`, `read_logs`, `query_db`.
