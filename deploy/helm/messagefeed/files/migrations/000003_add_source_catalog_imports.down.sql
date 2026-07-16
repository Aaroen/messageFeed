-- Helm 打包副本；源迁移位于项目 migrations 目录。
DROP TRIGGER IF EXISTS update_source_import_jobs_updated_at ON source_import_jobs;
DROP TABLE IF EXISTS source_import_jobs;

DROP TRIGGER IF EXISTS update_source_catalog_entries_updated_at ON source_catalog_entries;
DROP TABLE IF EXISTS source_catalog_entries;
