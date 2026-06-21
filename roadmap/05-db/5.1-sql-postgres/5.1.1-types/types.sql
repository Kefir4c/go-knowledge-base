/*
ШАГ 1.1: ТИПЫ ДАННЫХ И ИХ РАЗМЕРНОСТЬ

Логика: Прежде чем создать таблицу, нужно знать, из чего делать столбцы.
Ошибка в типе данных = проблемы с производительностью или потеря точности
(особенно с деньгами).

Что мы разберем:
  1. Целые числа (SMALLINT, INTEGER, BIGINT, SERIAL)
  2. Числа с плавающей точкой (REAL, DOUBLE PRECISION, NUMERIC)
  3. Строки (CHAR, VARCHAR, TEXT)
  4. Дата и время (DATE, TIME, TIMESTAMP, TIMESTAMPTZ)
  5. UUID
  6. JSON / JSONB
  7. BYTEA (бинарные данные)
  8. BOOLEAN и другие

Связь с Go:
  BIGINT  → int64
  NUMERIC → string (чтобы не потерять точность)
  TEXT    → string
  TIMESTAMPTZ → time.Time
  UUID    → uuid.UUID (github.com/google/uuid)
  JSONB   → map[string]interface{} или struct
  BYTEA   → []byte
*/

-- 1. ЦЕЛЫЕ ЧИСЛА (INTEGER TYPES)

-- Таблица с размерами:
--   SMALLINT   (2 байта)  -32 768 .. +32 767 (возраст, рейтинг)
--   INTEGER    (4 байта)  -2.1 млрд .. +2.1 млрд (стандартный ID)
--   BIGINT     (8 байт)   -9.2e18 .. +9.2e18 (ID для больших таблиц)
--   SERIAL     (4 байта)  автоинкремент (синтаксический сахар)
--   BIGSERIAL  (8 байт)   автоинкремент для больших таблиц

-- Пример создания таблицы с целыми числами:
CREATE TABLE IF NOT EXISTS test_integers(
    id_small SMALLINT,
    id_int INTEGER,
    id_big BIGINT,
    id_serial SERIAL PRIMARY KEY 
);

-- Вставляем данные (проверяем лимиты)
INSERT INTO test_integers (id_small, id_int, id_big)
VALUES (32767, 2147483647, 9223372036854775807);


-- Ошибка! (раскомментируй, чтобы проверить):
INSERT INTO test_integers (id_small) VALUES (32768); -- out of range

SELECT * FROM test_integers;

DROP TABLE IF EXISTS test_integers;


-- 2. ЧИСЛА С ПЛАВАЮЩЕЙ ТОЧКОЙ (FLOATING-POINT TYPES)

--   REAL             (4 байта)  неточное представление (float32)
--   DOUBLE PRECISION (8 байт)   неточное, точнее (float64)
--   NUMERIC(p,s)     (перем.)   точное (для денег!)

-- ⚠️ ОШИБКА НОВИЧКА: использовать REAL или DOUBLE для денег!

-- Пример неточности:
SELECT
    0.1::REAL + 0.2::REAL AS real_sum,
    0.1::DOUBLE PRECISION + 0.2::DOUBLE PRECISION doble_sum,
    0.1::NUMERIC(10,2) + 0.2::NUMERIC(10,2) AS numeric_sum;


-- ПРАВИЛО: Для денег используй NUMERIC(10,2) или BIGINT (копейки).

-- Пример для денег:
CREATE TABLE IF NOT EXISTS test_money(
    id BIGSERIAL PRIMARY KEY,
    amount NUMERIC(10,2) NOT NULL
);

INSERT INTO test_money (amount) VALUES (1000.50);
INSERT INTO test_money (amount) VALUES (99999999.99);

-- INSERT INTO test_money (amount) VALUES (999999999.99); -- Ошибка! Превышает 10 цифр.

SELECT * FROM test_money;
DROP TABLE IF EXISTS test_money;


-- 3. СТРОКИ (CHARACTER TYPES)

--   CHAR(n)    (фиксированный)  Ровно n символов (почти не используется)
--   VARCHAR(n) (длина + 1)      Максимум n символов (если нужно ограничение)
--   TEXT       (длина + 1)      Неограниченно (ПРЕДПОЧТИТЕЛЬНО)

-- 🔥 ВАЖНО: В PostgreSQL VARCHAR (без n) и TEXT — это одно и то же!
-- Ограничение длины (VARCHAR(255)) не экономит память, только валидирует.

CREATE TABLE IF NOT EXISTS test_strings (
    id BIGSERIAL PRIMARY KEY,
    fixed CHAR(5),
    limited VARCHAR(255),
    unlimited TEXT
);  

INSERT INTO test_strings (fixed, limited, unlimited)
VALUES ('Hi', 'Hello', 'This is a very long text that can be as long as we want');

-- CHAR(5) дополняется пробелами (убедись сам):
SELECT length(fixed) AS fixed_len, fixed FROM test_strings;

-- Попытка вставить слишком длинную строку:
-- INSERT INTO test_strings (limited) VALUES ('12345678901'); -- Ошибка! Превышает VARCHAR(10).

-- 4. ДАТА И ВРЕМЯ (DATE/TIME TYPES)

--   DATE         (4 байта)   только дата
--   TIME         (8 байт)    только время
--   TIMESTAMP    (8 байт)    дата и время БЕЗ часового пояса (НЕ ИСПОЛЬЗУЙ!)
--   TIMESTAMPTZ  (8 байт)    дата и время С ЧАСОВЫМ ПОЯСОМ (ВСЕГДА ИСПОЛЬЗУЙ!)

-- 🔥 ВАЖНО: Всегда используй TIMESTAMPTZ!
-- В Go это маппится на time.Time.

