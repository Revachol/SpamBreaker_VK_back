-- Миграция 002: Расширение схемы — модераторы, приложения, настройки, логи

-- ─── Модераторы ─────────────────────────────────────────────────────────────
-- Люди, которые администрируют систему через панель управления.
CREATE TABLE moderator
(
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    username      VARCHAR(64) NOT NULL UNIQUE,
    email         VARCHAR(255)         UNIQUE,
    password_hash VARCHAR(255),                          -- bcrypt / argon2
    role          VARCHAR(16) NOT NULL DEFAULT 'moderator'
                              CHECK (role IN ('admin', 'moderator')),
    is_active     BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Приложения ─────────────────────────────────────────────────────────────
-- Каждый подключённый чат / группа / компания — отдельное приложение.
-- Токен используется ботом для авторизации запросов к Core API.
CREATE TABLE application
(
    id          UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name        VARCHAR(255) NOT NULL,
    platform    VARCHAR(16)  NOT NULL
                             CHECK (platform IN ('vk', 'telegram', 'api')),
    external_id VARCHAR(128),                            -- ID чата/группы во внешней платформе
    token       VARCHAR(512) NOT NULL UNIQUE,            -- секретный токен для Core API
    owner_id    UUID         REFERENCES moderator (id) ON DELETE SET NULL,
    status      VARCHAR(16)  NOT NULL DEFAULT 'active'
                             CHECK (status IN ('active', 'suspended', 'inactive')),
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_application_platform ON application (platform);
CREATE INDEX idx_application_status   ON application (status);

-- ─── Настройки приложения ────────────────────────────────────────────────────
-- Каждое приложение имеет ровно одну запись настроек (1-to-1).
CREATE TABLE application_settings
(
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id      UUID        NOT NULL UNIQUE
                                    REFERENCES application (id) ON DELETE CASCADE,

    -- Порог токсичности (0–100). Сообщения с toxicity_score >= порога считаются спамом.
    toxicity_threshold  INT         NOT NULL DEFAULT 70
                                    CHECK (toxicity_threshold BETWEEN 0 AND 100),

    -- Что делать при обнаружении спама.
    action_on_spam      VARCHAR(32) NOT NULL DEFAULT 'notify'
                                    CHECK (action_on_spam IN ('notify', 'delete', 'ban', 'shadow_ban')),

    -- Автоматически применять action_on_spam без участия модератора.
    auto_moderate       BOOLEAN     NOT NULL DEFAULT FALSE,

    -- Отправлять уведомление модераторам при обнаружении спама.
    notify_moderator    BOOLEAN     NOT NULL DEFAULT TRUE,

    -- Языки, на которых работает фильтр (NULL = все).
    allowed_languages   VARCHAR(8)[] DEFAULT NULL,

    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- ─── Обновление таблицы сообщений ───────────────────────────────────────────
-- Добавляем связь сообщения с приложением, из которого оно пришло.
ALTER TABLE message
    ADD COLUMN application_id UUID REFERENCES application (id) ON DELETE SET NULL,
    ADD COLUMN sender_id      VARCHAR(128);  -- ID отправителя во внешней платформе

CREATE INDEX idx_message_application ON message (application_id);
CREATE INDEX idx_message_created_at  ON message (created_at DESC);

-- ─── Лог модерации ──────────────────────────────────────────────────────────
-- Фиксируем каждое действие — как автоматическое, так и ручное.
CREATE TABLE moderation_log
(
    id             UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    application_id UUID        REFERENCES application (id) ON DELETE SET NULL,
    message_id     UUID        REFERENCES message (id)     ON DELETE SET NULL,
    -- NULL означает автоматическое действие системы, а не конкретного модератора.
    moderator_id   UUID        REFERENCES moderator (id)   ON DELETE SET NULL,

    action         VARCHAR(32) NOT NULL
                               CHECK (action IN (
                                   'auto_flagged',   -- система отметила как подозрительное
                                   'auto_deleted',   -- система удалила автоматически
                                   'manual_approved',-- модератор одобрил (снял флаг)
                                   'manual_deleted', -- модератор удалил вручную
                                   'manual_banned',  -- модератор забанил отправителя
                                   'manual_warned'   -- модератор вынес предупреждение
                               )),

    -- Человекочитаемая причина действия (опционально).
    reason         TEXT,
    -- Снапшот вердикта на момент действия, чтобы история не менялась.
    verdict_label  VARCHAR(16),
    verdict_score  INT,         -- toxicity_score на момент действия (0–100)

    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_modlog_application ON moderation_log (application_id);
CREATE INDEX idx_modlog_moderator   ON moderation_log (moderator_id);
CREATE INDEX idx_modlog_created_at  ON moderation_log (created_at DESC);