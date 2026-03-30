-- Откат миграции 002

DROP TABLE IF EXISTS moderation_log;

ALTER TABLE message
    DROP COLUMN IF EXISTS sender_id,
    DROP COLUMN IF EXISTS application_id;

DROP TABLE IF EXISTS application_settings;
DROP TABLE IF EXISTS application;
DROP TABLE IF EXISTS moderator;