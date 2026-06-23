DROP TRIGGER IF EXISTS update_ai_analysis_jobs_updated_at ON ai_analysis_jobs;
DROP INDEX IF EXISTS idx_ai_analysis_jobs_candidate;
DROP INDEX IF EXISTS idx_ai_analysis_jobs_user_created;
DROP INDEX IF EXISTS idx_ai_analysis_jobs_status_scheduled;
DROP TABLE IF EXISTS ai_analysis_jobs;
