/*
ШАГ 2.4: ПРОДВИНУТЫЕ ЗАПРОСЫ (CTE, ОКНА)
ТЕОРИЯ

1. ЧТО ТАКОЕ CTE (Common Table Expression)?
   - CTE — это временная таблица, которая существует только в рамках одного запроса.
   - Синтаксис: WITH имя AS (SELECT ...) SELECT ... FROM имя.
   - Позволяет разбить сложный запрос на простые шаги, улучшая читаемость.
   - В PostgreSQL CTE может быть материализована (вычислена один раз) или встроена.

   Пример:
   WITH user_orders AS (
       SELECT user_id, COUNT(*) AS order_count
       FROM orders
       GROUP BY user_id
   )
   SELECT u.name, uo.order_count
   FROM users u
   LEFT JOIN user_orders uo ON u.id = uo.user_id;

   - CTE может быть рекурсивной (для иерархий), но это тема отдельная.

1.1. CTE — это не просто подзапрос, это «барьер» для оптимизатора
Вважный пункт: «CTE может быть материализована или встроена». Это ключевой нюанс, о котором забывают многие разработчики.

До PostgreSQL 12: CTE всегда материализовалась (вычислялась один раз и сохранялась во временной таблице). Это было безопасно, 
но иногда медленно, потому что оптимизатор не мог переставить операции.

С PostgreSQL 12: Если CTE используется только один раз, она не материализуется, а встраивается в основной запрос. Это позволяет 
оптимизатору переставлять операции внутри CTE, улучшая план.

Пример (где это важно):

WITH active_users AS (
    SELECT * FROM users WHERE is_active = true
)
SELECT * FROM active_users WHERE name = 'Alice';
До PG 12: Сначала создалась таблица всех активных пользователей (может быть 1 млн строк), потом из неё выбрали Алису.

С PG 12: Оптимизатор может сначала применить WHERE name = 'Alice', а потом уже is_active = true (если это выгоднее).

Как управлять материализацией (PG 12+):
-- Принудительная материализация (если CTE используется несколько раз)
WITH active_users AS MATERIALIZED (
    SELECT * FROM users WHERE is_active = true
)
SELECT * FROM active_users WHERE name = 'Alice';

-- Запрет материализации (если хотим, чтобы CTE встроилась)
WITH active_users AS NOT MATERIALIZED (
    SELECT * FROM users WHERE is_active = true
)
SELECT * FROM active_users WHERE name = 'Alice';   

1.2. Рекурсивные CTE — мощь, но с подвохом
Рекурсивные CTE (WITH RECURSIVE) — это тема отдельная, но важно знать одну опасность.
Опасность: Если забыть LIMIT или условие выхода, рекурсивный запрос может упасть в бесконечный цикл и нагрузить сервер до смерти.

Пример безопасной рекурсии:

WITH RECURSIVE subordinates AS (
    -- Базовый случай: сам сотрудник
    SELECT id, name, manager_id, 1 AS level
    FROM employees
    WHERE id = 1
    UNION ALL
    -- Рекурсивный случай: подчинённые
    SELECT e.id, e.name, e.manager_id, s.level + 1
    FROM employees e
    JOIN subordinates s ON e.manager_id = s.id
    WHERE s.level < 10  -- !!! Защита от бесконечного цикла !!!
)
SELECT * FROM subordinates;

2. ЧТО ТАКОЕ ОКОННЫЕ ФУНКЦИИ?
   - Оконные функции вычисляют значение для каждой строки на основе "окна" — набора строк, связанных с текущей строкой.
   - В отличие от GROUP BY, оконные функции НЕ схлопывают строки — каждая строка остаётся в результате.
   - Синтаксис: функция() OVER (PARTITION BY ... ORDER BY ...)
   - PARTITION BY — делит строки на группы (как GROUP BY, но без схлопывания).
   - ORDER BY — задаёт порядок строк внутри каждой группы (нужен для функций ранжирования и смещения).

2.1. Оконные функции — порядок выполнения и PARTITION BY
Выше написано: «Оконные функции вычисляются ПОСЛЕ WHERE, GROUP BY, HAVING, но ДО ORDER BY и LIMIT». Это критически важно для производительности.

Пример:
SELECT * FROM (
    SELECT
        *,
        ROW_NUMBER() OVER (PARTITION BY category ORDER BY price DESC) AS rn
    FROM products
    WHERE is_active = true
) t
WHERE rn = 1;
Здесь ROW_NUMBER() вычисляется после WHERE is_active = true. Если бы мы оставили WHERE снаружи, он бы применился после вычисления оконной функции, и это было бы медленнее.

Правило: Всегда фильтруй WHERE как можно раньше, до оконных функций.

2.2. Оконные функции с ROWS и RANGE — в чём разница?
ROWS — ограничивает окно строго по количеству строк.
RANGE — ограничивает окно по значению (например, все строки, где цена отличается не более чем на 10%).

Пример:

-- Скользящее среднее по 3 предыдущим строкам (ROWS)
SELECT
    date,
    amount,
    AVG(amount) OVER (ORDER BY date ROWS BETWEEN 2 PRECEDING AND CURRENT ROW) AS avg_3
FROM sales;

-- Все строки, где дата отличается не более чем на 7 дней (RANGE)
SELECT
    date,
    amount,
    AVG(amount) OVER (ORDER BY date RANGE BETWEEN '7 days' PRECEDING AND CURRENT ROW) AS avg_7d
FROM sales;
Фишка: RANGE с датами — мощная штука, которую мало кто знает, но она очень полезна для финансовых отчётов.   

3. ЗАЧЕМ НУЖНЫ ОКОННЫЕ ФУНКЦИИ?
   - Ранжирование: присвоить номер или ранг каждой строке в группе (ROW_NUMBER, RANK, DENSE_RANK).
   - Доступ к соседним строкам: показать предыдущее или следующее значение (LAG, LEAD).
   - Накопительные итоги: кумулятивная сумма, скользящее среднее (SUM() OVER (ORDER BY ...)).
   - Сравнение со средним по группе: показать зарплату сотрудника и среднюю зарплату по отделу в той же строке.

3.1. NTILE — распределение по корзинам
NTILE(n) — разбивает строки на n примерно равных групп (корзин). Используется для квартилей, децилей и т.д.

Пример:
-- Разбиваем сотрудников на 4 квартили по зарплате
SELECT
    name,
    salary,
    NTILE(4) OVER (ORDER BY salary DESC) AS quartile
FROM employees;
Фишка: Это часто спрашивают на собеседованиях, потому что задача «найти топ-10% сотрудников» решается через NTILE(10).

3.2. FIRST_VALUE и LAST_VALUE с подвохом
FIRST_VALUE — возвращает первое значение в окне.
LAST_VALUE — возвращает последнее значение в окне.

Подвох: По умолчанию LAST_VALUE считает последним текущую строку, потому что окно по умолчанию ROWS BETWEEN UNBOUNDED PRECEDING AND CURRENT ROW. 
Чтобы получить реальное последнее значение в группе, нужно явно указать ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING.

Пример:
-- Показывает разницу между зарплатой сотрудника и максимальной в отделе
SELECT
    name,
    salary,
    department,
    FIRST_VALUE(salary) OVER (PARTITION BY department ORDER BY salary DESC) AS max_salary,
    LAST_VALUE(salary) OVER (PARTITION BY department ORDER BY salary DESC
        ROWS BETWEEN UNBOUNDED PRECEDING AND UNBOUNDED FOLLOWING) AS min_salary
FROM employees;   

4. КЛЮЧЕВЫЕ ОКОННЫЕ ФУНКЦИИ (ПОСТГРЕС):
   - ROW_NUMBER() — порядковый номер строки в рамках PARTITION (уникальный, даже при равенстве).
   - RANK() — ранг с пропусками (1, 2, 2, 4).
   - DENSE_RANK() — ранг без пропусков (1, 2, 2, 3).
   - NTILE(n) — распределяет строки по n корзинам (например, квартили).
   - LAG(column, offset, default) — значение из предыдущей строки.
   - LEAD(column, offset, default) — значение из следующей строки.
   - FIRST_VALUE(column) — первое значение в окне.
   - LAST_VALUE(column) — последнее значение в окне.
   - SUM(column) OVER() — кумулятивная сумма.
   - AVG(column) OVER() — скользящее среднее.

5. ПОРЯДОК ВЫПОЛНЕНИЯ (связь с оконными функциями):
   - Оконные функции вычисляются ПОСЛЕ WHERE, GROUP BY, HAVING,
     но ДО ORDER BY и LIMIT.
   - Это значит, что в оконной функции ты можешь использовать агрегаты, полученные в GROUP BY,
     но не можешь использовать алиасы, созданные в SELECT (потому что SELECT ещё не выполнен).

6. СРАВНЕНИЕ GROUP BY И ОКОННЫХ ФУНКЦИЙ:
   - GROUP BY схлопывает строки в одну на группу.
   - Оконная функция оставляет все строки, но добавляет новую колонку с вычисленным значением.

   Пример: у нас есть заказы пользователей.
   - GROUP BY user_id даст одну строку на пользователя с суммой его заказов.
   - SUM(amount) OVER (PARTITION BY user_id) добавит к каждой строке сумму заказов этого пользователя — все строки останутся.

7. ТИПИЧНЫЕ ЗАДАЧИ, РЕШАЕМЫЕ ОКОННЫМИ ФУНКЦИЯМИ:
   - Топ-3 товара в каждой категории по продажам.
   - Для каждого заказа показать сумму предыдущего заказа того же пользователя.
   - Кумулятивная выручка по месяцам.
   - Сравнение зарплаты сотрудника со средней по отделу.
   - Нумерация строк для пагинации (ROW_NUMBER).

8. Фишки 
Козырь №1: «CTE — это не всегда быстрее подзапроса»
Суть: В PostgreSQL CTE может материализоваться, а подзапрос — нет. Иногда подзапрос работает быстрее, 
потому что он встраивается в план и позволяет оптимизатору переставлять операции.

Пример:
-- CTE (может быть медленнее)
WITH active_users AS (
    SELECT * FROM users WHERE is_active = true
)
SELECT * FROM active_users WHERE name = 'Alice';

-- Подзапрос (может быть быстрее)
SELECT * FROM (
    SELECT * FROM users WHERE is_active = true
) active_users
WHERE name = 'Alice';
Что сказать на собеседовании:
«В PostgreSQL CTE может материализоваться, что иногда хуже, чем простой подзапрос, потому что оптимизатор не может переставить операции. 
Поэтому я всегда проверяю EXPLAIN и, если нужно, использую NOT MATERIALIZED для CTE, чтобы заставить её встроиться.

2: «Оконные функции и DISTINCT — порядок имеет значение»
Суть: DISTINCT применяется после оконных функций. Поэтому если ты используешь DISTINCT и оконную функцию, она выполнится до удаления дубликатов.

Пример:
-- Сначала ROW_NUMBER(), потом DISTINCT
SELECT DISTINCT
    category,
    ROW_NUMBER() OVER (PARTITION BY category ORDER BY price DESC) AS rn
FROM products;
Что сказать на собеседовании:
«Важно помнить, что оконные функции вычисляются до DISTINCT. Поэтому если я хочу получить уникальные значения, 
я сначала применяю оконную функцию, а потом фильтрую или группирую.

3: «Оконные функции в WHERE — нельзя, но можно через подзапрос»
Суть: Оконные функции нельзя использовать напрямую в WHERE, потому что они вычисляются после WHERE.

Как обойти:
-- Ошибка: нельзя использовать оконную функцию в WHERE
SELECT * FROM products
WHERE ROW_NUMBER() OVER (PARTITION BY category ORDER BY price DESC) = 1;

-- Правильно: через подзапрос или CTE
WITH ranked AS (
    SELECT *,
           ROW_NUMBER() OVER (PARTITION BY category ORDER BY price DESC) AS rn
    FROM products
)
SELECT * FROM ranked WHERE rn = 1;

4: «Оконные функции с агрегатами — мощь комбинации»
Суть: Ты можешь использовать агрегаты внутри оконных функций.

Пример:
-- Показать зарплату, среднюю по отделу и разницу
SELECT
    name,
    salary,
    department,
    AVG(salary) OVER (PARTITION BY department) AS avg_dept,
    salary - AVG(salary) OVER (PARTITION BY department) AS diff_from_avg
FROM employees;
Фишка: Это заменяет отдельный GROUP BY и подзапрос, делая код чище и быстрее.

Козырь №5: «Оконные функции и пагинация без OFFSET»
Суть: Оконные функции позволяют делать пагинацию без OFFSET, что особенно полезно для больших таблиц.

Пример:
-- Пагинация с ROW_NUMBER
WITH numbered AS (
    SELECT
        *,
        ROW_NUMBER() OVER (ORDER BY id) AS rn
    FROM users
)
SELECT * FROM numbered
WHERE rn BETWEEN 11 AND 20;
Фишка: На собеседовании это покажет, что ты знаешь альтернативу OFFSET для пагинации на больших объёмах данных.   
*/

