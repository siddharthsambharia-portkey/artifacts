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

export interface Collection {
  create(data: Record<string, unknown>): Promise<Document>;
  update(id: string, data: Record<string, unknown>): Promise<Document>;
  delete(id: string): Promise<void>;
  list(opts?: { where?: Record<string, unknown>; order?: string; limit?: number; site?: string }): Promise<Document[]>;
  subscribe(callbacks: { onCreate?: (doc: Document) => void; onUpdate?: (doc: Document) => void; onDelete?: (doc: Document) => void }): () => void;
}

export interface ArtifactSDK {
  ready(): Promise<void>;
  readonly me: User;
  db: { collection(name: string, opts?: { site?: string }): Collection };
  kv: { set(key: string, value: string): Promise<void>; get(key: string): Promise<string | null> };
  files: { upload(file: File): Promise<{ id: string; url: string; filename: string; size: number }>; list(): Promise<unknown[]> };
  ai: { chat(messages: { role: string; content: string }[], opts?: { stream?: boolean; model?: string }): Promise<unknown>; image(prompt: string): Promise<{ url: string }> };
  warehouse: { query(sql: string): Promise<{ rows: Record<string, unknown>[] }> };
  ws: { room(name: string): { on(event: string, fn: (data: unknown) => void): void; send(payload: unknown): void; presence: { subscribe(fn: (users: User[]) => void): void } } };
  notify: { slack(channel: string, message: string): Promise<void> };
}

declare const artifact: ArtifactSDK;
export default artifact;
