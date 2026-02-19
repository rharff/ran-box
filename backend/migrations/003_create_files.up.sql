-- 003_create_files.up.sql
CREATE TABLE IF NOT EXISTS files (
    id          BIGSERIAL    PRIMARY KEY,
    user_id     BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name        TEXT         NOT NULL,
    mime_type   TEXT,
    total_size  BIGINT       NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS file_blocks (
    id          BIGSERIAL PRIMARY KEY,
    file_id     BIGINT    NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    block_id    BIGINT    NOT NULL REFERENCES blocks(id),
    block_index INT       NOT NULL,
    UNIQUE (file_id, block_index)
);

CREATE INDEX IF NOT EXISTS idx_files_user_id    ON files(user_id);
CREATE INDEX IF NOT EXISTS idx_file_blocks_file ON file_blocks(file_id);