-- 0. СОЗДАНИЕ ТАБЛИЦ (ЛОГИСТИКА И СКЛАДЫ)
-- Поставщики
CREATE TABLE suppliers (
id BIGSERIAL PRIMARY KEY,
name TEXT NOT NULL,
email TEXT UNIQUE NOT NULL,
phone TEXT,
is_active BOOLEAN DEFAULT TRUE
);

-- Склады
CREATE TABLE warehouses (
id BIGSERIAL PRIMARY KEY,
name TEXT NOT NULL,
location TEXT NOT NULL,
capacity INT NOT NULL -- максимальное количество паллет
);

-- Товары
CREATE TABLE products (
id BIGSERIAL PRIMARY KEY,
name TEXT NOT NULL,
sku TEXT UNIQUE NOT NULL, -- артикул
price NUMERIC(10,2) NOT NULL,
weight_kg NUMERIC(8,2) NOT NULL,
category TEXT,
is_active BOOLEAN DEFAULT TRUE
);

-- Поставки (связь товаров и складов, с количеством и датой)
CREATE TABLE shipments (
id BIGSERIAL PRIMARY KEY,
product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
warehouse_id BIGINT NOT NULL REFERENCES warehouses(id) ON DELETE CASCADE,
supplier_id BIGINT NOT NULL REFERENCES suppliers(id) ON DELETE CASCADE,
quantity INT NOT NULL CHECK (quantity > 0),
shipment_date DATE NOT NULL,
delivery_date DATE,
status TEXT DEFAULT 'pending' -- pending, in_transit, delivered, cancelled
);

