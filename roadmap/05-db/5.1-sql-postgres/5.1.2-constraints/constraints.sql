/*
ШАГ 1.2: СОЗДАНИЕ ТАБЛИЦЫ И ОГРАНИЧЕНИЯ (CONSTRAINTS)

Логика: Таблица есть, но нужно защитить её от мусора. 
Ограничения (Constraints) — это правила, которые база данных сама проверяет.
Если правило нарушено, база не даст вставить/обновить данные и вернёт ошибку.

Зачем это нужно?
  - Не дать сохранить пользователя без email.
  - Не дать сделать баланс отрицательным.
  - Не дать создать двух пользователей с одинаковым email.
  - Не дать удалить категорию, если есть товары в ней.

Мы разберем 6 основных ограничений:
  1. NOT NULL  — поле обязательно для заполнения.
  2. UNIQUE    — значение не должно повторяться.
  3. PRIMARY KEY — уникальный идентификатор строки (UNIQUE + NOT NULL).
  4. CHECK     — проверка условия (например, возраст >= 18).
  5. DEFAULT   — значение по умолчанию, если не указано.
  6. FOREIGN KEY — связь с другой таблицей.

Связь с Go:
  Каждая ошибка ограничения имеет свой код в PostgreSQL.
  В Go (pgx) ты можешь проверить код ошибки:
    - "23505"  → нарушение UNIQUE (дубликат)
    - "23502"  → нарушение NOT NULL
    - "23503"  → нарушение FOREIGN KEY (нет такого ID в родительской таблице)
    - "23514"  → нарушение CHECK
  Это позволяет обрабатывать ошибки по-разному в коде.
*/

-- 1. NOT NULL (поле обязательно)
-- Говорит: "Эту колонку нельзя оставить пустой".

CREATE TABLE IF NOT EXISTS users_notnull (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    email TEXT NOT NULL
);

-- Попробуем вставить корректную строку:
INSERT INTO users_notnull (name) VALUES ('Kolya');

-- Запрос выполнится успешно.

-- Попробуем вставить NULL в NOT NULL колонку:
INSERT INTO users_notnull (name) VALUES (NULL); -- ОШИБКА!

-- Что происходит в Go?
--   При попытке вставить NULL в NOT NULL, pgx вернёт ошибку с кодом "23502".
--   В коде можно проверить: if errCode == "23502" { ... }

-- 2. UNIQUE (уникальность)
-- Говорит: "Значение в этой колонке не должно повторяться".

CREATE TABLE IF NOT EXISTS users_unique(
    id BIGSERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL
);

INSERT INTO users_unique (email) VALUES ('alice@mail.com');
-- Вторая вставка с тем же email вызовет ошибку:
-- INSERT INTO users_unique (email) VALUES ('alice@mail.com'); -- ОШИБКА!

-- Код ошибки в PostgreSQL: "23505" (duplicate key value).

-- 3. PRIMARY KEY (первичный ключ)
-- Это комбинация UNIQUE + NOT NULL.
-- Каждая таблица должна иметь PRIMARY KEY (это хорошая практика).
-- Обычно это ID — автоинкрементное число или UUID.

CREATE TABLE IF NOT EXISTS users_pk (
    id BIGSERIAL PRIMARY KEY,   -- PRIMARY KEY = UNIQUE + NOT NULL
    name TEXT NOT NULL
);

-- Попытка вставить дубликат id вызовет ошибку (как UNIQUE).
-- Попытка вставить NULL вызовет ошибку (как NOT NULL).

-- 4. CHECK (проверка условия)
-- Говорит: "Значение должно удовлетворять условию".

CREATE TABLE IF NOT EXISTS users_check(
    id BIGSERIAL PRIMERY KEY,
    name TEXT NOT NULL,
    age SMALLINT CHECK (age >= 18 )
);

INSERT INTO users_check (name, age) VALUES ('Bob', 25); -- OK
-- INSERT INTO users_check (name, age) VALUES ('Bob', 17); -- ОШИБКА! (CHECK)

-- Можно добавить несколько CHECK:
CREATE TABLE IF NOT EXISTS products_check (
    id BIGSERIAL PRIMARY KEY,
    price NUMERIC(10,2) CHECK (price > 0),     -- цена больше нуля
    quantity INT CHECK (quantity >= 0)         -- количество не отрицательное
);

-- Код ошибки в PostgreSQL: "23514" (check constraint violation).

-- 5. DEFAULT (значение по умолчанию)
-- Если мы не указываем значение, база подставит его сама.

CREATE TABLE IF NOT EXISTS users_default (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW(),    -- текущее время
    is_active BOOLEAN DEFAULT TRUE           -- по умолчанию активен
);

INSERT INTO users_default (name) VALUES ('Alice'); -- все остальное подставится

SELECT * FROM users_default;
-- created_at заполнится текущим временем, is_active будет TRUE.

-- 6. FOREIGN KEY (внешний ключ) — связь между таблицами
-- Говорит: "Это значение должно существовать в другой таблице".

-- У нас есть пользователи
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

-- У нас есть заказы, которые ссылаются на пользователя
CREATE TABLE IF NOT EXISTS orders(
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id),
    amount NUMERIC(10,2) NOT NULL
);

-- Вставляем пользователя
INSERT INTO users (name) VALUES ('Alice');

-- Вставляем заказ с существующим user_id
INSERT INTO orders (amount) VALUES (20349.99);

-- Попытка вставить заказ с несуществующим user_id вызовет ошибку:
-- INSERT INTO orders (user_id, amount) VALUES (999, 50); -- ОШИБКА! (FOREIGN KEY)

-- Что делать при удалении пользователя?
-- 1. ON DELETE CASCADE — удалить заказы автоматически.
-- 2. ON DELETE RESTRICT — запретить удаление, если есть заказы.
-- 3. ON DELETE SET NULL — установить user_id = NULL.

-- Пример с CASCADE:
CREATE TABLE IF NOT EXISTS order_cascade(
    id BIGSERIAL PRIMARY KEY
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE

);

-- Пример с CASCADE:
CREATE TABLE IF NOT EXISTS orders_cascade (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    amount NUMERIC(10,2) NOT NULL
);

-- Пример с RESTRICT:
CREATE TABLE IF NOT EXISTS orders_restrict (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE RESTRICT,
    amount NUMERIC(10,2) NOT NULL
);

-- Пример с CASCADE:
CREATE TABLE IF NOT EXISTS orders_cascade (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    amount NUMERIC(10,2) NOT NULL
);

-- Пример с SET NULL:
CREATE TABLE IF NOT EXISTS orders_set_null (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    amount NUMERIC(10,2) NOT NULL
);

-- Код ошибки в PostgreSQL: "23503" (foreign key violation).