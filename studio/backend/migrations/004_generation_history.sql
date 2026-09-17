ALTER TABLE generations
    ADD COLUMN retry_of_generation_id TEXT REFERENCES generations(id);

CREATE INDEX generations_project_created_idx
    ON generations (project_id, created_at DESC, id DESC);

CREATE INDEX generations_project_environment_created_idx
    ON generations (project_id, environment, created_at DESC, id DESC);

CREATE INDEX generations_retry_of_idx
    ON generations (retry_of_generation_id);

PRAGMA optimize;
