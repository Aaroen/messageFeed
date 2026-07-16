-- Helm 打包副本；源迁移位于项目 migrations 目录。
-- messageFeed 数据库初始化迁移回滚
-- 文件名：000001_init_schema.down.sql
-- 本文件用于回滚 000001_init_schema.up.sql 创建的数据库结构。

-- 删除触发器
DROP TRIGGER IF EXISTS update_users_updated_at ON users;

-- 删除触发器函数
DROP FUNCTION IF EXISTS update_updated_at_column();

-- 删除索引
DROP INDEX IF EXISTS idx_users_created_at;
DROP INDEX IF EXISTS idx_users_username;

-- 删除表
DROP TABLE IF EXISTS users;
