-- 003_create_files.down.sql
DROP INDEX IF EXISTS idx_file_blocks_file;
DROP INDEX IF EXISTS idx_files_user_id;
DROP TABLE IF EXISTS file_blocks;
DROP TABLE IF EXISTS files;
