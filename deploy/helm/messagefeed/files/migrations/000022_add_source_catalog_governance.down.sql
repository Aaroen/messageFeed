DROP INDEX IF EXISTS idx_source_catalog_entries_popularity;
DROP INDEX IF EXISTS idx_source_catalog_entries_language_country;
DROP INDEX IF EXISTS idx_source_catalog_entries_license;

ALTER TABLE source_catalog_entries
    DROP CONSTRAINT IF EXISTS chk_source_catalog_entries_popularity_score;

ALTER TABLE source_catalog_entries
    DROP CONSTRAINT IF EXISTS chk_source_catalog_entries_license_status;

ALTER TABLE source_catalog_entries
    DROP COLUMN IF EXISTS popularity_score,
    DROP COLUMN IF EXISTS license_note,
    DROP COLUMN IF EXISTS license_status;
