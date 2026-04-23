-- Миграция 003: Добавление поля banned_words в настройки приложений

-- Добавляем столбец banned_words в таблицу application_settings
ALTER TABLE application_settings
    ADD COLUMN banned_words TEXT[] DEFAULT NULL;

-- Комментарий к новому столбцу
COMMENT ON COLUMN application_settings.banned_words IS 'Список запрещенных слов для фильтрации';