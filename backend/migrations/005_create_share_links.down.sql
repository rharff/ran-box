-- 005_create_share_links.down.sql
DROP INDEX IF EXISTS idx_share_links_file_id;
DROP INDEX IF EXISTS idx_share_links_token;
DROP TABLE IF EXISTS share_links;
