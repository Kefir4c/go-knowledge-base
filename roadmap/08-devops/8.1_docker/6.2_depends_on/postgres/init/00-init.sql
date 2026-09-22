-- Создаём пользователя и БД заранее.
-- Этот скрипт выполняется при первой инициализации Postgres.
CREATE TABLE IF NOT EXISTS schema_version (
    version INT PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);