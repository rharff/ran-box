-- 005_create_share_links.up.sql
CREATE TABLE IF NOT EXISTS share_links (
    id         BIGSERIAL    PRIMARY KEY,
    file_id    BIGINT       NOT NULL REFERENCES files(id) ON DELETE CASCADE,
    user_id    BIGINT       NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT         NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_share_links_token   ON share_links(token);
CREATE INDEX IF NOT EXISTS idx_share_links_file_id ON share_links(file_id);
