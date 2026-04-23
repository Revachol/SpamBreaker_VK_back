-- Миграция 003: Откат добавления поля banned_words в настройки приложений

-- Удаляем столбец banned_words из таблицы application_settings
ALTER TABLE application_settings
    DROP COLUMN banned_words;