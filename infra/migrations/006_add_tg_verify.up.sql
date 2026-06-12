ALTER TABLE application_admins
    ADD COLUMN role VARCHAR(16) NOT NULL DEFAULT 'moderator'
        CHECK (role IN ('admin', 'moderator'));

ALTER TABLE moderator
    DROP COLUMN role;

ALTER TABLE application
    ALTER COLUMN status SET DEFAULT 'inactive';

CREATE TABLE moderator_account
(
    id                 UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    moderator_id       UUID        NOT NULL REFERENCES moderator (id) ON DELETE CASCADE,
    platform           VARCHAR(16) NOT NULL CHECK (platform IN ('vk', 'telegram', 'api')),
    verification_token VARCHAR(64), -- одноразовый токен для подтверждения
    token_expires_at   TIMESTAMPTZ, -- срок жизни токена
    verified_at        TIMESTAMPTZ, -- дата успешной верификации
    account_id         VARCHAR(64),

    UNIQUE (moderator_id, account_id)
);

ALTER TABLE application
    ADD COLUMN own_acc_id UUID REFERENCES moderator_account (id) ON DELETE SET NULL;

COMMENT ON TABLE moderator_account IS 'Учётные записи модераторов на внешних платформах';
COMMENT ON COLUMN moderator_account.verification_token IS 'Одноразовый токен для подтверждения аккаунта';
COMMENT ON COLUMN moderator_account.token_expires_at IS 'Срок действия токена подтверждения';
COMMENT ON COLUMN moderator_account.verified_at IS 'Дата успешной верификации аккаунта';
