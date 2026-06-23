DROP TRIGGER IF EXISTS update_alert_candidates_updated_at ON alert_candidates;
DROP INDEX IF EXISTS idx_alert_candidates_rule_created;
DROP INDEX IF EXISTS idx_alert_candidates_status_created;
DROP INDEX IF EXISTS idx_alert_candidates_user_created;
DROP TABLE IF EXISTS alert_candidates;

DROP TRIGGER IF EXISTS update_alert_rules_updated_at ON alert_rules;
DROP INDEX IF EXISTS idx_alert_rules_user_scope;
DROP INDEX IF EXISTS idx_alert_rules_user_enabled;
DROP TABLE IF EXISTS alert_rules;
