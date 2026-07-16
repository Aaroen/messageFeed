-- Helm 打包副本；源迁移位于项目 migrations 目录。
ALTER TABLE agent_notification_preferences
    DROP COLUMN IF EXISTS handoff_on_budget,
    DROP COLUMN IF EXISTS handoff_on_permission,
    DROP COLUMN IF EXISTS handoff_on_failure,
    DROP COLUMN IF EXISTS quality_handoff_threshold,
    DROP COLUMN IF EXISTS auto_recovery_enabled,
    DROP COLUMN IF EXISTS max_queued_tasks,
    DROP COLUMN IF EXISTS max_concurrent_tasks;
