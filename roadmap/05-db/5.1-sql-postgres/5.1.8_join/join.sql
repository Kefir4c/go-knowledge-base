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

2.1. Случай с фильрацией для LEFT JOIN и RIGHT JOIN:
Примеры:
Запрос 1 (условие в ON)
SELECT u.id, u.name, o.created_at
FROM users u
LEFT JOIN orders o ON u.id = o.user_id AND o.created_at >= '2026-01-01';

Результат:
1 | Alice | 2026-01-15
1 | Alice | NULL       (потому что у неё есть и старый заказ, но он не удовлетворяет условию, но строка остаётся)
2 | Bob   | NULL       (у Боба нет заказов после 2026-01-01, но он остаётся)

Запрос 2 (условие в WHERE)
SELECT u.id, u.name, o.created_at
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
WHERE o.created_at >= '2026-01-01';

Результат:
1 | Alice | 2026-01-15

Рекомендация:
Если тебе нужны все пользователи, даже без заказов → ставь условие в ON.
Если тебе нужны только пользователи с заказами после даты → ставь условие в WHERE.

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
SELECT u.name, o.amount, o.created_at
FROM users u
INNER JOIN orders o ON u.id = o.user_id;

-- 1.2. Сокращённый синтаксис (без INNER)
SELECT u.name, o.amount
FROM users u, orders o
WHERE u.id = o.user_id;  -- не рекомендуется, лучше использовать явный JOIN

-- 1.3. INNER JOIN с сортировкой
SELECT u.name, o.amount
FROM users u
INNER JOIN orders o ON u.id = o.user_id
ORDER BY o.amount DESC;

-- 1.4. INNER JOIN с фильтром (WHERE)
SELECT u.name, o.amount
FROM users u
INNER JOIN orders o ON u.id = o.user_id
WHERE o.amount > 100;

-- 1.5. INNER JOIN трёх таблиц: пользователи → заказы → товары в заказ
SELECT
    u.name AS user_name,
    o.id AS order_id,
    p.name AS product_name,
    oi.quantity,
    oi.price
FROM users u
INNER JOIN orders o ON u.id = o.user_id
INNER JOIN order_items oi ON o.id  = oi.order_id
INNER JOIN products p ON  oi.product_id = p.id;

-- 1.6. INNER JOIN с агрегацией (сумма заказов по пользователю)
SELECT u.name, SUM(o.amount) AS total_spent
FROM users u
INNER JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
ORDER BY total_spent DESC;

-- 1.7. INNER JOIN с несколькими условиями (например, заказы за определённый период)
SELECT  u.name, o.amount, o.created_at
FROM users u
INNER JOIN orders o ON u.id = o.user_id
WHERE o.created_at BETWEEN '2026.01.01' AND '2026.12.31';

-- 1.8. INNER JOIN с подзапросом вместо таблицы
SELECT  u.name, stats.total_spent
FROM users u
INNER JOIN(
    SELECT user_id, SUM(amount) AS total_spent
    FROM orders
    GROUP BY user_id
)stats ON u.id = stats.user_id
WHERE stats.total_spent > 100;

-- 1.9. INNER JOIN с оконной функцией (ранг пользователей по сумме заказов)
SELECT u.name, SUM(o.amount) AS total_spent,
       RANK() OVER (ORDER BY SUM(o.amount) DESC ) AS rank
FROM users u
INNER JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
ORDER BY total_spent DESC;

-- 1.10. INNER JOIN с несколькими агрегатами (средний чек, количество заказов)
SELECT u.name,
       COUNT(o.id) AS order_count,
       SUM(o.amount) AS total_spent,
       AVG(o.amount) AS avg_order_amount
FROM users u
INNER JOIN orders o ON u.id = o.user_id
GROUP BY u.id,u.name
ORDER BY  order_count DESC;

-- 2. LEFT JOIN

-- 2.1. LEFT JOIN: все пользователи и их заказы (включая тех, у кого нет заказов)
SELECT u.name, o.amount
FROM users u
LEFT JOIN orders o ON u.id = o.user_id;

-- 2.2. LEFT JOIN с фильтром: найти пользователей без заказов
SELECT u.name
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
WHERE o.id IS NULL;

