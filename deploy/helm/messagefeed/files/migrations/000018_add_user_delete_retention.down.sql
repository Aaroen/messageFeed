-- Helm 打包副本；源迁移位于项目 migrations 目录。
DROP INDEX IF EXISTS idx_users_deleted_at;

ALTER TABLE users
    DROP COLUMN IF EXISTS deleted_at;
