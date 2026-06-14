DROP INDEX IF EXISTS idx_moderator_account_verified_at;
DROP INDEX IF EXISTS idx_moderator_account_verification_token;
DROP INDEX IF EXISTS idx_moderator_account_platform_account;
DROP INDEX IF EXISTS idx_moderator_account_moderator;

ALTER TABLE application
    DROP COLUMN IF EXISTS own_acc_id;

ALTER TABLE message
    DROP COLUMN IF EXISTS message_id;

DROP TABLE IF EXISTS moderator_account;

ALTER TABLE application
    ALTER COLUMN status SET DEFAULT 'active';

ALTER TABLE moderator
    ADD COLUMN role VARCHAR(16) NOT NULL DEFAULT 'moderator'
        CHECK (role IN ('admin', 'moderator'));

ALTER TABLE application_admins
    DROP COLUMN IF EXISTS role;
