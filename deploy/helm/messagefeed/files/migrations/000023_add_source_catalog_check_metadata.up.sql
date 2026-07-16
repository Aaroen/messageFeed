ALTER TABLE source_catalog_entries
    ADD COLUMN IF NOT EXISTS last_check_http_status INTEGER,
    ADD COLUMN IF NOT EXISTS last_check_content_type TEXT NOT NULL DEFAULT '';

ALTER TABLE source_catalog_entries
    DROP CONSTRAINT IF EXISTS chk_source_catalog_entries_last_check_http_status;

ALTER TABLE source_catalog_entries
    ADD CONSTRAINT chk_source_catalog_entries_last_check_http_status
        CHECK (last_check_http_status IS NULL OR last_check_http_status BETWEEN 100 AND 599);
