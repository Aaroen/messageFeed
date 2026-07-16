-- Helm 打包副本；源迁移位于项目 migrations 目录。
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;

UPDATE users
SET deleted_at = COALESCE(deleted_at, updated_at, NOW())
WHERE status = 'deleted'
  AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_users_deleted_at
    ON users(deleted_at)
    WHERE status = 'deleted';
