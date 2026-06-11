/**
 * Artifact Browser SDK — zero-config internal platform API.
 * Load via <script src="/artifact.js"></script>
 */

export interface User {
  email: string;
  name: string;
  title?: string;
  team?: string;
  slack?: string;
  avatar?: string;
  groups?: string[];
}

export interface Document {
  id: string;
  site: string;
  collection: string;
  data: Record<string, unknown>;
  created_by: string;
  updated_by: string;
  created_at: string;
  updated_at: string;
}

export interface ListOptions {
  where?: Record<string, unknown>;
  order?: string;
  limit?: number;
  site?: string;
}

export interface SubscribeCallbacks {
  onCreate?: (doc: Document) => void;
  onUpdate?: (doc: Document) => void;
  onDelete?: (doc: Document) => void;
}

export class Collection {
  constructor(
    private name: string,
    private site?: string
  ) {}

  /** Create a document. Server stamps site, created_by, timestamps.
   * @example await posts.create({ title: 'Hello' }) */
  async create(data: Record<string, unknown>): Promise<Document> {
    const res = await api('POST', `/db/${this.name}`, data);
    return res;
  }

  /** Update a document by id.
   * @example await posts.update(id, { title: 'Updated' }) */
  async update(id: string, data: Record<string, unknown>): Promise<Document> {
    return api('PUT', `/db/${this.name}/${id}`, data);
  }

  /** Delete a document by id.
   * @example await posts.delete(id) */
  async delete(id: string): Promise<void> {
    await api('DELETE', `/db/${this.name}/${id}`);
  }

  /** List documents in the collection.
   * @example await posts.list({ order: '-created_at', limit: 50 }) */
  async list(opts: ListOptions = {}): Promise<Document[]> {
    const q = opts.site ? `?site=${opts.site}` : '';
    return api('GET', `/db/${this.name}${q}`);
  }

  /** Subscribe to realtime changes. Auto-reconnects.
   * @example const off = posts.subscribe({ onCreate: d => console.log(d) }) */
  subscribe(callbacks: SubscribeCallbacks): () => void {
    return subscribeCollection(this.name, callbacks, this.site);
  }
}

let _me: User | null = null;
let _ready: Promise<void> | null = null;
const _wsByRoom = new Map<string, WebSocket>();
const _subs = new Map<string, Set<SubscribeCallbacks>>();

async function api(method: string, path: string, body?: unknown): Promise<any> {
  const res = await fetch(`/api/v1${path}`, {
    method,
    headers: body ? { 'Content-Type': 'application/json' } : {},
    body: body ? JSON.stringify(body) : undefined,
    credentials: 'same-origin',
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }));
    throw new Error(err.error || res.statusText);
  }
  if (res.status === 204) return;
  return res.json();
}

function siteName(): string {
  const host = location.hostname;
  const parts = host.split('.');
  if (parts.length >= 2 && parts[parts.length - 1] === 'localhost') return parts[0];
  return parts[0];
}

function connectWS(room: string): WebSocket {
  const existing = _wsByRoom.get(room);
  if (existing && existing.readyState === WebSocket.OPEN) return existing;
  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(`${proto}//${location.host}/ws?room=${encodeURIComponent(room)}`);
  ws.onmessage = (ev) => {
    try {
      const msg = JSON.parse(ev.data);
      if (msg.type === 'create' || msg.type === 'update' || msg.type === 'delete') {
        const key = `${msg.site}:${msg.collection}`;
        const subs = _subs.get(key);
        if (subs) {
          const doc = typeof msg.document === 'string' ? JSON.parse(msg.document) : msg.document;
          subs.forEach((cb) => {
            if (msg.type === 'create') cb.onCreate?.(doc);
            if (msg.type === 'update') cb.onUpdate?.(doc);
            if (msg.type === 'delete') cb.onDelete?.(doc);
          });
        }
      }
    } catch {}
  };
  ws.onclose = () => {
    _wsByRoom.delete(room);
    setTimeout(() => connectWS(room), 2000);
  };
  _wsByRoom.set(room, ws);
  return ws;
}

