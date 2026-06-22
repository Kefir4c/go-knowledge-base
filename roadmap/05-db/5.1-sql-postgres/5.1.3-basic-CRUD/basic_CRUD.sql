/*
ШАГ 1.3: БАЗОВЫЙ CRUD (INSERT, SELECT, UPDATE, DELETE)

Логика: Таблица создана, теперь нужно научиться:
  1. Класть данные (INSERT)
  2. Доставать данные (SELECT)
  3. Обновлять данные (UPDATE)
  4. Удалять данные (DELETE)

Что мы разберем:
  - INSERT с RETURNING (возврат ID)
  - SELECT с WHERE, ORDER BY, LIMIT
  - UPDATE с WHERE
  - DELETE с WHERE

Связь с Go:
  - INSERT ... RETURNING → получаем ID созданной записи
  - SELECT → сканируем строки в структуру
  - sql.ErrNoRows → обрабатываем случай "пользователь не найден"
*/

-- ПОДГОТОВКА: СОЗДАЁМ ТАБЛИЦУ
-- Мы будем использовать таблицу users_test из прошлых уроков.
-- Если её нет — создадим заново.

CREATE TABLE IF NOT EXISTS users_test(
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    age SMALLINT CHECK (age >= 0),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 1. INSERT (вставка данных)
-- 1.1. Вставка одной записи
INSERT INTO users_test (name, email, age) VALUES ('Popka', 'Pikalov.r@yandex.ru',22)

-- 1.2. Вставка нескольких записей
INSERT INTO users_test (name, email, age) VALUES
    ('Bob', 'bob@mail.com', 25),
    ('Charlie', 'charlie@mail.com', 35),
    ('Diana', 'diana@mail.com', 28),
    ('Eve', 'eve@mail.com', 22),
    ('Frank', 'frank@mail.com', 40);

-- 1.3. INSERT с RETURNING (возвращает ID созданной записи)
INSERT INTO users_test (name, email, age)
VALUES ('Ivan', 'helen@mail.com', 35)
RETURNING *;

-- 2. SELECT (выборка данных)
-- 2.1. Все поля, все записи (без условий, без сортировки, без лимитов)
SELECT * FROM users_test;

-- 2.2. Только нужные поля
SELECT id, name, email FROM users_test;

-- 2.3. Выборка с простым условием (только по ID)
SELECT * FROM users_test WHERE id = 1;

-- 3. UPDATE (обновление данных)
-- 3.1. Обновление по ID (одно поле)
UPDATE users_test
SET age = 56
WHERE id = 1;

-- 3.2. Обновление по ID (несколько полей)
UPDATE users_test
SET name = 'Vova Bublik', age = 16
WHERE id = 1;

-- 3.3. Обновление с RETURNING (возвращает обновлённую запись)
UPDATE users_test
SET age = 33
WHERE id = 1
RETURNING *;

-- 4. DELETE (удаление данных)
-- 4.1. Удаление по ID
DELETE FROM users_test WHERE id = 12;

-- 4.2. Удаление с RETURNING (возвращает удалённые записи)
DELETE FROM users_test
WHERE id = 3
RETURNING *;

-- Пример:

CREATE TABLE IF NOT EXISTS products(
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    price NUMERIC(10,2) CHECK (price > 0),
    stock INT CHECK ( stock >= 0 ),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

INSERT INTO products (name, price, stock) VALUES
    ('Laptop', 1200.50, 10),
    ('Mouse', 25.99, 50),
    ('Keyboard', 45.00, 30),
    ('Monitor', 320.00, 15),
    ('USB Cable', 5.99, 100);

SELECT * FROM products WHERE price >= 50;

UPDATE products
SET price = price * 1.1;

INSERT INTO products (name, price, stock) VALUES
('Headphones', 75.00, 20)
RETURNING id;

DELETE FROM products WHERE id = 3
RETURNING *;

SELECT * FROM products ORDER BY id;