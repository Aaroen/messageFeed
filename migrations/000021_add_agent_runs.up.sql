CREATE TABLE IF NOT EXISTS agent_runs (
    id BIGSERIAL PRIMARY KEY,
    parent_run_id BIGINT REFERENCES agent_runs(id) ON DELETE SET NULL,
    session_id BIGINT REFERENCES agent_sessions(id) ON DELETE SET NULL,
    turn_id BIGINT REFERENCES agent_turns(id) ON DELETE SET NULL,
    role VARCHAR(32) NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'running',
    task_packet_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    capability_scope_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    model_key TEXT NOT NULL DEFAULT '',
    context_budget_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    context_trace_ref TEXT NOT NULL DEFAULT '',
    result_ref TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    trace_id TEXT NOT NULL DEFAULT '',
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_runs_role CHECK (role IN ('controller', 'executor')),
    CONSTRAINT chk_agent_runs_status CHECK (status IN ('running', 'succeeded', 'failed', 'input_required', 'canceled'))
);

CREATE INDEX IF NOT EXISTS idx_agent_runs_turn_role
    ON agent_runs(turn_id, role, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_agent_runs_parent
    ON agent_runs(parent_run_id, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_agent_runs_session_created
    ON agent_runs(session_id, created_at DESC);

DROP TRIGGER IF EXISTS update_agent_runs_updated_at ON agent_runs;
CREATE TRIGGER update_agent_runs_updated_at
    BEFORE UPDATE ON agent_runs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS agent_run_context_traces (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    trace_kind VARCHAR(64) NOT NULL,
    prompt_version TEXT NOT NULL DEFAULT '',
    model_key TEXT NOT NULL DEFAULT '',
    content_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    content_hash TEXT NOT NULL DEFAULT '',
    redaction_status VARCHAR(32) NOT NULL DEFAULT 'redacted',
    token_estimate INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_agent_run_context_redaction CHECK (redaction_status IN ('redacted', 'not_required'))
);

CREATE INDEX IF NOT EXISTS idx_agent_run_context_traces_run
    ON agent_run_context_traces(run_id, created_at ASC);

CREATE TABLE IF NOT EXISTS agent_observations (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    capability_key TEXT NOT NULL DEFAULT '',
    input_summary TEXT NOT NULL DEFAULT '',
    output_summary TEXT NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT '',
    error TEXT NOT NULL DEFAULT '',
    artifact_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_observations_run
    ON agent_observations(run_id, created_at ASC);

CREATE TABLE IF NOT EXISTS agent_artifacts (
    id BIGSERIAL PRIMARY KEY,
    run_id BIGINT NOT NULL REFERENCES agent_runs(id) ON DELETE CASCADE,
    artifact_type TEXT NOT NULL DEFAULT '',
    content_ref TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    source_refs_json JSONB NOT NULL DEFAULT '[]'::jsonb,
    content_hash TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_agent_artifacts_run
    ON agent_artifacts(run_id, created_at ASC);
