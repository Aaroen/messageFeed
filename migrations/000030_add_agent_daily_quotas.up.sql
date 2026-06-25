ALTER TABLE agent_notification_preferences
    ADD COLUMN IF NOT EXISTS daily_task_quota INTEGER NOT NULL DEFAULT 50,
    ADD COLUMN IF NOT EXISTS daily_external_call_quota INTEGER NOT NULL DEFAULT 200,
    ADD COLUMN IF NOT EXISTS daily_capability_call_quota INTEGER NOT NULL DEFAULT 500;
