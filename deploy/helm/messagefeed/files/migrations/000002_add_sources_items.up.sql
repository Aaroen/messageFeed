-- Helm 打包副本；源迁移位于项目 migrations 目录。
-- messageFeed 阶段二 Feed 闭环迁移
-- 文件名：000002_add_sources_items.up.sql
-- 本迁移创建订阅源、抓取条目、用户条目状态和阅读模式偏好表。

-- ==================== 订阅源表 ====================
CREATE TABLE IF NOT EXISTS sources (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(32) NOT NULL DEFAULT 'rss',
    url TEXT NOT NULL,
    normalized_url TEXT NOT NULL,
    status VARCHAR(32) NOT NULL DEFAULT 'active',
    fetch_interval_seconds INTEGER NOT NULL DEFAULT 3600,
    tags JSONB NOT NULL DEFAULT '[]'::jsonb,
    weight INTEGER NOT NULL DEFAULT 0,
    last_fetched_at TIMESTAMP WITH TIME ZONE,
    last_fetch_status VARCHAR(32),
    last_fetch_error TEXT,
    last_fetch_duration_ms INTEGER,
    last_fetch_item_count INTEGER,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT chk_sources_type CHECK (type IN ('rss', 'atom', 'json_feed')),
    CONSTRAINT chk_sources_status CHECK (status IN ('active', 'inactive')),
    CONSTRAINT chk_sources_fetch_interval CHECK (fetch_interval_seconds >= 0),
    CONSTRAINT chk_sources_last_fetch_duration CHECK (last_fetch_duration_ms IS NULL OR last_fetch_duration_ms >= 0),
    CONSTRAINT chk_sources_last_fetch_item_count CHECK (last_fetch_item_count IS NULL OR last_fetch_item_count >= 0),
    CONSTRAINT uq_sources_user_normalized_url UNIQUE (user_id, normalized_url)
);

CREATE INDEX IF NOT EXISTS idx_sources_user_status ON sources(user_id, status);
CREATE INDEX IF NOT EXISTS idx_sources_updated_at ON sources(updated_at);

DROP TRIGGER IF EXISTS update_sources_updated_at ON sources;
CREATE TRIGGER update_sources_updated_at
    BEFORE UPDATE ON sources
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ==================== 条目表 ====================
CREATE TABLE IF NOT EXISTS items (
    id BIGSERIAL PRIMARY KEY,
    source_id BIGINT NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    title TEXT NOT NULL,
    url TEXT NOT NULL,
    normalized_url TEXT NOT NULL,
    raw_guid TEXT,
    content_hash VARCHAR(128),
    summary TEXT,
    content_snippet TEXT,
    author TEXT,
    published_at TIMESTAMP WITH TIME ZONE,
    fetched_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_items_source_normalized_url UNIQUE (source_id, normalized_url)
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_items_source_raw_guid
    ON items(source_id, raw_guid)
    WHERE raw_guid IS NOT NULL AND raw_guid <> '';

CREATE INDEX IF NOT EXISTS idx_items_source_published_at ON items(source_id, published_at DESC NULLS LAST, fetched_at DESC);
CREATE INDEX IF NOT EXISTS idx_items_fetched_at ON items(fetched_at DESC);

DROP TRIGGER IF EXISTS update_items_updated_at ON items;
CREATE TRIGGER update_items_updated_at
    BEFORE UPDATE ON items
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ==================== 用户条目状态表 ====================
CREATE TABLE IF NOT EXISTS user_item_states (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id BIGINT NOT NULL REFERENCES items(id) ON DELETE CASCADE,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    read_at TIMESTAMP WITH TIME ZONE,
    is_favorite BOOLEAN NOT NULL DEFAULT FALSE,
    favorited_at TIMESTAMP WITH TIME ZONE,
    is_hidden BOOLEAN NOT NULL DEFAULT FALSE,
    hidden_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_user_item_states_user_item UNIQUE (user_id, item_id)
);

CREATE INDEX IF NOT EXISTS idx_user_item_states_user_read ON user_item_states(user_id, is_read);
CREATE INDEX IF NOT EXISTS idx_user_item_states_user_favorite ON user_item_states(user_id, is_favorite);
CREATE INDEX IF NOT EXISTS idx_user_item_states_user_hidden ON user_item_states(user_id, is_hidden);

DROP TRIGGER IF EXISTS update_user_item_states_updated_at ON user_item_states;
CREATE TRIGGER update_user_item_states_updated_at
    BEFORE UPDATE ON user_item_states
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ==================== 阅读模式偏好表 ====================
CREATE TABLE IF NOT EXISTS feed_view_preferences (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    view_mode VARCHAR(32) NOT NULL DEFAULT 'timeline',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_feed_view_preferences_user UNIQUE (user_id),
    CONSTRAINT chk_feed_view_preferences_view_mode CHECK (view_mode IN ('timeline'))
);

DROP TRIGGER IF EXISTS update_feed_view_preferences_updated_at ON feed_view_preferences;
CREATE TRIGGER update_feed_view_preferences_updated_at
    BEFORE UPDATE ON feed_view_preferences
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();
