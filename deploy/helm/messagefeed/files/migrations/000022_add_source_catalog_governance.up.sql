ALTER TABLE source_catalog_entries
    ADD COLUMN IF NOT EXISTS license_status VARCHAR(32) NOT NULL DEFAULT 'unknown',
    ADD COLUMN IF NOT EXISTS license_note TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS popularity_score INTEGER NOT NULL DEFAULT 0;

ALTER TABLE source_catalog_entries
    DROP CONSTRAINT IF EXISTS chk_source_catalog_entries_license_status;

ALTER TABLE source_catalog_entries
    ADD CONSTRAINT chk_source_catalog_entries_license_status
        CHECK (license_status IN ('unknown', 'allowed', 'restricted', 'blocked'));

ALTER TABLE source_catalog_entries
    DROP CONSTRAINT IF EXISTS chk_source_catalog_entries_popularity_score;

ALTER TABLE source_catalog_entries
    ADD CONSTRAINT chk_source_catalog_entries_popularity_score
        CHECK (popularity_score >= 0);

UPDATE source_catalog_entries
SET
    license_status = 'allowed',
    license_note = CASE
        WHEN license_note = '' THEN 'Official public feed catalog seed.'
        ELSE license_note
    END,
    popularity_score = CASE
        WHEN popularity_score = 0 AND official THEN 100
        ELSE popularity_score
    END
WHERE source_origin = 'official_seed';

CREATE INDEX IF NOT EXISTS idx_source_catalog_entries_license
    ON source_catalog_entries(license_status);

CREATE INDEX IF NOT EXISTS idx_source_catalog_entries_language_country
    ON source_catalog_entries(language, country);

CREATE INDEX IF NOT EXISTS idx_source_catalog_entries_popularity
    ON source_catalog_entries(popularity_score DESC);
