DROP INDEX IF EXISTS idx_auth_invite_redemptions_invite;
DROP TABLE IF EXISTS auth_invite_redemptions;

DROP TRIGGER IF EXISTS update_auth_invite_codes_updated_at ON auth_invite_codes;
DROP INDEX IF EXISTS idx_auth_invite_codes_created_by;
DROP TABLE IF EXISTS auth_invite_codes;

ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
