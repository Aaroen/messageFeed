-- Helm 打包副本；源迁移位于项目 migrations 目录。
-- messageFeed 阶段二 Feed 闭环迁移回滚
-- 文件名：000002_add_sources_items.down.sql

DROP TRIGGER IF EXISTS update_feed_view_preferences_updated_at ON feed_view_preferences;
DROP TABLE IF EXISTS feed_view_preferences;

DROP TRIGGER IF EXISTS update_user_item_states_updated_at ON user_item_states;
DROP TABLE IF EXISTS user_item_states;

DROP TRIGGER IF EXISTS update_items_updated_at ON items;
DROP INDEX IF EXISTS idx_items_fetched_at;
DROP INDEX IF EXISTS idx_items_source_published_at;
DROP INDEX IF EXISTS uq_items_source_raw_guid;
DROP TABLE IF EXISTS items;

DROP TRIGGER IF EXISTS update_sources_updated_at ON sources;
DROP INDEX IF EXISTS idx_sources_updated_at;
DROP INDEX IF EXISTS idx_sources_user_status;
DROP TABLE IF EXISTS sources;
