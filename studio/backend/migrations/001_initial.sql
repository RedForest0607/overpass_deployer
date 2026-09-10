CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    owner_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT NOT NULL,
    revision INTEGER NOT NULL CHECK (revision >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX projects_owner_updated_idx
    ON projects (owner_id, updated_at DESC, id DESC);

CREATE TABLE topologies (
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment TEXT NOT NULL CHECK (environment IN ('dev', 'qa', 'prod')),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    blueprint_json TEXT NOT NULL,
    layout_json TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    PRIMARY KEY (project_id, environment)
);

CREATE TABLE artifacts (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('jar', 'config-template', 'env-example')),
    original_name TEXT NOT NULL,
    blob_key TEXT NOT NULL,
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    created_at TEXT NOT NULL
);

CREATE INDEX artifacts_project_created_idx
    ON artifacts (project_id, created_at DESC, id DESC);

CREATE TABLE packages (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment TEXT NOT NULL CHECK (environment IN ('dev', 'qa', 'prod')),
    topology_revision INTEGER NOT NULL CHECK (topology_revision >= 1),
    output TEXT NOT NULL CHECK (output IN ('draft', 'package')),
    generator TEXT NOT NULL,
    generator_version TEXT NOT NULL,
    chart_format TEXT NOT NULL,
    manifest_json TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE package_files (
    id TEXT PRIMARY KEY,
    package_id TEXT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    blob_key TEXT NOT NULL,
    media_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    previewable INTEGER NOT NULL CHECK (previewable IN (0, 1)),
    UNIQUE (package_id, path)
);

CREATE TABLE generations (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    environment TEXT NOT NULL CHECK (environment IN ('dev', 'qa', 'prod')),
    topology_revision INTEGER NOT NULL CHECK (topology_revision >= 1),
    snapshot_json TEXT NOT NULL,
    generator TEXT NOT NULL,
    output TEXT NOT NULL CHECK (output IN ('draft', 'package')),
    status TEXT NOT NULL CHECK (status IN ('queued', 'running', 'succeeded', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    package_id TEXT REFERENCES packages(id),
    error_json TEXT,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE INDEX generations_status_created_idx
    ON generations (status, created_at, id);

CREATE TABLE idempotency_keys (
    owner_id TEXT NOT NULL,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL,
    request_hash TEXT NOT NULL CHECK (length(request_hash) = 64),
    generation_id TEXT NOT NULL REFERENCES generations(id) ON DELETE CASCADE,
    expires_at TEXT NOT NULL,
    PRIMARY KEY (owner_id, project_id, idempotency_key)
);
