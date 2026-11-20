DROP TABLE feed_recommendation_snapshots;
DROP INDEX idx_items_source_feed_order;
DROP INDEX idx_items_feed_order;
ALTER TABLE items DROP COLUMN publication_verified;
ALTER TABLE items DROP COLUMN source_updated_at;