-- 1. НАПОЛНЕНИЕ ТЕСТОВЫМИ ДАННЫМИ (10 000+ СТРОК ДЛЯ ДЕМОНСТРАЦИИ)
INSERT INTO suppliers (name, email, phone, is_active) VALUES
('ООО "Рога и Копыта"', 'roga@mail.ru', '+7-999-111-22-33', TRUE),
('ЗАО "ТехноПром"', 'tehno@mail.ru', '+7-999-222-33-44', TRUE),
('ИП Иванов', 'ivanov@mail.ru', '+7-999-333-44-55', FALSE),
('ООО "Склад-Сервис"', 'sklad@mail.ru', '+7-999-444-55-66', TRUE),
('АО "Глобал Логистик"', 'global@mail.ru', '+7-999-555-66-77', TRUE);

INSERT INTO warehouses (name, location, capacity) VALUES
('Склад Северный', 'Москва, ул. Северная, 1', 1000),
('Склад Южный', 'Москва, ул. Южная, 2', 800),
('Склад Восточный', 'Москва, ул. Восточная, 3', 1200),
('Склад Западный', 'Москва, ул. Западная, 4', 900);

INSERT INTO products (name, sku, price, weight_kg, category, is_active) VALUES
('Смартфон X', 'PH-001', 800.00, 0.2, 'Electronics', TRUE),
('Ноутбук Pro', 'NB-002', 1500.00, 2.5, 'Electronics', TRUE),
('Наушники HD', 'HP-003', 120.00, 0.3, 'Audio', TRUE),
('Монитор 27"', 'MN-004', 450.00, 5.0, 'Electronics', TRUE),
('Клавиатура MX', 'KB-005', 90.00, 0.8, 'Accessories', TRUE),
('Мышь Wireless', 'MS-006', 45.00, 0.1, 'Accessories', TRUE),
('Принтер Лазерный', 'PR-007', 350.00, 8.0, 'Office', FALSE),
('Сканер A4', 'SC-008', 200.00, 3.0, 'Office', TRUE),
('Внешний SSD 1TB', 'SS-009', 120.00, 0.05, 'Electronics', TRUE),
('Роутер Wi-Fi 6', 'RT-010', 150.00, 0.5, 'Networking', TRUE);

