-- messageFeed 数据库初始化迁移
-- 文件名：000001_init_schema.up.sql
-- 本迁移创建阶段一所需的基础表结构，为后续阶段预留扩展空间。

-- ==================== 用户表（预留多租户支持）====================
-- 当前阶段为单用户模式，但数据表预留 user_id 字段便于后续扩展。
-- 阶段一可先插入一条默认用户记录，所有数据关联到该用户。
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    username VARCHAR(255) NOT NULL UNIQUE,
    email VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
);

-- 创建默认用户（单用户模式）
INSERT INTO users (id, username, email) VALUES (1, 'default', 'default@messagefeed.local')
ON CONFLICT (id) DO NOTHING;

-- ==================== 索引与约束 ====================
-- 用户表索引
CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users(created_at);

-- ==================== 触发器：自动更新 updated_at ====================
-- 创建通用触发器函数，用于自动更新 updated_at 字段
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- 为 users 表创建触发器
CREATE TRIGGER update_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- ==================== 说明 ====================
-- 本迁移只创建最基础的用户表，后续阶段会追加：
-- - sources: 订阅源表
-- - items: 内容条目表
-- - user_item_states: 阅读状态表
-- - summaries: AI 摘要表
-- - notifications: 通知记录表
-- - market_instruments: 金融标的表
-- - market_quotes: 行情快照表
-- - market_alert_rules: 告警规则表
-- - market_alert_events: 告警事件表
-- - control_commands: 自然语言设置控制指令表
-- - control_audit_logs: 设置变更审计日志表

-- 迁移策略：
-- 每个阶段创建该阶段所需的表，使用递增版本号（000002、000003...）。
-- 避免在单个迁移文件中创建所有表，保持迁移文件与实施阶段对应。
