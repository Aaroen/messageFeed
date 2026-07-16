ALTER TABLE agent_notification_preferences
    DROP COLUMN IF EXISTS daily_capability_call_quota,
    DROP COLUMN IF EXISTS daily_external_call_quota,
    DROP COLUMN IF EXISTS daily_task_quota;
