CREATE TABLE moderator_account
(
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    moderator_id       UUID        NOT NULL,
    platform           VARCHAR(16) NOT NULL CHECK (platform IN ('vk', 'telegram', 'api')),
    verification_token VARCHAR(64), -- одноразовый токен для подтверждения
    token_expires_at   TIMESTAMPTZ, -- срок жизни токена
    verified_at        TIMESTAMPTZ, -- дата успешной верификации
    account_id VARCHAR(64) NOT NULL,

    UNIQUE (moderator_id, account_id)
);