-- Генерируем много поставок (100 000 строк) для демонстрации производительности
INSERT INTO shipments (product_id, warehouse_id, supplier_id, quantity, shipment_date, delivery_date, status)
SELECT
    (random() * 9 + 1)::int AS product_id,   -- от 1 до 10
    (random() * 3 + 1)::int AS warehouse_id, -- от 1 до 4
    (random() * 4 + 1)::int AS supplier_id,  -- от 1 до 5
    (random() * 100 + 1)::int AS quantity,
    NOW() - (random() * 365 || ' days')::interval AS shipment_date,
    CASE
        WHEN random() > 0.3 THEN NOW() - (random() * 30 || ' days')::interval
        ELSE NULL
        END AS delivery_date,
    CASE
        WHEN random() < 0.1 THEN 'cancelled'
        WHEN random() < 0.3 THEN 'pending'
        WHEN random() < 0.6 THEN 'in_transit'
        ELSE 'delivered'
        END AS status
FROM generate_series(1, 500000);

-- 1. БАЗОВЫЙ CTE (простой)
-- 1.1. Посчитаем количество поставок от каждого поставщика.
WITH supplier_stats AS (
    SELECT supplier_id, COUNT(*) AS shipment_count
    FROM shipments
    GROUP BY supplier_id
)
SELECT s.name, ss.shipment_count
FROM suppliers s
LEFT JOIN supplier_stats ss ON s.id = ss.supplier_id
ORDER BY supplier_id DESC;

