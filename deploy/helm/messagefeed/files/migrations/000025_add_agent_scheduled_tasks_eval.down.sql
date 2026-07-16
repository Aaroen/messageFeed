DROP INDEX IF EXISTS idx_agent_eval_results_run;
DROP TABLE IF EXISTS agent_eval_results;

DROP TRIGGER IF EXISTS update_agent_eval_runs_updated_at ON agent_eval_runs;
DROP INDEX IF EXISTS idx_agent_eval_runs_created;
DROP TABLE IF EXISTS agent_eval_runs;

DROP TRIGGER IF EXISTS update_agent_eval_cases_updated_at ON agent_eval_cases;
DROP INDEX IF EXISTS idx_agent_eval_cases_category_enabled;
DROP TABLE IF EXISTS agent_eval_cases;

DROP TRIGGER IF EXISTS update_agent_scheduled_tasks_updated_at ON agent_scheduled_tasks;
DROP INDEX IF EXISTS idx_agent_scheduled_tasks_session;
DROP INDEX IF EXISTS idx_agent_scheduled_tasks_user_created;
DROP INDEX IF EXISTS idx_agent_scheduled_tasks_due;
DROP TABLE IF EXISTS agent_scheduled_tasks;
