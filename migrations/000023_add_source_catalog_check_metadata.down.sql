ALTER TABLE source_catalog_entries
    DROP CONSTRAINT IF EXISTS chk_source_catalog_entries_last_check_http_status;

ALTER TABLE source_catalog_entries
    DROP COLUMN IF EXISTS last_check_content_type,
    DROP COLUMN IF EXISTS last_check_http_status;
