/*
ШАГ 1.9: АГРЕГАЦИЯ (GROUP BY, HAVING)

ТЕОРИЯ

1. ЧТО ТАКОЕ АГРЕГАЦИЯ?
   - Агрегация — это процесс вычисления одного значения из множества строк.
   - Например: подсчёт количества заказов, сумма всех покупок, средний чек.
   - Агрегатные функции работают с группами строк.

2. АГРЕГАТНЫЕ ФУНКЦИИ (основные):
   - COUNT(*) — количество строк (включая NULL).
   - COUNT(column) — количество НЕ NULL значений в колонке.
   - SUM(column) — сумма значений.
   - AVG(column) — среднее арифметическое.
   - MIN(column) — минимальное значение.
   - MAX(column) — максимальное значение.
   - ARRAY_AGG(column) — собрать значения в массив.
   - STRING_AGG(column, delimiter) — собрать строки в одну с разделителем.

3. GROUP BY — ГРУППИРОВКА:
   - Разбивает строки на группы по одинаковым значениям в указанных колонках.
   - Агрегатные функции вычисляются отдельно для каждой группы.
   - SELECT может содержать только колонки из GROUP BY или агрегаты.
   - Порядок: FROM → JOIN → WHERE → GROUP BY → HAVING → SELECT → ORDER BY.

4. HAVING — ФИЛЬТР ПОСЛЕ ГРУППИРОВКИ:
   - WHERE фильтрует строки ДО группировки.
   - HAVING фильтрует группы ПОСЛЕ группировки.
   - В HAVING можно использовать агрегатные функции (в отличие от WHERE).
   - Например: "оставить только группы с COUNT(*) > 5".

5. ПРАВИЛА НАПИСАНИЯ:
   - Все колонки в SELECT, кроме агрегатов, должны быть в GROUP BY.
   - Используй псевдонимы (алиасы) для удобства.
   - HAVING обычно пишется после GROUP BY.

6. РАСШИРЕННЫЕ ВОЗМОЖНОСТИ:
   - Группировка по нескольким колонкам: GROUP BY col1, col2.
   - Группировка по выражению: GROUP BY DATE(created_at).
   - ROLLUP, CUBE, GROUPING SETS (для аналитики, редко на собесах).

7. СВЯЗЬ С GO:
   - Результат агрегации часто маппится в структуру с полями: Name, Total, Count и т.д.
   - Для Dashboard используются запросы с GROUP BY и сортировкой LIMIT.
   - Можно использовать оконные функции для ранжирования (позже).
*/

-- Таблица пользователей
CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Таблица товаров
CREATE TABLE products (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    category VARCHAR(100) NOT NULL,
    price DECIMAL(10, 2) NOT NULL CHECK (price > 0)
);

-- Таблица заказов
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Таблица позиций заказа (корзина)
CREATE TABLE order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    quantity INT NOT NULL CHECK (quantity > 0),
    price DECIMAL(10, 2) NOT NULL CHECK (price > 0) -- Цена за единицу в момент покупки
);

-- Заполняем пользователей
INSERT INTO users (name, email) VALUES
('Alice Wonderland', 'alice@mail.com'),
('Bob Builder', 'bob@mail.com'),
('Charlie Brown', 'charlie@mail.com'),
('Diana Prince', 'diana@mail.com'),
('Eve Online', 'eve@mail.com'),
('Frank Castle', 'frank@mail.com'); -- Этот ничего не купит

-- Заполняем товары
INSERT INTO products (name, category, price) VALUES
('iPhone 15', 'Electronics', 999.99),
('Samsung TV', 'Electronics', 1500.00),
('Sony Headphones', 'Audio', 299.99),
('JBL Speaker', 'Audio', 150.00),
('MacBook Pro', 'Computers', 2500.00),
('Office Chair', 'Furniture', 350.00),
('Desk Lamp', 'Furniture', 45.00);

-- Заполняем заказы
INSERT INTO orders (user_id, status, created_at) VALUES
(1, 'delivered', '2025-01-10 10:00:00'),
(1, 'delivered', '2025-02-15 14:30:00'),
(2, 'delivered', '2025-01-20 09:00:00'),
(2, 'pending',   '2025-03-01 16:00:00'),
(3, 'cancelled', '2025-01-05 11:00:00'),
(3, 'delivered', '2025-02-10 12:00:00'),
(4, 'delivered', '2025-03-15 08:00:00'),
(4, 'delivered', '2025-03-20 08:00:00'),
(5, 'delivered', '2025-02-28 17:00:00');

