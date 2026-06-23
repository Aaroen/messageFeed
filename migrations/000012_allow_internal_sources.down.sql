ALTER TABLE sources DROP CONSTRAINT IF EXISTS chk_sources_type;
ALTER TABLE sources
    ADD CONSTRAINT chk_sources_type CHECK (type IN ('rss', 'atom', 'json_feed'));
