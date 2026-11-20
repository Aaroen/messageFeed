ALTER TABLE items ADD COLUMN source_updated_at timestamptz;
ALTER TABLE items ADD COLUMN publication_verified boolean NOT NULL DEFAULT false;

CREATE INDEX idx_items_feed_order ON items ((COALESCE(published_at, source_updated_at, created_at)) DESC, id DESC);
CREATE INDEX idx_items_source_feed_order ON items (source_id, (COALESCE(published_at, source_updated_at, created_at)) DESC, id DESC);

CREATE TABLE feed_recommendation_snapshots (
    cache_key text NOT NULL,
    generation text NOT NULL,
    status text NOT NULL CHECK (status IN ('running', 'ready', 'failed')),
    items jsonb NOT NULL DEFAULT '[]',
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (cache_key, generation)
);

CREATE INDEX idx_feed_recommendation_expiry ON feed_recommendation_snapshots (updated_at);
CREATE UNIQUE INDEX idx_feed_recommendation_running ON feed_recommendation_snapshots (cache_key) WHERE status = 'running';
