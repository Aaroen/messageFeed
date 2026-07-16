DROP TRIGGER IF EXISTS update_notification_deliveries_updated_at ON notification_deliveries;
DROP INDEX IF EXISTS idx_notification_deliveries_user_created;
DROP INDEX IF EXISTS idx_notification_deliveries_job_created;
DROP TABLE IF EXISTS notification_deliveries;

DROP TRIGGER IF EXISTS update_notification_jobs_updated_at ON notification_jobs;
DROP INDEX IF EXISTS idx_notification_jobs_candidate;
DROP INDEX IF EXISTS idx_notification_jobs_user_created;
DROP INDEX IF EXISTS idx_notification_jobs_status_scheduled;
DROP TABLE IF EXISTS notification_jobs;