-- 2. CTE С НЕСКОЛЬКИМИ ШАГАМИ (цепочка)
-- 2.1. Найти поставщиков, у которых общий вес поставок превышает 1000 кг.
WITH product_weight AS(
    SELECT id, products.weight_kg
    FROM products
),
    shipment_weights AS(
    SELECT
        s.supplier_id,
        SUM(s.quantity * pw.weight_kg) AS total_weight
    FROM shipments s
    JOIN product_weight pw ON s.product_id = pw.id
    GROUP BY s.supplier_id
    )
SELECT
    sup.name,
    sw.total_weight
FROM suppliers sup
JOIN shipment_weights sw ON sup.id = sw.supplier_id
WHERE total_weight > 1000
ORDER BY total_weight DESC;

-- 3. ОКОННЫЕ ФУНКЦИИ: ROW_NUMBER, RANK, DENSE_RANK
-- 3.1. Пронумеровать все поставки по дате (глобально).
SELECT
    id,
    shipment_date,
    product_id,
    quantity,
    ROW_NUMBER() over (ORDER BY shipment_date DESC ) AS row_num
FROM shipments
ORDER BY shipment_date DESC
LIMIT 20;

-- 3.2. Пронумеровать поставки внутри каждого поставщика (по дате).
SELECT
    id,
    supplier_id,
    shipment_date,
    ROW_NUMBER() OVER (PARTITION BY supplier_id ORDER BY shipment_date) AS shipment_num
FROM shipments
ORDER BY supplier_id,shipment_date;

-- 3.3. Ранжирование поставщиков по общему количеству поставок.
WITH supplier_total AS (
    SELECT
        supplier_id, COUNT(*) AS total
    FROM shipments
    GROUP BY  supplier_id
)
SELECT
    s.name,
    st.total,
    RANK() OVER (ORDER BY st.total DESC ) AS rank,
    DENSE_RANK() OVER (ORDER BY st.total DESC ) AS dense_rank