CREATE TABLE IF NOT EXISTS(
    id BIGSERIAL PRIMARY KEY,
    event_date DATE,
    event_time TIME,
    ts_no_tz TIMESTAMP,      -- БЕЗ зоны (не используй!)
    ts_with_tz TIMESTAMPTZ   -- С зоной (всегда используй!)
);

INSERT INTO test_dates (event_date, event_time, ts_no_tz, ts_with_tz)
VALUES ('2026-06-20', '14:30:00', NOW(), NOW());

SELECT * FROM test_dates;


-- Посмотри, как TIMESTAMPTZ преобразуется в твой локальный часовой пояс:
SHOW TIMEZONE; -- покажет твою текущую зону

-- 5. UUID (UNIVERSALLY UNIQUE IDENTIFIER)

--   UUID (16 байт)  123e4567-e89b-12d3-a456-426614174000
--   Для распределённых систем.

-- В Go: github.com/google/uuid
-- В PostgreSQL: gen_random_uuid() (доступно с версии 13)

CREATE TABLE IF NOT EXISTS test_uuids (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT
);

INSERT INTO test_uuids (name) VALUES ('Alice'), ('Bob');
SELECT * FROM test_uuids;

-- 6. JSON / JSONB

--   JSON  (текстовое)  сохраняет порядок, медленнее
--   JSONB (бинарное)   быстрее, поддерживает индексы (GIN)

-- 🔥 ВАЖНО: Всегда используй JSONB!

CREATE TABLE IF NOT EXISTS test_jsonb(
    id BIGSERIAL PRIMARY KEY,
    data JSONB
);

INSERT INTO test_jsonb (data) VALUES
    ('{"name": "Alice", "age": 30, "tags": ["go", "postgres"]}'),
    ('{"name": "Bob", "age": 25, "tags": ["python"]}');

-- Поиск внутри JSONB
SELECT * from test_jsonb WHERE data @> '{"tags" : ["go"]}';

-- Создаём индекс GIN для быстрого поиска
CREATE INDEX idx_test_jsonb_data ON test_jsonb USING GIN (data);

-- Проверяем, что индекс используется
EXPLAIN SELECT * FROM test_jsonb WHERE data @> '{"tags" : ["go"]}';



-- 7. BYTEA (БИНАРНЫЕ ДАННЫЕ)

--   BYTEA (длина + 4)  для картинок, файлов (до нескольких МБ)
--   В Go: []byte

CREATE TABLE IF NOT EXISTS test_files (
    id BIGSERIAL PRIMARY KEY,
    content BYTEA
);

-- Вставляем бинарные данные (в реальной жизни — из Go)
INSERT INTO test_files (content) VALUES (E'\\xdeadbeef');

SELECT * FROM test_files;

-- 8. BOOLEAN (TRUE / FALSE)

CREATE TABLE IF NOT EXISTS test_bool(
    id BIGSERIAL PRIMARY KEY,
    is_active BOOLEAN DEFAULT TRUE
);

INSERT INTO test_bool (is_active) VALUES (TRUE), (FALSE);

SELECT * FROM test_bool;

-- ПРАКТИЧЕСКОЕ ЗАДАНИЕ (КРИТЕРИЙ ПРОХОЖДЕНИЯ)

-- Задача: Создать таблицу users_test с 7 колонками.
-- Требования:
--   id BIGSERIAL PRIMARY KEY
--   balance NUMERIC(10,2) NOT NULL
--   name VARCHAR(255) NOT NULL
--   bio TEXT
--   is_banned BOOLEAN DEFAULT FALSE
--   registered_at TIMESTAMPTZ NOT NULL
--   session_uuid UUID UNIQUE

-- Дополнительно:
--   - Вставить одну тестовую запись.
--   - Выполнить SELECT.
--   - Измерить размер таблицы.

-- Критерий прохождения:
--   - Скрипт выполняется без ошибок.
--   - В таблице одна строка.
--   - Размер таблицы меньше 1 МБ.

-- ВЫПОЛНЕНИЕ ЗАДАНИЯ (запускай по порядку)

-- 1. Создаём таблицу

-- 2. Вставляем тестовую запись

-- 3. Проверяем

-- 4. Смотрим размер таблицы

-- 5. (Опционально) Смотрим размер каждой колонки

CREATE TABLE IF NOT EXISTS users_test(
    id BIGSERIAL PRIMARY KEY,
    balance NUMERIC(10,2) NOT NULL,
    name VARCHAR(255) NOT NULL,
    bio TEXT,
    is_banned BOOLEAN DEFAULT FALSE,
    register_at TIMESTAMPTZ NOT NULL,
    session_uid UUID UNIQUE
);

INSERT INTO users_test (balance, name, bio, register_at, session_uid)
VALUES (215632.32, 'Pypyni', 'I am gei, ooooo yes, mami', NOW(), gen_random_uuid());

SELECT * FROM users_test;

SELECT pg_size_pretty(pg_relation_size("users_test"));

SELECT pg_column_size(id) AS id_size,
       pg_column_size(balance) AS balance_size,
       pg_column_size(name) AS name_size,
       pg_column_size(bio) AS bio_size,
       pg_column_size(register_at) AS created_size
FROM users_test
LIMIT 1;



-- ЭКСПЕРИМЕНТЫ ДЛЯ ЗАКРЕПЛЕНИЯ

-- Эксперимент 1: NUMERIC vs DOUBLE PRECISION

-- Обрати внимание: NUMERIC = 0.30, DOUBLE = 0.30000000000000004

-- Эксперимент 2: TIMESTAMP vs TIMESTAMPTZ


-- Эксперимент 3: JSONB и GIN индекс (уже сделали выше)
-- Проверь, что EXPLAIN показывает Index Scan
