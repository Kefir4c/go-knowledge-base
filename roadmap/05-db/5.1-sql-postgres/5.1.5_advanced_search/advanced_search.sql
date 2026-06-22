/*
ШАГ 1.5: ПРОДВИНУТЫЙ ПОИСК (LIKE, ILIKE, BETWEEN, IN)

Логика: Нужно искать по части слова (например, имя начинается на "Алекс"),
по диапазону (например, возраст от 25 до 35) или по списку значений
(например, город Москва или СПб).

ЧТО МЫ УЧИМ В ЭТОМ БЛОКЕ:
  1. LIKE / ILIKE   — поиск по шаблону текста (%, _)
  2. BETWEEN        — поиск в диапазоне (включительно)
  3. IN             — поиск по списку значений

Связь с Go: Динамическое построение WHERE-части с опциональными фильтрами.
ВАЖНО: LIKE без индекса — нагрузка на базу.
*/

-- ПОДГОТОВКА: СОЗДАЁМ ТАБЛИЦУ И НАПОЛНЯЕМ ДАННЫМИ

CREATE TABLE users_test (
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

-- 1. LIKE / ILIKE — ПОИСК ПО ШАБЛОНУ ТЕКСТА

-- 1.1. LIKE — с учётом регистра
-- Начинается с 'A'
SELECT * FROM users_test WHERE name LIKE 'A%';

-- Заканчивается на 'e'
SELECT * FROM users_test WHERE name LIKE '%e';

-- Содержит 'li'
SELECT * FROM users_test WHERE name LIKE '%li%'

-- 1.2. Поиск с экранированием специальных символов (%, _)
-- Если нужно найти строку, содержащую символ '%' или '_', их нужно экранировать.
-- Добавим тестовую запись:
INSERT INTO users_test (name, email, age, city) 
VALUES ('Test_100%', 'test@mail.com', 20, 'TestCity');

-- Поиск с экранированием (используем ESCAPE)
SELECT * FROM users_test
WHERE name LIKE '%\%%' ESCAPE '\'; -- ищем имена, содержащие символ %

SELECT * FROM users_test
WHEN name LIKE '%\_%' ESCAPE '\';  -- ищем имена, содержащие символ _

-- 1.3. LIKE с учётом регистра (по умолчанию регистрозависимый)
SELECT * FROM users_test WHERE name LIKE 'alice%';  -- не найдёт (регистр не совпадает)

-- 1.4. ILIKE (регистронезависимый) — стандарт в PostgreSQL
SELECT * FROM users_test WHERE name ILIKE 'alice%'; -- найдёт Alice

-- 1.5. LIKE с отрицанием (NOT LIKE)
SELECT * FROM users_test WHERE name NOT LIKE 'A%'; -- все, чьи имена не на 'A'

-- 1.6. LIKE с несколькими условиями
SELECT * FROM users_test
WHERE name LIKE 'A%' OR name LIKE 'C%' OR name LIKE 'E%';

-- 1.7. LIKE и NULL (NULL не подходит ни под какой LIKE)
SELECT * FROM users_test name LIKE '%' -- все строки, где name NOT NULL

-- 2. BETWEEN — РАСШИРЕННЫЕ ВОЗМОЖНОСТИ

-- 2.1. BETWEEN с датами (включая приведение типов)
SELECT * FROM users_test
WHERE name BETWEEN '2026-01-01' AND '2026-12-31';

-- 2.2. BETWEEN с текстом (по алфавиту)
SELECT * FROM users_test
WHERE name BETWEEN 'Alice' AND 'Eve'; -- имена от Alice до Eve включительно

-- 2.3. NOT BETWEEN (вне диапазона)
SELECT * FROM users_test
WHERE age NOT BETWEEN 25 AND 35; 

-- 2.4. BETWEEN с NULL (если одна граница NULL, результат NULL)
SELECT * FROM users_test
WHERE age BETWEEN NULL AND 50;

-- 2.5. BETWEEN с выражением
SELECT * FROM users_test
WHERE age BETWEEN age +\- 4 AND age + 6; -- странно, но синтаксически верно

-- 2.6. BETWEEN с подзапросом (редко, но бывает)
SELECT * FROM users_test
WHERE age BETWEEN (SELECT AVG(age) - 5 FROM users_test)
            AND (SELECT AVG(age) + 5 FROM users_test); -- Тяжёлый запрос

-- Оптимизированный
WITH avg_age AS (
    SELECT AVG(age) AS avg_val FROM users_test
)    
SELECT u.*
FROM users_test u, avg_age a
WHERE u.age BETWEEN a.avg_val - 5 AND a.avg_val + 5;

-- 3. IN — РАСШИРЕННЫЕ ВОЗМОЖНОСТИ

-- 3.1. IN с подзапросом (получаем города, в которых есть активные пользователи)
SELECT * FROM users_test
WHERE city IN (SELECT DISTINCT city FROM users_test WHERE is_active = TRUE);

-- 3.2. Пользователи из городов, где есть пользователи старше 35
SELECT * FROM users_test
WHERE city IN (SELECT DISTINCT city FROM users_test WHERE age > 35);

-- 3.3. Пользователи, которые НЕ из городов, где есть пользователи старше 30
SELECT * FROM users_test
WHERE city NOT IN(SELECT DISTINCT city FROM users_test WHERE age > 30);

-- 4. КОМБИНАЦИИ (LIKE + BETWEEN + IN) — РАЗНЫЕ СЦЕНАРИИ

-- 4.1. Комбинация LIKE + BETWEEN (имя на 'A' и возраст от 25 до 35)
SELECT * FROM users_test
WHERE LIKE 'A%' AND age BETWEEN 25 AND 35;

-- 4.2. LIKE + IN (имя содержит 'li' ИЛИ город Moscow)
SELECT * FROM users_test
WHERE name LIKE '%li%' OR IN ('Moscow');

-- 4.3. LIKE + BETWEEN + IN (все три вместе)
SELECT * FROM users_test 
WHERE name ILIKE 'a%' 
  AND age BETWEEN 25 AND 40 
  AND city IN ('Moscow', 'SPb', 'Kazan');

-- 4.4. Сложный фильтр с отрицанием
SELECT * FROM users_test 
WHERE name NOT LIKE 'A%' 
  AND age NOT BETWEEN 25 AND 35 
  AND city NOT IN ('Moscow', 'SPb');

-- Решение примеров

-- Найди пользователей из городов, где есть хотя бы один активный 
--пользователь, И у них возраст больше среднего возраста по всем пользователям.
SELECT * FROM users_test
WHERE city IN (SELECT DISTINCT city FROM USERS_test AND WHERE is_active = TRUE)
AND age > (SELECT AVG(age) FROM users_test);

-- ПОЛЬЗОВАТЕЛИ СТАРШЕ СРЕДНЕГО ВОЗРАСТА В СВОЁМ ГОРОДЕ
SELECT u1.*
FROM users_test u1
WHERE u1.age > (SELECT AVG(u2.age) FROM users_test u2 WHERE u2.city = u1.city)
ORDER BY u1.city, u1.age DESC;

-- Найди пользователей, у которых есть другой 
-- пользователь с таким же возрастом (EXISTS + SELF JOIN).
SELECT * FROM users_test u1
WHERE EXISTS (SELECT 1 FROM users_test u2 WHERE u2.age = u1.age AND u2.age != u1.age)