FROM suppliers s
JOIN supplier_total st ON s.id = st.supplier_id
ORDER BY rank;

-- 3.4. Топ-3 продукта по количеству поставок в каждой категории.
WITH product_rank AS(
    SELECT
        p.category,
        p.name,
        COUNT(s.id) AS shipment_count,
        ROW_NUMBER() OVER (PARTITION BY p.category ORDER BY COUNT(s.id) DESC) AS rn
    FROM products p
    JOIN shipments s ON p.id = s.product_id
    GROUP BY p.category,p.id
)
SELECT category, name, shipment_count
FROM product_rank
WHERE rn <= 3
ORDER BY category, rn;

-- 4. ОКОННЫЕ ФУНКЦИИ: LAG, LEAD (доступ к соседним строкам)
-- 4.1. Для каждой поставки показать дату предыдущей поставки того же поставщика.
SELECT
    id,
    supplier_id,
    shipment_date,
    LAG(shipment_date) OVER (PARTITION BY supplier_id ORDER BY shipment_date) AS prev_shipment_date
FROM shipments
ORDER BY  supplier_id, prev_shipment_date;

-- 4.2. Для каждой поставки показать разницу в днях с предыдущей поставкой.
SELECT
    id,
    supplier_id,
    shipment_date,
    LAG(shipment_date) OVER (PARTITION BY supplier_id ORDER BY shipment_date) AS prev_date,
    shipment_date - LAG(shipment_date) OVER (PARTITION BY supplier_id ORDER BY shipment_date) AS days_diff
FROM shipments
WHERE supplier_id IN (1,2,3)
ORDER BY supplier_id, shipment_date;

-- 4.3. Показать поставку и следующую поставку этого же поставщика.
SELECT
    id,
    supplier_id,
    shipment_date,
    LEAD(shipment_date) OVER (PARTITION BY supplier_id ORDER BY shipment_date) AS next_shipment_date
FROM shipments
ORDER BY supplier_id, shipment_date;

-- 5. ОКОННЫЕ ФУНКЦИИ: КУМУЛЯТИВНЫЕ ИТОГИ (SUM, AVG OVER)
-- 5.1. Кумулятивная сумма количества поставок по месяцам для каждого поставщика.
SELECT
    supplier_id,
    DATE_TRUNC('month', shipment_date)  AS month,
    SUM(quantity) AS monthly_quantity,
    SUM(SUM(quantity)) OVER (PARTITION BY supplier_id ORDER BY DATE_TRUNC('month', shipment_date)) AS cumulative_quantity
FROM shipments
GROUP BY supplier_id, DATE_TRUNC('month', shipment_date)
ORDER BY supplier_id, month;

-- 5.2. Скользящее среднее количества поставок за последние 3 месяца (для каждого поставщика).
-- Для простоты сгруппируем по месяцам.
WITH monthly AS (
    SELECT
        supplier_id,
        DATE_TRUNC('month', shipment_date) AS month,
        SUM(quantity) AS total_qty
    FROM shipments
    GROUP BY supplier_id, month
)
SELECT
    supplier_id,
    month,
    total_qty,
    AVG(total_qty) OVER (PARTITION BY supplier_id ORDER BY month ROWS BETWEEN 2 PRECEDING AND CURRENT ROW) AS moving_avg_3m
FROM monthly
ORDER BY supplier_id, month;

-- 6. КОМБИНАЦИЯ CTE И ОКОННЫХ ФУНКЦИЙ (сложный пример)
-- 6.1. Найти поставщиков, у которых есть поставки с количеством выше среднего по их собственным поставкам.
WITH suppler_stats AS (
    SELECT supplier_id,
           quantity,
           AVG(quantity) OVER (PARTITION BY supplier_id) AS avg_qty_for_supplier
    FROM shipments
)
SELECT supplier_id
FROM suppler_stats
WHERE quantity > avg_qty_for_supplier;

