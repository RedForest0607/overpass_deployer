CREATE TABLE package_files_v2 (
    id TEXT NOT NULL,
    package_id TEXT NOT NULL REFERENCES packages(id) ON DELETE CASCADE,
    path TEXT NOT NULL,
    blob_key TEXT NOT NULL,
    media_type TEXT NOT NULL,
    size_bytes INTEGER NOT NULL CHECK (size_bytes >= 0),
    sha256 TEXT NOT NULL CHECK (length(sha256) = 64),
    previewable INTEGER NOT NULL CHECK (previewable IN (0, 1)),
    PRIMARY KEY (package_id, id),
    UNIQUE (package_id, path)
);

INSERT INTO package_files_v2 (
    id,
    package_id,
    path,
    blob_key,
    media_type,
    size_bytes,
    sha256,
    previewable
)
SELECT
    id,
    package_id,
    path,
    blob_key,
    media_type,
    size_bytes,
    sha256,
    previewable
FROM package_files;

DROP TABLE package_files;
ALTER TABLE package_files_v2 RENAME TO package_files;
