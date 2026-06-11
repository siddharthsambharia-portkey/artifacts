CREATE TABLE IF NOT EXISTS documents (
    id TEXT NOT NULL,
    site TEXT NOT NULL,
    collection TEXT NOT NULL,
    data TEXT NOT NULL,
    created_by TEXT NOT NULL,
    updated_by TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    PRIMARY KEY (id, site)
);

CREATE INDEX IF NOT EXISTS idx_documents_site_collection ON documents(site, collection);

CREATE TABLE IF NOT EXISTS kv (
    site TEXT NOT NULL,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    PRIMARY KEY (site, key)
);

CREATE TABLE IF NOT EXISTS sites (
    name TEXT PRIMARY KEY,
    owner TEXT,
    deploy_id TEXT NOT NULL,
    deployed_by TEXT NOT NULL,
    deployed_at DATETIME NOT NULL,
    size_bytes INTEGER NOT NULL DEFAULT 0,
    visibility TEXT NOT NULL DEFAULT 'public'
);

CREATE TABLE IF NOT EXISTS audit_log (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME NOT NULL,
    user_email TEXT NOT NULL,
    site TEXT NOT NULL,
    action TEXT NOT NULL,
    detail TEXT
);

CREATE TABLE IF NOT EXISTS ai_usage (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_email TEXT NOT NULL,
    site TEXT NOT NULL,
    tokens INTEGER NOT NULL,
    timestamp DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS uploaded_files (
    id TEXT PRIMARY KEY,
    site TEXT NOT NULL,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL,
    storage_path TEXT NOT NULL,
    uploaded_by TEXT NOT NULL,
    uploaded_at DATETIME NOT NULL
);