-- 6.2. Для каждой поставки показать, какой процент от общей суммы поставок этого поставщика она составляет.
WITH supplier_totals AS (
    SELECT
        supplier_id,
        SUM(quantity) AS total_qty
    FROM shipments
    GROUP BY supplier_id
)
SELECT
    s.id AS shipment_id,
    s.supplier_id,
    s.quantity,
    st.total_qty,
    ROUND(100 * s.quantity/st.total_qty,2) AS percent_of_total
FROM shipments s
JOIN supplier_totals st ON s.supplier_id = st.supplier_id
ORDER BY s.supplier_id, s.shipment_date;

-- 7. РАЗНИЦА МЕЖДУ ROW_NUMBER, RANK, DENSE_RANK (НАГЛЯДНО)
WITH scores AS (
    SELECT 'Alice' AS name, 95 AS score UNION ALL
    SELECT 'Bob', 90 UNION ALL
    SELECT 'Charlie', 90 UNION ALL
    SELECT 'Diana', 85 UNION ALL
    SELECT 'Eve', 85
)
SELECT
    name,
    score,
    ROW_NUMBER() OVER (ORDER BY score DESC) AS row_num,
    RANK() OVER (ORDER BY score DESC) AS rank,
    DENSE_RANK() OVER (ORDER BY score DESC) AS dense_rank
FROM scores;


-- Задача 1.
-- Для каждой поставки выведи её id, product_id, quantity и порядковый номер
-- поставки в рамках одного product_id (по дате shipment_date).
-- Используй ROW_NUMBER().
-- Решение:
SELECT
    id,
    product_id,
    quantity,
    shipment_date,
    ROW_NUMBER() OVER (PARTITION BY product_id ORDER BY shipment_date) AS row_num
-- PARTITION BY product_id — группируем по продукту
-- ORDER BY shipment_date — нумеруем по дате (самая ранняя → 1)
FROM shipments
ORDER BY product_id, row_num;

-- Задача 2.
-- Для каждого поставщика выведи его name и общее количество поставок (shipment_count).
-- Отранжируй поставщиков по этому количеству по убыванию с использованием RANK().
-- Решение:
WITH supplier_counts AS (
    SELECT supplier_id, COUNT(*) AS shipment_count
    FROM shipments
    GROUP BY supplier_id
)
SELECT
    s.name,
    sc.shipment_count,
    RANK() OVER (ORDER BY sc.shipment_count DESC) AS rank
-- RANK() даст ранг с пропусками (1,2,2,4...)
FROM suppliers s
         JOIN supplier_counts sc ON s.id = sc.supplier_id
ORDER BY rank;

-- Задача 3.
-- Для каждой поставки выведи id, supplier_id, shipment_date и дату предыдущей
-- поставки этого же поставщика (используй LAG). Если предыдущей нет, NULL.
-- Решение:
SELECT
    id,
    supplier_id,
    shipment_date,
    LAG(shipment_date) OVER (PARTITION BY supplier_id ORDER BY shipment_date) AS prev_shipment_date
-- LAG берёт значение из предыдущей строки внутри группы
FROM shipments
ORDER BY supplier_id, shipment_date;

-- Задача 4.
-- Для каждого продукта выведи его name, category, общее количество поставок
-- и ранг продукта по количеству поставок внутри его категории (по убыванию).
-- Используй RANK().
-- Решение:
WITH product_count AS (
    SELECT
        product_id, COUNT(*) AS shipment_count
    FROM shipments
    GROUP BY product_id
)
SELECT
    p.name,
    p.category,
    pc.shipment_count,
    RANK() OVER (PARTITION BY p.category ORDER BY pc.shipment_count DESC ) AS rank
FROM products p
JOIN product_count pc ON p.id = pc.product_id
ORDER BY p.category, rank;

-- Задача 5.
-- Для каждой поставки выведи id, supplier_id, shipment_date, quantity,
-- а также разницу в днях между текущей и предыдущей поставкой этого же поставщика.
-- Решение:
SELECT
    id,
    supplier_id,
    shipment_date,
    quantity,
    shipment_date - LAG(shipment_date) OVER (PARTITION BY supplier_id ORDER BY shipment_date) AS day_diff
