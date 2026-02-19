-- 002_create_blocks.up.sql
CREATE TABLE IF NOT EXISTS blocks (
    id          BIGSERIAL    PRIMARY KEY,
    sha256_hash CHAR(64)     NOT NULL UNIQUE,
    s3_key      TEXT         NOT NULL,
    size_bytes  BIGINT       NOT NULL,
    ref_count   INT          NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_blocks_sha256 ON blocks(sha256_hash);