function subscribeCollection(name: string, callbacks: SubscribeCallbacks, site?: string): () => void {
  connectWS(name);
  const key = `${site || siteName()}:${name}`;
  if (!_subs.has(key)) _subs.set(key, new Set());
  _subs.get(key)!.add(callbacks);
  return () => _subs.get(key)?.delete(callbacks);
}

const artifact = {
  /** Wait for SDK initialization and identity load.
   * @example await artifact.ready() */
  ready(): Promise<void> {
    if (!_ready) {
      _ready = api('GET', '/me').then((u: User) => { _me = u; });
    }
    return _ready;
  },

  /** Current authenticated user identity.
   * @example console.log(artifact.me.email) */
  get me(): User {
    if (!_me) throw new Error('Call artifact.ready() first');
    return _me;
  },

  /** Firebase-style document collection.
   * @example artifact.db.collection('posts') */
  db: {
    collection(name: string, opts?: { site?: string }): Collection {
      return new Collection(name, opts?.site);
    },
  },

  /** Key-value store scoped to the current site.
   * @example await artifact.kv.set('theme', 'dark') */
  kv: {
    async set(key: string, value: string): Promise<void> {
      await api('PUT', `/kv/${key}`, { value });
    },
    async get(key: string): Promise<string | null> {
      try {
        const r = await api('GET', `/kv/${key}`);
        return r.value;
      } catch {
        return null;
      }
    },
  },

  /** File uploads with served URLs.
   * @example const { url } = await artifact.files.upload(file) */
  files: {
    async upload(file: File): Promise<{ id: string; url: string; filename: string; size: number }> {
      const form = new FormData();
      form.append('file', file);
      const res = await fetch('/api/v1/files', { method: 'POST', body: form, credentials: 'same-origin' });
      if (!res.ok) throw new Error((await res.json()).error);
      return res.json();
    },
    async list(): Promise<{ id: string; url: string; filename: string; size: number }[]> {
      return api('GET', '/files');
    },
  },

  /** AI chat and image generation (keys server-side).
   * @example const r = await artifact.ai.chat([{ role: 'user', content: 'hi' }], { stream: true }) */
  ai: {
    async chat(messages: { role: string; content: string }[], opts?: { stream?: boolean; model?: string }): Promise<any> {
      const res = await fetch('/api/v1/ai/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ messages, stream: opts?.stream, model: opts?.model }),
        credentials: 'same-origin',
      });
      if (!res.ok) throw new Error((await res.json()).error);
      if (opts?.stream) return streamResponse(res);
      return res.json();
    },
    async image(prompt: string): Promise<{ url: string }> {
      const res = await api('POST', '/ai/image', { prompt });
      return res;
    },
  },

  /** Read-only warehouse SQL queries.
   * @example const { rows } = await artifact.warehouse.query('SELECT 1') */
  warehouse: {
    async query(sql: string): Promise<{ rows: Record<string, unknown>[] }> {
      return api('POST', '/warehouse/query', { sql });
    },
  },

  /** WebSocket rooms with presence.
   * @example const room = artifact.ws.room('lobby') */
  ws: {
    room(name: string) {
      const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
      const conn = new WebSocket(`${proto}//${location.host}/ws?room=${name}`);
      const handlers: ((data: unknown) => void)[] = [];
      conn.onmessage = (ev) => {
        const msg = JSON.parse(ev.data);
        handlers.forEach((h) => h(msg));
      };
      return {
        on(event: string, fn: (data: unknown) => void) {
          handlers.push(fn);
        },
        send(payload: unknown) {
          conn.send(JSON.stringify({ type: 'message', payload }));
        },
        presence: {
          subscribe(fn: (users: User[]) => void) {
            handlers.push((msg: any) => { if (msg.type === 'presence') fn([msg.user]); });
          },
        },
      };
    },
  },

  /** Post to Slack via server-held webhook.
   * @example await artifact.notify.slack('#team', 'Ship it!') */
  notify: {
    async slack(channel: string, message: string): Promise<void> {
      await api('POST', '/notify/slack', { channel, message });
    },
  },
};

async function* streamResponse(res: Response) {
  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    yield decoder.decode(value);
  }
}

declare global {
  interface Window {
    artifact: typeof artifact;
  }
}

(window as any).artifact = artifact;
export default artifact;