FROM shipments
ORDER BY supplier_id, day_diff;

-- Задача 6.
-- Выведи для каждого поставщика его name, общую сумму количества поставок (total_qty),
-- а также кумулятивную сумму поставок по всем поставщикам,
-- отсортированную по total_qty по возрастанию.
-- Используй SUM() OVER (ORDER BY total_qty).
WITH supplier_totals AS (
    SELECT supplier_id, SUM(quantity) AS total_qty
    FROM shipments
    GROUP BY supplier_id
)
SELECT
    s.name,
    st.total_qty,
    SUM(st.total_qty) OVER (ORDER BY st.total_qty) AS cumulative_total
-- Кумулятивная сумма всех total_qty, упорядоченных по возрастанию
FROM suppliers s
JOIN supplier_totals st ON s.id = st.supplier_id
ORDER BY st.total_qty;

-- Задача 7.
-- Для каждого продукта выведи его name, category, общее количество поставок,
-- а также долю этого продукта в общем количестве поставок по всей таблице (в %).
-- Используй оконную функцию для получения общего количества.
-- Решение:
WITH product_counts AS (
    SELECT
        product_id,
        COUNT(*) AS shipment_count
    FROM shipments
    GROUP BY product_id
),
    total_all AS(
        SELECT SUM(shipment_count) AS grand_total FROM product_counts
    )
SELECT
    p.name,
    p.category,
    pc.shipment_count,
    ROUND(100 * pc.shipment_count / (SELECT grand_total FROM total_all), 2) AS percent_of_total
-- Подзапрос возвращает общее количество поставок
FROM products p
JOIN product_counts pc ON p.id = pc.product_id
ORDER BY percent_of_total DESC;

-- Задача 8.
-- Для каждого поставщка найди его самую последнюю поставку (по shipment_date).
-- -- Выведи supplier_id, иshipment_date, product_id, quantity.
-- Используй ROW_NUMBER() с фильтром.
-- Решение:
WITH ranked AS (
    SELECT
        supplier_id,
        shipment_date,
        product_id,
        quantity,
        ROW_NUMBER() OVER (PARTITION BY supplier_id ORDER BY shipment_date DESC) AS rn
    -- Нумеруем от самой свежей поставки
    FROM shipments
)
SELECT supplier_id, shipment_date, product_id, quantity
FROM ranked
WHERE rn = 1;  -- оставляем только последнюю

--БЕЗ CTE
SELECT DISTINCT ON (supplier_id)
    supplier_id, shipment_date, product_id, quantity
FROM shipments
ORDER BY supplier_id, shipment_date DESC;

-- Задача 9.
-- Для каждого месяца (по shipment_date) выведи общее количество поставок
-- и кумулятивную сумму поставок с начала времён (по месяцам).
-- Используй SUM() OVER (ORDER BY month).
-- Решение:
WITH monthly AS (
    SELECT DATE_TRUNC('month', shipment_date) AS month,
           COUNT(*) AS total_shipments
    FROM shipments
    GROUP BY month
)
SELECT
    month,
    total_shipments,
    SUM(total_shipments) OVER (ORDER BY month) AS cumulative_shipments
-- Накопительная сумма по месяцам
FROM monthly
ORDER BY month;

-- Задача 10.
-- Найди топ-2 продукта по количеству поставок в каждой категории.
-- Выведи category, product_name, shipment_count.
-- Используй ROW_NUMBER().
-- Решение:
WITH product_counts AS (
    SELECT product_id, COUNT(*) AS shipment_count
    FROM shipments
    GROUP BY product_id
),
ranked AS (
    SELECT
        p.category,
        p.name AS product_name,
        pc.shipment_count,
        ROW_NUMBER() OVER (PARTITION BY p.category ORDER BY pc.shipment_count DESC) AS rn
    FROM products p
    JOIN product_counts pc ON p.id = pc.product_id
)
SELECT category, product_name, shipment_count
FROM ranked
WHERE rn <= 2  -- берём только топ-2 в каждой категории
ORDER BY category, rn;
