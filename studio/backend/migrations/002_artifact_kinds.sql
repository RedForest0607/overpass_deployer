CREATE TABLE artifacts_v2 (
    id TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('jar', 'config-template', 'env-example', 'file', 'archive', 'script-values')),
    original_name TEXT NOT NULL,
    blob_key TEXT NOT NULL,
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    created_at TEXT NOT NULL
);

INSERT INTO artifacts_v2 SELECT * FROM artifacts;
DROP TABLE artifacts;
ALTER TABLE artifacts_v2 RENAME TO artifacts;

CREATE INDEX artifacts_project_created_idx
    ON artifacts (project_id, created_at DESC, id DESC);
