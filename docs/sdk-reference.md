# SDK Reference

> Quick reference table. For full examples, constraints, and CLI/MCP usage, see
> [`skills/SKILL.md`](../skills/SKILL.md) (dropped into every project by
> `artifact init`) — that file is canonical.

Load the SDK in any Artifact-hosted page:

```html
<script src="/artifact.js"></script>
```

## artifact.ready()

Returns a Promise that resolves when identity is loaded.

## artifact.me

`{ email, name, title, team, slack, avatar, groups }`

## artifact.db.collection(name, { site? })

| Method | Description |
|--------|-------------|
| `create(data)` | Create document (server stamps metadata) |
| `update(id, data)` | Update document |
| `delete(id)` | Delete document |
| `list({ order, limit, site })` | List documents |
| `subscribe({ onCreate, onUpdate, onDelete })` | Realtime subscription |

## artifact.kv

`set(key, value)` · `get(key)`

## artifact.files

`upload(file)` → `{ id, url, filename, size }`

## artifact.ai

`chat(messages, { stream, model })` · `image(prompt)`

## artifact.warehouse

`query(sql)` → `{ rows }` — SELECT only

## artifact.ws

`room(name)` → `{ on, send, presence: { subscribe } }`

## artifact.notify

`slack(channel, message)`
