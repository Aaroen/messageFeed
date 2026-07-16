-- Helm 打包副本；源迁移位于项目 migrations 目录。
CREATE TABLE IF NOT EXISTS task_locks (
    name VARCHAR(255) PRIMARY KEY,
    owner VARCHAR(255) NOT NULL,
    locked_until TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_task_locks_locked_until
    ON task_locks(locked_until);

DROP TRIGGER IF EXISTS update_task_locks_updated_at ON task_locks;
CREATE TRIGGER update_task_locks_updated_at
    BEFORE UPDATE ON task_locks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
