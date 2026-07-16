-- Helm 打包副本；源迁移位于项目 migrations 目录。
SELECT setval(
    pg_get_serial_sequence('users', 'id'),
    GREATEST(COALESCE((SELECT MAX(id) FROM users), 1), 1),
    true
);
