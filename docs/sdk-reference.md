# SDK reference

The browser SDK is loaded with `<script src="/artifact.js"></script>` and exposed as the
global `artifact`. Every call is a same-origin request to `/api/v1/*` carrying the session
cookie — there are no API keys and no configuration. TypeScript types ship in
[`sdk/artifact.d.ts`](../sdk/artifact.d.ts).

Call `ready()` before using any other method or property:

```js
await artifact.ready();
```

The server enforces two invariants:

- **Identity is server-stamped.** You never send who you are; the server fills `created_by` /
  `updated_by` from your session.
- **Site is server-derived** from the request host. You never send a `site` on writes.

API requests are rate-limited per user (token bucket): general API ~20 req/s (burst 50), AI
~5 req/s (burst 10), warehouse ~2 req/s (burst 5), deploy ~1 per 5s (burst 3). Errors return
as JSON `{ "error": "<a sentence telling you how to fix it>" }`.

---

## `artifact.me`

The signed-in employee, available after `ready()`. Read-only.

```js
artifact.me; // { email, name, title?, team?, slack?, avatar?, groups? }
```

Backed by `GET /api/v1/me`.

---

## `artifact.db` — document collections

`artifact.db.collection(name, { site? })` returns a collection handle.

```js
const posts = artifact.db.collection('posts');

const doc  = await posts.create({ title: 'Hello', body: 'World' }); // server stamps site, created_by, timestamps
const doc2 = await posts.update(doc.id, { title: 'Updated' });
await posts.delete(doc.id);

const list = await posts.list({ order: '-created_at', limit: 50 });

const off = posts.subscribe({
  onCreate: (d) => {},
  onUpdate: (d) => {},
  onDelete: (d) => {},
});
// later: off();  // stop listening
```

A document looks like:

```ts
{ id, site, collection, data, created_by, updated_by, created_at, updated_at }
```

Your fields live under `data`. `list()` options: `{ where?, order?, limit?, site? }`.
`order` accepts `created_at` / `-created_at` (descending with the `-` prefix). `limit` is
capped at 1000 (default 50).

**Realtime.** `subscribe()` opens a WebSocket to `/ws` and replays create/update/delete
events for the collection, auto-reconnecting.

**Cross-site reads.** `artifact.db.collection('posts', { site: 'team-blog' }).list()` reads
another site's collection. Allowed in trust mode; subject to visibility in governed mode.
Cross-site writes are not permitted.

HTTP: `POST/GET /api/v1/db/{collection}`, `PUT/DELETE /api/v1/db/{collection}/{id}`. JSON
bodies are capped at 1 MiB.

---

## `artifact.kv` — per-site key/value

```js
await artifact.kv.set('theme', 'dark');
const theme = await artifact.kv.get('theme'); // string | null
```

Values are strings, scoped to the current site. HTTP: `PUT/GET /api/v1/kv/{key}`.

---

## `artifact.files` — uploads

```js
const { id, url, filename, size } = await artifact.files.upload(fileInput.files[0]);
const all = await artifact.files.list();
```

Returns a `url` for the served file, usable in `<img src>` or as a download link. Uploads are
limited by the `upload_max_mb` quota (default 50 MB). Served files use
`Content-Disposition: attachment` and a restrictive CSP; uploaded files cannot execute as sites.
HTTP: `POST/GET /api/v1/files`, `GET /api/v1/files/{id}`.

---

## `artifact.ai` — chat + images

Keys live on the server; the browser never sees them. Artifact proxies calls to your configured
OpenAI-compatible upstream. See [AI gateway](ai-gateway.md).

```js
// Streaming chat
const resp = await artifact.ai.chat(
  [{ role: 'user', content: 'Summarize this page in 3 bullets' }],
  { stream: true, model: 'gpt-4o-mini' } // model optional; subject to the server allowlist
);
for await (const chunk of resp) { /* SSE chunks */ }

// Image generation (only when the server has image_model configured)
const { url } = await artifact.ai.image('a watercolor fox');
```

HTTP: `POST /api/v1/ai/chat`, `POST /api/v1/ai/image`. Per-user daily call quotas can be set
with `ai_daily_calls_per_user` (0 = unlimited).

---

## `artifact.warehouse` — read-only SQL

```js
const { rows } = await artifact.warehouse.query(
  'SELECT region, sum(gmv) AS gmv FROM sales.daily GROUP BY region'
);
```

**SELECT-only.** The server parses and rejects anything that isn't a single read query,
enforces an allowlist of datasets, applies a row limit, and uses read-only credentials. If no
warehouse driver is configured, this method is unavailable. HTTP:
`POST /api/v1/warehouse/query`. See [Warehouse](warehouse.md).

---

## `artifact.ws` — rooms, messages, presence

```js
const room = artifact.ws.room('lobby');

room.on('message', (msg) => console.log(msg));
room.send({ action: 'move', x: 10, y: 20 });

room.presence.subscribe((users) => console.log('online:', users)); // identity-attached
```

Backed by the realtime hub at `/ws`. The same hub delivers `db.subscribe` events.

---

## `artifact.notify` — Slack

```js
await artifact.notify.slack('#team-deploys', 'Lunch poll is live 🎉');
```

Posts via a server-held webhook or bot token. Channels are restricted to a server-side
allowlist and rate-limited per user. HTTP: `POST /api/v1/notify/slack`. Requires Slack to be
configured (`notify.slack.mode`).

---

## Error handling pattern

```js
try {
  await artifact.db.collection('posts').create({ title: 'Hi' });
} catch (err) {
  // err.message is a human sentence, e.g.
  // "site \"my-site\" has 100000 documents (limit 100000). Delete old data or ask an admin..."
  showToast(err.message);
}
```
