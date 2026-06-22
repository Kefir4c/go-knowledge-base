/*
ШАГ 1.4: БАЗОВАЯ ФИЛЬТРАЦИЯ (WHERE) — РАСШИРЕННАЯ ВЕРСИЯ

Логика: В таблице 1 млн строк. Тянуть все в Go — самоубийство.
Нужно уметь искать по конкретным условиям.

ЧТО МЫ УЧИМ В ЭТОМ БЛОКЕ:
  1. Операторы сравнения: =, !=, <, >, <=, >=
  2. Работа с NULL: IS NULL, IS NOT NULL
  3. Комбинации условий: AND, OR, NOT
  4. Приоритет операторов и скобки
  5. Сравнение разных типов данных

Связь с Go: Динамическое построение WHERE-части в Go (опциональные фильтры).
*/

-- ПОДГОТОВКА: СОЗДАЁМ ТАБЛИЦУ И НАПОЛНЯЕМ ДАННЫМИ

CREATE TABLE IF NOT EXISTS users_test(
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    age INT CHECK (age >= 0),
    city TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO users_test (name, email, age, city, is_active) VALUES
    ('Alice', 'alice@mail.com', 30, 'Moscow', TRUE),
    ('Bob', 'bob@mail.com', 25, 'SPb', TRUE),
    ('Charlie', 'charlie@mail.com', 35, 'Moscow', FALSE),
    ('Diana', 'diana@mail.com', 28, 'Kazan', TRUE),
    ('Eve', 'eve@mail.com', 22, 'Moscow', TRUE),
    ('Frank', 'frank@mail.com', 40, NULL, TRUE),
    ('Grace', 'grace@mail.com', 33, 'SPb', FALSE),
    ('Helen', 'helen@mail.com', 27, 'Kazan', TRUE),
    ('Ivan', 'ivan@mail.com', 29, 'Moscow', TRUE),
    ('John', 'john@mail.com', 45, NULL, TRUE);

-- 1. ОПЕРАТОРЫ СРАВНЕНИЯ (=, !=, <, >, <=, >=)
-- 1.1. Равно (=)
-- Находит точное совпадение. Для строк важно совпадение регистра.
SELECT * FROM users_test WHERE name = 'Alice';
SELECT * FROM users_test WHERE city = 'Moscow';
SELECT * FROM users_test WHERE is_active = true;

-- 1.2. Не равно (!= или <>)
SELECT * FROM users_test WHERE name != 'Eve';
SELECT * FROM users_test WHERE city <> 'Moscow';  -- альтернативный синтаксис

-- 1.3. Больше (>), Меньше (<), Больше или равно (>=), Меньше или равно (<=)
SELECT * FROM users_test WHERE age > 30;          -- больше 30
SELECT * FROM users_test WHERE age >= 30;         -- 30 и больше
SELECT * FROM users_test WHERE age < 25;          -- меньше 25
SELECT * FROM users_test WHERE age <= 25;         -- 25 и меньше

-- 1.4. Сравнение с датами
-- Все пользователи, зарегистрированные после 2026-01-01
SELECT * FROM users_test WHERE created_at > '2026-01-01';

-- 1.5. Сравнение строк (по алфавиту)
SELECT * FROM users_test WHERE name > 'Charlie'; -- имена, которые идут после 'Charlie'

-- 2. NULL (IS NULL, IS NOT NULL)

-- 2.1. Проверка на NULL
-- NULL означает "неизвестно" или "отсутствует"
SELECT * FROM users_test WHERE city IS NULL;

-- 2.2. Проверка на НЕ NULL
SELECT * FROM users_test WHERE city IS NOT NULL;

-- ❌ ОШИБКА НОВИЧКА №1: сравнение с NULL через =
-- SELECT * FROM users_test WHERE city = NULL; -- НЕ РАБОТАЕТ! Всегда FALSE
-- ❌ ОШИБКА НОВИЧКА №2: сравнение с NULL через !=
-- SELECT * FROM users_test WHERE city != NULL; -- НЕ РАБОТАЕТ! Всегда FALSE

-- 2.3. NULL не равен NULL
-- NULL = NULL → FALSE (в SQL это неизвестно)
-- Поэтому для проверки NULL используем ТОЛЬКО IS NULL / IS NOT NULL.

-- 3. КОМБИНАЦИИ УСЛОВИЙ (AND, OR, NOT)

-- 3.1. AND — все условия должны выполняться
SELECT * FROM users_test
WHERE city = 'Moscow' AND age > 20;

-- 3.2. OR — хотя бы одно условие
SELECT * FROM users_test
WHERE city = 'Moscow' OR city = 'SPb';

-- 3.3. NOT — отрицание условия
SELECT * FROM users_test
WHERE NOT city = 'Moscow';
--или
SELECT * FROM users_test
WHERE city != 'Moscow';

-- 3.4. ПРИОРИТЕТ ОПЕРАТОРОВ (важно!)
-- AND выполняется РАНЬШЕ, чем OR.
-- Всегда используй скобки, чтобы явно указать порядок.
-- ❌ ПЛОХО (без скобок) — читается как:
SELECT * FROM users_test
WHERE age >= 18 AND city = 'Moscow' OR city = 'SPb';

-- ✅ ХОРОШО (со скобками) — явный порядок:
SELECT * FROM users_test
WHERE age >= 18 AND (city = 'Moscow' OR city = 'SPb');

-- 3.5. NOT с AND/OR
SELECT * FROM users_test
WHERE NOT (city = 'Moscow' OR city = 'SPb'); -- все, кроме Москвы и SPb

-- 4. СРАВНЕНИЕ РАЗНЫХ ТИПОВ ДАННЫХ

-- 4.1. Строки сравниваются по алфавиту (с учётом кодировки)
SELECT * FROM users_test WHERE name > 'Diana'

-- 4.2. Булевы значения можно сравнивать как TRUE/FALSE
SELECT * FROM users_test WHERE is_active = TRUE;
SELECT * FROM users_test WHERE is_active = FALSE;
-- Можно и без = TRUE/FALSE, просто булево поле в условии:
SELECT * FROM users_test WHERE is_active;  -- то же, что is_active = TRUE
SELECT * FROM users_test WHERE NOT is_active;  -- то же, что is_active = FALSE

-- 4.3. Числа и строки сравниваются с приведением типов (если возможно)
-- PostgreSQL попытается привести строку к числу, но лучше так не делать.

-- 5. ПРАКТИЧЕСКИЕ ЗАДАНИЯ
-- УРОВЕНЬ 1: Простые условия (5 заданий)
-- 1.1. Найди пользователей с возрастом больше 30.
-- 1.2. Найди пользователей с именем 'Bob'.
-- 1.3. Найди пользователей из Москвы.
-- 1.4. Найди пользователей, у которых city NULL.
-- 1.5. Найди активных пользователей (is_active = TRUE).

-- УРОВЕНЬ 2: Комбинированные условия (5 заданий)
-- 2.1. Найди пользователей из Москвы или SPb, у которых возраст > 25.
-- 2.2. Найди пользователей с возрастом >= 25 И <= 35 (без BETWEEN).
-- 2.3. Найди активных пользователей из Москвы.
-- 2.4. Найди пользователей, у которых возраст < 25 ИЛИ city NULL.
-- 2.5. Найди пользователей, которые НЕ из Москвы и НЕ из SPb.

-- УРОВЕНЬ 3: Сложные комбинации (5 заданий)
-- 3.1. Найди пользователей, у которых имя начинается с 'A' или 'B' (без LIKE, только =).
--       (Подсказка: используй OR с несколькими условиями =)
-- 3.3. Найди пользователей, у которых город NULL или возраст < 25 ИЛИ активны.
-- 3.4. Найди пользователей, у которых возраст >= 30 И город не NULL И активны.
-- 3.5. Найди пользователей, у которых email содержит 'mail' (без LIKE, используй = и OR с несколькими значениями).

-- 1.1
SELECT * FROM users_test WHERE age > 30;

-- 1.2
SELECT * FROM users_test WHERE name = 'Bob';

-- 1.3
SELECT * FROM users_test WHERE city = 'Moscow';

-- 1.4
SELECT * FROM users_test WHERE city IS NULL;

-- 1.5
SELECT * FROM users_test WHERE is_active = TRUE;

-- 2.1
SELECT * FROM users_test
WHERE city IN ('Moscow', 'SPb') AND age > 25;

-- 2.2
SELECT * FROM users_test WHERE age >= 25 AND age <= 35;

-- 2.3
SELECT * FROM users_test WHERE is_active = TRUE AND city = 'Moscow';

-- 2.4
SELECT * FROM users_test WHERE age < 25 OR city IS NULL;

-- 2.5
SELECT * FROM users_test WHERE city NOT IN ('Moscow', 'SPb');

-- 3.1
SELECT * FROM users_test
WHERE name = 'Alice' OR name = 'Bob';

-- 3.2
SELECT * FROM users_test
WHERE city IS NULL OR age < 25 OR is_active = TRUE;

-- 3.3
SELECT * FROM users_test
WHERE age >= 30 AND city IS NOT NULL AND is_active = TRUE;

-- 3.4 (вариант, но лучше использовать LIKE в 1.5)
SELECT * FROM users_test
WHERE email = 'alice@mail.com' OR email = 'bob@mail.com' OR email = 'charlie@mail.com';