-- Заполняем позиции заказов (order_items)
-- Заказ 1 (Alice): 1 iPhone
INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
(1, 1, 1, 999.99);

-- Заказ 2 (Alice): 1 MacBook + 1 Мышь (допустим, мыши нет в таблице, добавим виртуально, но лучше возьмем существующий продукт -> 1 Ноутбук)
-- Заказ 2 (Alice): 1 MacBook Pro
INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
(2, 5, 1, 2500.00);

-- Заказ 3 (Bob): 1 Samsung TV + 1 Sony Headphones
INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
(3, 2, 1, 1500.00),
(3, 3, 2, 299.99); -- 2 штуки

-- Заказ 4 (Bob): 1 JBL Speaker (pending, не доставлен)
INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
(4, 4, 1, 150.00);

-- Заказ 5 (Charlie): отменен -> 1 Office Chair
INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
(5, 6, 1, 350.00);

-- Заказ 6 (Charlie): 1 Desk Lamp + 1 Sony Headphones
INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
(6, 7, 2, 45.00),   -- 2 лампы
(6, 3, 1, 299.99);

-- Заказ 7 (Diana): 1 iPhone + 1 JBL Speaker
INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
(7, 1, 1, 999.99),
(7, 4, 1, 150.00);

-- Заказ 8 (Diana): 1 MacBook Pro
INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
(8, 5, 1, 2500.00);

-- Заказ 9 (Eve): 1 Samsung TV
INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
(9, 2, 1, 1500.00);

-- 3. ПРАКТИЧЕСКИЕ ПРИМЕРЫ (ГРАДАЦИЯ СЛОЖНОСТИ)

-- УРОВЕНЬ 1: ЛЕГКИЕ

-- Пример 1.1: Количество товаров в каждой категории.
-- Простейшая группировка по одному полю.
SELECT category, COUNT(*) AS total_products
FROM products
GROUP BY category
ORDER BY total_products DESC;

-- Пример 1.2: Общая стоимость товаров в каждой категории (цена * количество).
-- Группировка с SUM и вычисляемым полем.
SELECT category, SUM(price) AS total_category_value
FROM products
GROUP BY category
ORDER BY total_category_value DESC;

-- Пример 1.3: Сколько заказов сделал каждый пользователь (просто COUNT).
-- JOIN + GROUP BY.
SELECT u.id, u.name, COUNT(o.id) AS order_count
FROM users u
JOIN orders o ON u.id = o.user_id
ORDER BY u.id, u.name
ORDER BY order_count DESC;

-- УРОВЕНЬ 2: СРЕДНИЕ
-- Пример 2.1: Общая сумма покупок по каждому пользователю (только доставленные).
-- Фильтр WHERE (до группировки) + SUM.
SELECT u.id, u.name,
      COALESCE(SUM(oi.quantity * oi.price)) AS total_spent
FROM users u
LEFT JOIN orders o ON u.id = o.user_id AND status = 'delivered'
LEFT JOIN oreder_item oi ON o.id = oi.order_id
GROUP BY u.id, u.name
ORDER BY total_spent DESC;  

-- Пример 2.2: Группировка по нескольким полям (Пользователь + Статус заказа).
-- Сколько заказов в каждом статусе у каждого пользователя.
SELECT u.id, o.status,
      COUNT(*) AS order_count
FROM users u
JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name, o.status
ORDER BY u.name, o.status;

-- Пример 2.3: HAVING - Найти пользователей, у которых общая сумма покупок > 3000.
-- Фильтр после группировки.

SELECT
   u.name
   COALESCE(SUM(oi.quantity * oi.price), 0) AS total_spent
FROM users u
LEFT JOIN orders o ON u.id = o.user_id AND o.status = 'delivered'
LEFT JOIN order_item oi ON o.id = oi.order_id
GROUP BY u.id, u.name
HAVING COALESCE(SUM(oi.quantity * oi.price), 0) > 500
ORDER BY total_spent DESC;

-- Пример 2.4: Условная агрегация (COUNT с фильтром внутри).
-- Посчитать количество доставленных, отмененных и ожидающих заказов для каждого пользователя.
-- Используем CASE внутри COUNT.
SELECT
   u.name
   COUNT(o.id) AS total_orders,
   COUNT(CASE WHEN o.status = 'delivered' THEN 1 AND) AS delivered_orders,
   COUNT(CASE WHEN o.status = 'canceled' THEN 1 AND) AS canceled_orderes,
   COUNT(CASE WHEN o.status = 'pending' THEN 1 AND) AS pending_orderes,
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
GROUP BY u.id, u.name
ORDER BY total_orders DESC;