-- 2.3. LEFT JOIN с сортировкой
SELECT u.name, o.amount
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
ORDER BY o.amount DESC NULLS LAST;

-- 2.4. LEFT JOIN с COUNT (количество заказов, включая 0 для пользователей без заказов)
SELECT u.name, COUNT(o.id) AS order_count
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
ORDER BY order_count DESC;

-- 2.5. LEFT JOIN трёх таблиц (пользователи → заказы → товары, с сохранением пользователей без заказов)
SELECT u.name, o.id AS order_id, p.name AS products_name
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
LEFT JOIN order_items oi ON o.id = oi.order_id
LEFT JOIN products p ON oi.product_id = p.id;

-- 2.6. LEFT JOIN с агрегацией: сумма заказов по пользователю (включая 0 для без заказов)
SELECT u.name, COALESCE(SUM(o.amount), 0) AS total_spent
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
ORDER BY total_spent DESC;

-- 2.7. LEFT JOIN с условием на правую таблицу (ОСТОРОЖНО! условие в WHERE может превратить LEFT в INNER)
-- Если мы хотим увидеть только пользователей с заказами больше 100, но сохранить всех пользователей?
-- Нужно условие поместить в ON, а не в WHERE.

-- Неправильно (потеряем пользователей без заказов):
SELECT u.name, o.amount
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
WHERE o.amount > 100;

--2.8. LEFT JOIN с подзапросом: средняя сумма заказов для каждого пользователя (включая NULL)
SELECT u.name, stats.avg_amount
FROM users u
LEFT JOIN (
    SELECT user_id, AVG(amount) AS avg_amount
    FROM orders
    GROUP BY user_id
)stats ON u.id = stats.user_id;

-- 2.9. LEFT JOIN с оконной функцией и COALESCE для замены NULL
SELECT u.name,
       COALESCE(SUM(o.amount), 0) AS total_spent,
       RANK() OVER (ORDER BY SUM(o.amount) DESC )
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
ORDER BY total_spent DESC;

-- 2.10. LEFT JOIN с несколькими условиями и фильтрацией на обе таблицы
SELECT u.name, o.amount, o.created_at
FROM users u
LEFT JOIN orders o ON u.id = o.user_id AND o.created_at >= '2026-01-01'
ORDER BY u.name;

-- 3. RIGHT JOIN
-- RIGHT JOIN редко используется, так как его можно заменить LEFT JOIN, поменяв таблицы местами.
-- Но для полноты картины покажем.

-- 3.1. RIGHT JOIN: все заказы и их пользователи (эквивалентно LEFT JOIN orders TO users)
SELECT o.id, u.name
FROM users u
         RIGHT JOIN orders o ON u.id = o.user_id;

-- 3.2. RIGHT JOIN с фильтром: заказы без пользователей (таких быть не должно, но для примера)
SELECT o.id, u.name
FROM users u
         RIGHT JOIN orders o ON u.id = o.user_id
WHERE u.id IS NULL;

-- 3.3. RIGHT JOIN с агрегацией (все заказы, даже если пользователь удалён — но FOREIGN KEY запрещает)
-- Так как у нас ON DELETE CASCADE, таких не будет.

-- 4. FULL JOIN
-- FULL JOIN возвращает все строки из обеих таблиц.

-- 4.1. FULL JOIN: все пользователи и все заказы (объединение)
SELECT u.name, o.id AS order_id
FROM users u
FULL JOIN orders o ON u.id = o.user_id;

-- 4.2. FULL JOIN с фильтром: пользователи без заказов или заказы без пользователей
SELECT u.name, o.id
FROM users u
FULL JOIN orders o ON u.id = o.user_id
WHERE u.name IS NULL AND o.user_id IS NULL;

-- 4.3. FULL JOIN с COUNT (сколько пользователей и заказов)
SELECT COUNT(DISTINCT u.id) AS user_count, COUNT(o.id) AS order_count
FROM users u
FULL JOIN orders o ON u.id = o.user_id;

-- 5. SELF JOIN

-- Создаём таблицу сотрудников (иерархия)
CREATE TABLE employees (
id BIGSERIAL PRIMARY KEY,
name TEXT NOT NULL,
position TEXT NOT NULL,
salary NUMERIC(10,2) NOT NULL,
manager_id BIGINT REFERENCES employees(id) ON DELETE SET NULL,
department TEXT
);

