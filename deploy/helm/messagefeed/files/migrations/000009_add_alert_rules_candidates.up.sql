-- Helm 打包副本；源迁移位于项目 migrations 目录。
CREATE TABLE IF NOT EXISTS alert_rules (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    scope VARCHAR(32) NOT NULL,
    condition_json JSONB NOT NULL DEFAULT '{}'::jsonb,
    min_importance NUMERIC(4, 3) NOT NULL DEFAULT 0,
    ai_required BOOLEAN NOT NULL DEFAULT FALSE,
    cooldown_seconds INTEGER NOT NULL DEFAULT 0,
    channel VARCHAR(64) NOT NULL DEFAULT 'in_app',
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    last_triggered_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_alert_rules_scope CHECK (scope IN ('source', 'category', 'tag', 'keyword', 'ticker', 'global')),
    CONSTRAINT chk_alert_rules_importance CHECK (min_importance >= 0 AND min_importance <= 1),
    CONSTRAINT chk_alert_rules_cooldown CHECK (cooldown_seconds >= 0)
);

CREATE INDEX IF NOT EXISTS idx_alert_rules_user_enabled
    ON alert_rules(user_id, enabled);
CREATE INDEX IF NOT EXISTS idx_alert_rules_user_scope
    ON alert_rules(user_id, scope);

DROP TRIGGER IF EXISTS update_alert_rules_updated_at ON alert_rules;
CREATE TRIGGER update_alert_rules_updated_at
    BEFORE UPDATE ON alert_rules
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE IF NOT EXISTS alert_candidates (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    rule_id BIGINT NOT NULL REFERENCES alert_rules(id) ON DELETE CASCADE,
    item_event_id BIGINT REFERENCES item_events(id) ON DELETE SET NULL,
    source_id BIGINT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    item_id BIGINT REFERENCES items(id) ON DELETE CASCADE,
    status VARCHAR(32) NOT NULL DEFAULT 'ready',
    matched_reasons JSONB NOT NULL DEFAULT '[]'::jsonb,
    dedupe_key TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_alert_candidates_status CHECK (status IN ('ready', 'pending_analysis', 'suppressed')),
    CONSTRAINT uq_alert_candidates_dedupe_key UNIQUE (dedupe_key)
);

CREATE INDEX IF NOT EXISTS idx_alert_candidates_user_created
    ON alert_candidates(user_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_alert_candidates_status_created
    ON alert_candidates(status, created_at ASC);
CREATE INDEX IF NOT EXISTS idx_alert_candidates_rule_created
    ON alert_candidates(rule_id, created_at DESC);

DROP TRIGGER IF EXISTS update_alert_candidates_updated_at ON alert_candidates;
CREATE TRIGGER update_alert_candidates_updated_at
    BEFORE UPDATE ON alert_candidates
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
