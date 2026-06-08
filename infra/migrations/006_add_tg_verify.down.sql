DROP INDEX IF EXISTS idx_moderator_account_verified_at;
DROP INDEX IF EXISTS idx_moderator_account_verification_token;
DROP INDEX IF EXISTS idx_moderator_account_platform_account;
DROP INDEX IF EXISTS idx_moderator_account_moderator;

DROP TABLE IF EXISTS moderator_account;

ALTER TABLE application_admins
    DROP COLUMN IF EXISTS role;