INSERT INTO employees (name, position, salary, manager_id, department) VALUES
('Алексей Иванов', 'CEO', 20000.00, NULL, 'Management'),
('Мария Смирнова', 'CTO', 18000.00, 1, 'IT'),
('Олег Петров', 'CFO', 18000.00, 1, 'Finance'),
('Анна Сидорова', 'Dev Lead', 12000.00, 2, 'IT'),
('Дмитрий Козлов', 'Senior Developer', 10000.00, 4, 'IT'),
('Елена Морозова', 'Senior Developer', 10000.00, 4, 'IT'),
('Иван Новиков', 'Junior Developer', 6000.00, 5, 'IT'),
('Ольга Васильева', 'Junior Developer', 6000.00, 5, 'IT'),
('Сергей Павлов', 'Financial Analyst', 9000.00, 3, 'Finance'),
('Наталья Романова', 'Accountant', 7000.00, 3, 'Finance');

-- Создаём таблицу друзей (связи многие-ко-многим через себя)
CREATE TABLE friends (
user_id BIGINT NOT NULL,
friend_id BIGINT NOT NULL,
PRIMARY KEY (user_id, friend_id)
);

INSERT INTO friends (user_id, friend_id) VALUES
(1, 2),
(1, 3),
(2, 4),
(3, 4),
(4, 5);

-- Создаём таблицу продуктов для сравнения
CREATE TABLE products_compare (
id BIGSERIAL PRIMARY KEY,
name TEXT NOT NULL,
price NUMERIC(10,2) NOT NULL,
category TEXT
);

INSERT INTO products_compare (name, price, category) VALUES
('Laptop Pro', 1500.00, 'Electronics'),
('Laptop Lite', 800.00, 'Electronics'),
('Smartphone X', 900.00, 'Electronics'),
('Smartphone Y', 600.00, 'Electronics'),
('Tablet', 350.00, 'Electronics'),
('Mouse', 25.99, 'Accessories'),
('Keyboard', 45.00, 'Accessories'),
('Monitor', 320.00, 'Electronics'),
('Headphones', 120.00, 'Audio'),
('Speaker', 80.00, 'Audio');


-- 1. ИЕРАРХИЯ СОТРУДНИКОВ
-- 1.1. Вывести сотрудников и их непосредственных руководителей
SELECT e1.name AS employee, e2.name AS manager
FROM employees e1
LEFT JOIN employees e2 ON e1.manager_id = e2.id
ORDER BY e1.name;

-- 1.2.Подсчитать количество подчинённых у каждого руководителя
SELECT e2.name AS manager, COUNT(e1.id) AS subordinates_count
FROM employees e1
RIGHT JOIN employees e2 ON e1.manager_id = e2.id
GROUP BY e2.id, e2.name
ORDER BY subordinates_count DESC;

-- 1.3. Найти всех подчинённых (всех уровней) для конкретного менеджера (рекурсивный CTE)
WITH RECURSIVE sub_tree AS (
    -- Базовый случай: непосредственные подчинённые CEO
    SELECT id, name, manager_id, 1 AS level
    FROM employees
    WHERE manager_id = 1
    UNION ALL
    -- Рекурсивный случай: подчинённые подчинённых
    SELECT e.id, e.name, e.manager_id, st.level + 1
    FROM employees e
    INNER JOIN sub_tree st ON e.manager_id = st.id
)
SELECT name, level
FROM sub_tree
ORDER BY level, name;

-- 1.4.Найти цепочку подчинения для конкретного сотрудника
WITH RECURSIVE chain AS (
    -- Базовый случай: сам сотрудник
    SELECT id, name, manager_id, 0 AS level
    FROM employees
    WHERE name = 'Иван Новиков'
    UNION ALL
    -- Рекурсивный случай: поднимаемся вверх по иерархии
    SELECT e.id, e.name, e.manager_id, c.level + 1
    FROM employees e
    INNER JOIN chain c ON e.id = c.manager_id
)
SELECT name, level
FROM chain
ORDER BY level;

-- 1.5.Найти всех друзей конкретного пользователя
SELECT u2.name AS friend
FROM friends f
JOIN employees u1 ON f.user_id = u1.id
JOIN employees u2 ON f.friend_id = u2.id
WHERE u1.name = 'Алексей Иванов';

