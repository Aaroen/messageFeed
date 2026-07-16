DROP TRIGGER IF EXISTS update_task_locks_updated_at ON task_locks;
DROP INDEX IF EXISTS idx_task_locks_locked_until;
DROP TABLE IF EXISTS task_locks;
