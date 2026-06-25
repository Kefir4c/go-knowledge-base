/*
=============================================================================
ШАГ 1.8: ОБЪЕДИНЕНИЕ ТАБЛИЦ (JOIN)
=============================================================================

ТЕОРИЯ
-------

1. ЗАЧЕМ НУЖНЫ JOIN?
   - Данные часто находятся в разных таблицах (нормализация).
   - JOIN позволяет объединить данные из нескольких таблиц в одном запросе.
   - Без JOIN пришлось бы делать несколько запросов и объединять результаты в коде, что медленнее и сложнее.

2. ВИДЫ JOIN:
   - INNER JOIN — возвращает только строки, где есть совпадения в обеих таблицах.
   - LEFT JOIN (или LEFT OUTER JOIN) — возвращает все строки из левой таблицы и совпадающие из правой. Если совпадений нет — поля правой таблицы заполняются NULL.
   - RIGHT JOIN — аналогично LEFT, но все строки из правой таблицы.
   - FULL JOIN (или FULL OUTER JOIN) — возвращает все строки из обеих таблиц, с NULL для отсутствующих совпадений.
   - CROSS JOIN — декартово произведение (каждая строка первой таблицы с каждой строкой второй). Почти не используется.
   - SELF JOIN — соединение таблицы с самой собой (например, для иерархий).

3. СИНТАКСИС:
   SELECT columns
   FROM table1
   [INNER | LEFT | RIGHT | FULL] JOIN table2 ON condition
   [WHERE ...]
   [GROUP BY ...]
   [ORDER BY ...];

   Условие соединения (ON) обычно сравнивает внешний ключ одной таблицы с первичным ключом другой.

4. ОСОБЕННОСТИ:
   - При использовании LEFT JOIN, если в правой таблице несколько строк, соответствующих одной строке левой, левая строка будет повторяться.
   - JOIN можно комбинировать: JOIN нескольких таблиц последовательно.
   - Можно использовать JOIN с подзапросами и CTE.

5. ПРОИЗВОДИТЕЛЬНОСТЬ:
   - INNER JOIN обычно быстрее, чем LEFT JOIN, так как меньше строк.
   - Для ускорения JOIN важно иметь индексы на внешние ключи.

6. СВЯЗЬ С GO:
   - Результат JOIN часто маппится в структуру, содержащую поля из разных таблиц.
   - Например, структура OrderWithUser содержит поля заказа и пользователя.
   - Для обработки повторяющихся строк (например, заказов с товарами) может использоваться сканирование строк и сборка вложенных структур.

*/

-- 0. ПОДГОТОВКА 

-- Создаём таблицы
CREATE TABLE IF NOT EXISTS users (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    amount NUMERIC(10,2) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS products (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    price NUMERIC(10,2) NOT NULL
);

CREATE TABLE IF NOT EXISTS order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    quantity INT NOT NULL CHECK (quantity > 0),
    price NUMERIC(10,2) NOT NULL
);

CREATE TABLE IF NOT EXISTS reviews (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    rating INT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    text TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Добавляем тестовые данные (если таблицы пустые)
INSERT INTO users (name, email) VALUES
    ('Alice', 'alice@mail.com'),
    ('Bob', 'bob@mail.com'),
    ('Charlie', 'charlie@mail.com'),
    ('Diana', 'diana@mail.com')
ON CONFLICT (email) DO NOTHING;

INSERT INTO products (name, price) VALUES
    ('Laptop', 1200.00),
    ('Mouse', 25.99),
    ('Keyboard', 45.00),
    ('Monitor', 320.00)
ON CONFLICT DO NOTHING;

INSERT INTO orders (user_id, amount) VALUES
    (1, 100.50),
    (1, 200.00),
    (2, 50.25),
    (3, 300.00)
ON CONFLICT DO NOTHING;

INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
    (1, 1, 1, 1200.00),
    (1, 2, 2, 25.99),
    (2, 3, 1, 45.00),
    (3, 4, 1, 320.00);

INSERT INTO reviews (user_id, product_id, rating, text) VALUES
    (1, 1, 5, 'Great laptop!'),
    (1, 2, 4, 'Good mouse'),
    (2, 1, 4, 'Nice laptop'),
    (3, 3, 5, 'Excellent keyboard'),
    (4, 4, 3, 'Average monitor');

-- 1. INNER JOIN 

-- 1.1. Простой INNER JOIN: пользователи и их заказы
SELECT u.name, o.amount, o.create_at
FROM users UNBOUNDED
INNER JOIN orders o ON u.id = o.user_id;

-- 1.2. Сокращённый синтаксис (без INNER)
SELECT u.name, o.amount
FROM users u, orders o
WHERE u.id = o.user_id;  -- не рекомендуется, лучше использовать явный JOIN

-- 1.3. INNER JOIN с сортировкой
SELECT u.name, o.amount
FROM users u
INNER JOIN orders o ON u.id = o.user_id
ORDER BY o.amount DESC

-- 1.4. INNER JOIN с фильтром (WHERE)
SELECT u.name, o.amount
FROM users u
INNER JOIN orders o ON u.id = o.user_id
WHERE o.amount > 100;

-- 1.5. INNER JOIN трёх таблиц: пользователи → заказы → товары в заказ