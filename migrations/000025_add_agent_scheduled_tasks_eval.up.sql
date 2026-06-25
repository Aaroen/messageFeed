CREATE TABLE IF NOT EXISTS agent_scheduled_tasks (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    plan_id BIGINT REFERENCES agent_plans(id) ON DELETE SET NULL,
    source_run_id BIGINT REFERENCES agent_runs(id) ON DELETE SET NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    task_type VARCHAR(64) NOT NULL DEFAULT 'agent_task',
    goal TEXT NOT NULL DEFAULT '',
    target_channel VARCHAR(64) NOT NULL DEFAULT '',
    target_ref TEXT NOT NULL DEFAULT '',
    execution_window_start TIMESTAMP WITH TIME ZONE,
    execution_window_end TIMESTAMP WITH TIME ZONE,
    scheduled_at TIMESTAMP WITH TIME ZONE NOT NULL,
    deliver_at TIMESTAMP WITH TIME ZONE,
    freshness_policy VARCHAR(64) NOT NULL DEFAULT 'latest_at_run',
    allowed_capabilities_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    model_policy_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    failure_policy_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    payload_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    attempt_count INTEGER NOT NULL DEFAULT 0,
    max_attempts INTEGER NOT NULL DEFAULT 3,
    locked_by TEXT NOT NULL DEFAULT '',
    locked_at TIMESTAMP WITH TIME ZONE,
    last_error TEXT NOT NULL DEFAULT '',
    next_run_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_scheduled_tasks_status CHECK (status IN ('queued', 'running', 'input_required', 'succeeded', 'failed', 'canceled', 'expired')),
    CONSTRAINT chk_agent_scheduled_tasks_attempts CHECK (attempt_count >= 0 AND max_attempts >= 1)
);

CREATE INDEX IF NOT EXISTS idx_agent_scheduled_tasks_due
    ON agent_scheduled_tasks(status, scheduled_at ASC, id ASC)
    WHERE status = 'queued';

CREATE INDEX IF NOT EXISTS idx_agent_scheduled_tasks_user_created
    ON agent_scheduled_tasks(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_scheduled_tasks_session
    ON agent_scheduled_tasks(session_id, created_at DESC)
    WHERE session_id IS NOT NULL;

DROP TRIGGER IF EXISTS update_agent_scheduled_tasks_updated_at ON agent_scheduled_tasks;
CREATE TRIGGER update_agent_scheduled_tasks_updated_at
    BEFORE UPDATE ON agent_scheduled_tasks
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_eval_cases (
    id BIGSERIAL PRIMARY KEY,
    case_key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    category VARCHAR(64) NOT NULL DEFAULT '',
    description TEXT NOT NULL DEFAULT '',
    input_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    expected_behavior TEXT NOT NULL DEFAULT '',
    safety_tags_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    metadata_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_eval_cases_category_enabled
    ON agent_eval_cases(category, enabled);

DROP TRIGGER IF EXISTS update_agent_eval_cases_updated_at ON agent_eval_cases;
CREATE TRIGGER update_agent_eval_cases_updated_at
    BEFORE UPDATE ON agent_eval_cases
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_eval_runs (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    trigger_source VARCHAR(64) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'queued',
    model_key TEXT NOT NULL DEFAULT '',
    case_count INTEGER NOT NULL DEFAULT 0,
    passed_count INTEGER NOT NULL DEFAULT 0,
    failed_count INTEGER NOT NULL DEFAULT 0,
    metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_eval_runs_status CHECK (status IN ('queued', 'running', 'completed', 'failed', 'canceled')),
    CONSTRAINT chk_agent_eval_runs_counts CHECK (case_count >= 0 AND passed_count >= 0 AND failed_count >= 0)
);

CREATE INDEX IF NOT EXISTS idx_agent_eval_runs_created
    ON agent_eval_runs(created_at DESC);

DROP TRIGGER IF EXISTS update_agent_eval_runs_updated_at ON agent_eval_runs;
CREATE TRIGGER update_agent_eval_runs_updated_at
    BEFORE UPDATE ON agent_eval_runs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_eval_results (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES agent_eval_runs(id) ON DELETE CASCADE,
    case_id BIGINT NOT NULL REFERENCES agent_eval_cases(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'skipped',
    score DOUBLE PRECISION NOT NULL DEFAULT 0,
    input_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    expected TEXT NOT NULL DEFAULT '',
    actual TEXT NOT NULL DEFAULT '',
    failure_reason TEXT NOT NULL DEFAULT '',
    metrics_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    evidence_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_eval_results_status CHECK (status IN ('passed', 'failed', 'skipped', 'error')),
    CONSTRAINT chk_agent_eval_results_score CHECK (score >= 0 AND score <= 1),
    CONSTRAINT uq_agent_eval_results_run_case UNIQUE (run_id, case_id)
);

CREATE INDEX IF NOT EXISTS idx_agent_eval_results_run
    ON agent_eval_results(run_id, id ASC);