-- 1.6.Найти пользователей, у которых есть друзья, и количество друзей
SELECT u1.name AS user, COUNT(f.friend_id) AS friend_count
FROM friends f
JOIN employees u1 ON f.user_id = u1.id
GROUP BY u1.id, u1.name
ORDER BY friend_count DESC;

-- 6. КОМБИНАЦИИ JOIN С ПОДЗАПРОСАМИ И CTE
-- 6.1. JOIN с CTE: топ-3 пользователей по сумме заказов
WITH user_spent AS (
    SELECT user_id, SUM(amount) AS total
    FROM orders
    GROUP BY user_id
)
SELECT u.name, us.total
FROM users u
JOIN user_spent us ON u.id = us.user_id
ORDER BY us.total DESC
LIMIT 3 OFFSET 0;

-- 6.2. JOIN с подзапросом в SELECT (скалярный подзапрос)
SELECT u.name,
       (SELECT COUNT(*) FROM orders WHERE user_id = u.id) AS order_count
FROM users u;

-- 6.3. JOIN с подзапросом в FROM
SELECT u.name, stats.total_spent
FROM users u
JOIN (
    SELECT user_id, SUM(amount) AS total_spent
    FROM orders
    GROUP BY user_id
) stats ON u.id = stats.user_id;

-- 6.4. JOIN с EXISTS (найти пользователей, у которых есть заказы)
SELECT u.name
FROM users u
WHERE EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id);

-- 6.5. JOIN с NOT EXISTS (найти пользователей без заказов)
SELECT u.name
FROM users u
WHERE NOT EXISTS (SELECT 1 FROM orders o WHERE o.user_id = u.id);

-- 7. ПРАКТИЧЕСКИЕ ЗАДАНИЯ

-- ЗАДАНИЕ 1 (Junior): Вывести имена пользователей и сумму их последнего заказа (используйте JOIN и оконную функцию).

-- ЗАДАНИЕ 2 (Junior): Найти все продукты, которые были заказаны хотя бы один раз (JOIN order_items и products).

-- ЗАДАНИЕ 3 (Middle): Вывести список пользователей, общую сумму их заказов и количество заказов. Включить пользователей без заказов.

-- ЗАДАНИЕ 4 (Middle): Найти пользователей, которые оставляли отзывы на продукты, которые они заказывали (JOIN orders, order_items, reviews).

-- ЗАДАНИЕ 5 (Senior): Для каждого продукта вывести его название, общее количество заказов и средний рейтинг из отзывов.

-- РЕШЕНИЯ ЗАДАНИЙ

-- ЗАДАНИЕ 1
WITH last_order AS (
    SELECT DISTINCT ON (user_id) user_id, amount, created_at
    FROM orders
    ORDER BY user_id, created_at DESC
)
SELECT u.name, lo.amount AS last_order_amount, lo.created_at
FROM users u
LEFT JOIN last_order lo ON u.id = lo.user_id;

-- ЗАДАНИЕ 2
SELECT DISTINCT p.id, p.name, p.price
FROM products p
JOIN order_items oi ON p.id = oi.product_id;

-- ЗАДАНИЕ 3
SELECT u.name,
       COALESCE(SUM(o.amount)) AS total_spent,
       COUNT(o.amount) AS order_count
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
ORDER BY total_spent DESC;

-- ЗАДАНИЕ 4
SELECT DISTINCT u.name
FROM users u
JOIN orders o on u.id = o.user_id
JOIN order_items oi on o.id = oi.order_id
JOIN reviews r ON u.id = r.user_id AND oi.product_id = r.product_id;

-- ЗАДАНИЕ 5
-- ЗАДАНИЕ 5
SELECT p.id, p.name,
       COUNT(DISTINCT oi.order_id) AS order_count,
       COALESCE(AVG(r.rating), 0) AS avg_rating
FROM products p
LEFT JOIN order_items oi ON p.id = oi.product_id
LEFT JOIN orders o ON oi.order_id = o.id
LEFT JOIN reviews r ON p.id = r.product_id
GROUP BY p.id, p.name
ORDER BY avg_rating DESC, order_count DESC;