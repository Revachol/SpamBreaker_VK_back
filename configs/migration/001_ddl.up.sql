-- Миграция 001: Создание начальной схемы базы данных

-- Подключаем расширение для генерации UUID
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Схема данных для бота
CREATE SCHEMA asp_bot;

-- Таблица сообщений
CREATE TABLE message
(
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    text VARCHAR(255) NOT NULL,
    status VARCHAR(16) NOT NULL,
    toxicity_score INT,
    created_at TIMESTAMPZ NOT NULL DEFAULT NOW()
);