-- УРОВЕНЬ 3: СЛОЖНЫЕ
-- Пример 3.1: Группировка по вычисляемому полю (CASE) - создаем ценовые сегменты.
-- Найти, сколько товаров попадает в категории цен: 'Бюджетный' (< 200), 'Средний' (200-1000), 'Премиум' (> 1000).
SELECT
   CASE
      WHEN price < 200 THEN 'Budget'
      WHEN price >= 200 AND price < 1000 THEN 'Mid'
      ELSE 'Premium'
   AND AS price_segment,
   COUNT(*) AS product_count,
   AVG(price) AS avg_price   
FROM products
GROUP BY price_segment
ORDER BY avg_price DESC;

-- Пример 3.2: Группировка с несколькими агрегатами и фильтром HAVING на сложное условие.
-- Показать пользователей, у которых СРЕДНИЙ чек (avg per order) больше 1500.
-- Здесь нужно сначала считать сумму по заказам, а потом усреднять по пользователю.
-- Используем CTE (Common Table Expression) для читаемости.

WITH order_total AS (
   SELECT
      o.user_id,
      o.id AS order_id
      SUM(oi.quantity * oi.price) AS order_total
   FROM order o 
   JOIN order_item oi ON o.id = oi.order_id
   WHERE o.status = 'delivered'
   GROUP BY o.user_id, o.id   
)
SELECT
   u.name
   COUNT(ot.order_id) AS total_orders,
    AVG(ot.order_total) AS avg_order_value
FROM users u
JOIN order_totals ot ON u.id = ot.user_id
GROUP BY u.id, u.name
HAVING AVG(ot.order_total) > 1500
ORDER BY avg_order_value DESC;    

-- Пример 3.3: ROLLUP / CUBE (OLAP) - Итоговые суммы по категориям с общим итогом.
-- Получаем сумму цен товаров по категориям, а в конце общую сумму всех товаров.
-- (Сложный уровень, т.к. требует понимания многомерных агрегаций).
SELECT 
    COALESCE(category, 'Grand Total') AS category,
    SUM(price) AS total_value,
    COUNT(*) AS total_count
FROM products
GROUP BY ROLLUP(category)
ORDER BY category NULLS LAST;

-- Пример 3.4: HAVING + Подзапрос + Сравнение с лучшим результатом.
-- Найти пользователей, которые потратили больше, чем средний показатель всех пользователей.
-- Используем коррелированный подзапрос или разделим на 2 этапа.
-- Вариант с CTE для сложной аналитики
WITH user_spending AS(
   SELECT
      u.id,
      u.name,
      COALESCE(SUM(oi.quantity * oi.price), 0) AS total_spent
   FROM users u
   LEFT JOIN orders o ON u.id = o.user_id AND o.status = 'delivered'
   LEFT JOIN order_items oi ON o.id = oi.order_id
   GROUP BY u.id, u.name   
),
global_avg AS(
   SELECT AVG(total_spent) AS avg_spent FROM user_spending
)
SELECT 
    us.name,
    us.total_spent,
    ga.avg_spent
FROM user_spending us, global_avg ga
WHERE us.total_spent > ga.avg_spent
ORDER BY us.total_spent DESC;

-- 4. БОНУС: ТОП-5 ДЛЯ ДАШБОРДА (ГОТОВЫЙ ЗАПРОС)
-- Именно этот запрос обычно улетает в Go-приложение.
-- Выводит Топ-5 пользователей по сумме покупок (только доставленные).

SELECT 
    u.id,
    u.name,
    COALESCE(SUM(oi.quantity * oi.price), 0) AS total_spent,
    COUNT(DISTINCT o.id) AS orders_count,
    MAX(o.created_at) AS last_order_date
FROM users u
LEFT JOIN orders o ON u.id = o.user_id AND o.status = 'delivered'
LEFT JOIN order_items oi ON o.id = oi.order_id
GROUP BY u.id, u.name
HAVING COALESCE(SUM(oi.quantity * oi.price), 0) > 0 -- Убираем нулевых, если не нужны
ORDER BY total_spent DESC
LIMIT 5;