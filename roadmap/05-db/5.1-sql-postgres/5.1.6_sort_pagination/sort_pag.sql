/*
ШАГ 1.6: СОРТИРОВКА И ПАГИНАЦИЯ (ORDER BY, LIMIT, OFFSET)

ТЕОРИЯ:
--------

1. ORDER BY — сортировка результатов запроса.
   Синтаксис: ORDER BY column1 [ASC | DESC] [, column2 [ASC | DESC], ...]
   - ASC — по возрастанию (по умолчанию)
   - DESC — по убыванию
   - Можно сортировать по нескольким полям: сначала по первому, потом по второму и т.д.
   - NULLS FIRST / NULLS LAST — управление положением NULL значений.
   - Можно сортировать по вычисляемым выражениям (например, price * stock).

2. LIMIT — ограничение количества возвращаемых строк.
   Синтаксис: LIMIT n
   - Возвращает не более n строк.
   - Часто используется с ORDER BY для получения "топ-N" записей.

3. OFFSET — пропуск строк перед возвратом результата.
   Синтаксис: OFFSET m
   - Пропускает первые m строк.
   - Используется для пагинации: страница = OFFSET + LIMIT.
   - Формула: OFFSET = (page - 1) * limit.

4. ПРОБЛЕМА БОЛЬШОГО OFFSET:
   - При большом OFFSET (например, 10000) база данных всё равно читает и сортирует все строки до этого смещения,
     что приводит к падению производительности.
   - Альтернатива: Keyset Pagination (пагинация по курсору).

5. KEYSET PAGINATION (пагинация по курсору):
   - Использует WHERE для выборки следующих строк на основе последнего значения из предыдущей страницы.
   - Пример: WHERE id > last_id ORDER BY id LIMIT n.
   - Для сортировки по нескольким полям используются кортежи: WHERE (city, id) > (last_city, last_id).
   - Для DESC-сортировки условие строится сложнее: (age < last_age) OR (age = last_age AND id > last_id).
   - Keyset работает быстрее, так как использует индекс и не сканирует пропущенные строки.

6. ПРАКТИЧЕСКИЕ РЕКОМЕНДАЦИИ:
   - Всегда используйте ORDER BY с LIMIT, чтобы гарантировать стабильный порядок.
   - Для больших таблиц предпочитайте Keyset Pagination вместо OFFSET.
   - Используйте индексы на поля, по которым происходит сортировка и фильтрация.

7. СВЯЗЬ С GO:
   - В HTTP-эндпоинтах пагинация передаётся через query-параметры: page, limit.
   - В коде рассчитывается OFFSET = (page - 1) * limit.
   - Для Keyset передаются last_id, last_city и т.д.
   - Результат часто дополняется общим количеством записей (total) для фронтенда.

*/

CREATE TABLE products (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT,
    price NUMERIC(10,2) CHECK (price > 0) NOT NULL,
    stock INT CHECK (stock >= 0) DEFAULT 0,
    category TEXT,
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX idx_products_category ON products(category);

INSERT INTO products (name, description, price, stock, category, is_active) VALUES
    ('Laptop Pro', 'High-end laptop', 1500.00, 10, 'Electronics', TRUE),
    ('Smartphone X', 'Latest model', 800.00, 25, 'Electronics', TRUE),
    ('Wireless Mouse', 'Ergonomic mouse', 25.99, 50, 'Accessories', TRUE),
    ('Mechanical Keyboard', 'RGB mechanical keyboard', 120.00, 30, 'Accessories', TRUE),
    ('USB-C Cable', '2m cable', 9.99, 100, 'Accessories', TRUE),
    ('Monitor 27"', '4K monitor', 450.00, 15, 'Electronics', FALSE),
    ('External SSD', '1TB portable SSD', 200.00, 20, 'Storage', TRUE),
    ('Memory Card 128GB', 'MicroSD card', 30.00, 80, 'Storage', TRUE),
    ('Smart Watch', 'Fitness tracker', 250.00, 12, 'Wearables', TRUE),
    ('Bluetooth Headphones', 'Noise-cancelling', 150.00, 18, 'Audio', TRUE),
    ('Laptop Bag', 'Waterproof bag', 45.00, 40, 'Accessories', TRUE),
    ('USB Hub', '7-port USB hub', 35.00, 35, 'Electronics', TRUE),
    ('Webcam HD', '1080p webcam', 60.00, 22, 'Electronics', FALSE),
    ('Gaming Chair', 'Ergonomic gaming chair', 300.00, 8, 'Furniture', TRUE),
    ('Desk Mat', 'Large mouse pad', 25.00, 60, 'Accessories', TRUE),
    ('Power Bank', '20000mAh power bank', 50.00, 45, 'Electronics', TRUE),
    ('Wireless Charger', 'Qi wireless charger', 30.00, 30, 'Accessories', TRUE),
    ('VR Headset', 'Virtual reality headset', 400.00, 5, 'Electronics', TRUE),
    ('Smart Speaker', 'Voice assistant speaker', 120.00, 20, 'Audio', TRUE),
    ('Fitness Band', 'Activity tracker', 80.00, 25, 'Wearables', FALSE),
    ('Tablet', '10-inch tablet', 350.00, 15, 'Electronics', TRUE),
    ('Stylus Pen', 'Digital pen', 40.00, 50, 'Accessories', TRUE),
    ('Keyboard Cover', 'Silicone cover', 15.00, 70, 'Accessories', TRUE),
    ('HDMI Cable', '2m HDMI cable', 12.00, 90, 'Electronics', TRUE),
    ('Router', 'Wi-Fi 6 router', 180.00, 10, 'Networking', TRUE),
    ('Network Switch', '8-port switch', 90.00, 12, 'Networking', TRUE),
    ('Cat6 Cable', '3m Ethernet cable', 8.00, 150, 'Networking', TRUE),
    ('SATA SSD', '500GB SSD', 60.00, 30, 'Storage', TRUE),
    ('External HDD', '2TB external drive', 100.00, 18, 'Storage', TRUE),
    ('USB Flash Drive', '64GB USB drive', 15.00, 80, 'Storage', TRUE),
    ('Digital Camera', '4K camera', 600.00, 7, 'Photography', TRUE),
    ('Tripod', 'Aluminum tripod', 70.00, 20, 'Photography', TRUE),
    ('Lens Filter', 'UV filter', 25.00, 35, 'Photography', FALSE),
    ('Photo Printer', 'Wireless photo printer', 200.00, 10, 'Photography', TRUE),
    ('Projector', 'Full HD projector', 350.00, 6, 'Electronics', TRUE),
    ('Soundbar', 'Wireless soundbar', 180.00, 14, 'Audio', TRUE),
    ('Microphone', 'USB microphone', 90.00, 22, 'Audio', TRUE),
    ('Studio Headphones', 'Professional headphones', 120.00, 16, 'Audio', TRUE),
    ('MIDI Controller', 'USB MIDI controller', 150.00, 8, 'Music', TRUE),
    ('Guitar Cable', '3m instrument cable', 10.00, 60, 'Music', TRUE),
    ('Music Stand', 'Portable music stand', 30.00, 25, 'Music', TRUE),
    ('Piano Keyboard', 'Digital piano', 500.00, 5, 'Music', FALSE),
    ('Drum Sticks', 'Wooden drum sticks', 15.00, 40, 'Music', TRUE),
    ('Guitar Pick', 'Pack of 10 picks', 5.00, 100, 'Music', TRUE),
    ('Tuning Fork', 'A440 tuning fork', 12.00, 30, 'Music', TRUE),
    ('Metronome', 'Digital metronome', 25.00, 20, 'Music', TRUE),
    ('Sheet Music', 'Piano sheet music', 10.00, 50, 'Music', TRUE),
    ('Karaoke Machine', 'Portable karaoke', 120.00, 10, 'Audio', TRUE),
    ('DJ Controller', 'USB DJ controller', 250.00, 6, 'Audio', TRUE),
    ('Studio Monitors', 'Active studio monitors', 350.00, 8, 'Audio', TRUE);

-- 1. ORDER BY — СОРТИРОВКА

-- УРОВЕНЬ 1: Одно поле, ASC/DESC

-- 1.1. Сортировка по цене (от дешёвых к дорогим)
    SELECT * FROM products ORDER BY price;

-- 1.2. Сортировка по цене (от дорогих к дешёвым)
    SELECT * FROM products ORDER BY price DESC;

-- 1.3. Сортировка по названию (алфавит)
SELECT * FROM products ORDER BY name;

-- 1.4. Сортировка по количеству на складе (от большего к меньшему)
SELECT * FROM products ORDER BY stock DESC;

-- УРОВЕНЬ 2: Несколько полей, NULLS FIRST/LAST

-- 1.5. Сортировка по категории, затем по цене (дешёвые внутри категории)
SELECT * FROM products ORDER BY category, price;

-- 1.6. Сортировка по категории (по убыванию), внутри — по названию
SELECT * FROM products ORDER BY category DESC, name;

-- 1.7. Сортировка по категории, затем по активности (сначала активные)
SELECT * FROM products ORDER BY category, is_active DESC;

-- 1.8. Сортировка по цене с NULL (если бы были), но у нас NOT NULL
-- Показываем синтаксис: NULLS FIRST / LAST
SELECT * FROM products ORDER BY price NULL ;

-- 1.9. Сортировка по категории, затем по цене, но с NULL в конце
SELECT * FROM products ORDER BY category NULLS LAST, price;

-- УРОВЕНЬ 3: Вычисляемые поля, оконные функции

-- 1.10. Сортировка по общей стоимости (цена × остаток)
SELECT name, price, strock, price * stock AS total_value
FROM products
ORDER BY total_value DESC;

-- 1.11. Сортировка по длине названия
SELECT name, LENGTH(name) AS name_len
FROM products
ORDER BY name_len; 

-- 1.12. Сортировка с использованием оконной функции (ранг внутри категории по цене)
SELECT name, category, price
    RANK () OVER (PARTITION BY category ORDER BY price DESC) AS rank_in_category
FROM products
ORDER BY category, rank_in_category;

-- 2. LIMIT — ОГРАНИЧЕНИЕ КОЛИЧЕСТВА СТРОК (10 примеров с градацией)

-- УРОВЕНЬ 1
SELECT * FROM products LIMIT 5;

-- 2.2. Вывести первые 10 товаров
SELECT * FROM products LIMIT 10;

-- 2.3. Топ-5 самых дорогих товаров
SELECT * FROM products
ORDER BY price DESC LIMIT 5;

-- 2.4. Топ-3 товара с наибольшим остатком
SELECT * FROM products
ORDER BY stock DESC LIMIT 3

-- УРОВЕНЬ 2
-- 2.5. Первые 10 товаров из категории 'Electronics'
SELECT * FROM products
WHERE category = 'Electronics'
LIMIT 10;

-- 2.6. Топ-5 самых дорогих активных товаров
SELECT * FROM products
WHERE is_active = TRUE
ORDER BY price DESC
LIMIT 5;

-- 2.7. Топ-3 товара с наименьшим остатком (распродажа)
SELECT * FROM products
WHERE stock > 0
ORDER BY stock
LIMIT 5

-- 2.8. Первые 5 самых дорогих товаров, отсортированных по цене, но только из категорий 'Electronics' и 'Audio'
SELECT * FROM products
WHERE category IN ('Electronics', 'Audio')
ORDER BY price DESC
LIMIT 5;

-- УРОВЕНЬ 3 

-- 2.9. LIMIT с подзапросом: выбрать столько товаров, сколько категорий (всего категорий)
SELECT * FROM products
LIMIT (SELECT COUNT(DISTINCT category) FROM products);
-- 2.10. LIMIT случайных 5 записей (для рандомной выборки)
SELECT * FROM products
ORDER BY RANDOM()
LIMIT 5;

-- 3. OFFSET — ПАГИНАЦИЯ (14 примеров с градацией)

-- УРОВЕНЬ 1

-- 3.1. Пропустить первые 5 записей
SELECT * FROM products
ORDER BY id OFFSET 5;

-- 3.2. Пропустить первые 10 записей
SELECT * FROM products ORDER BY id OFFSET 10;

-- 3.3. Страница 1: 5 записей (OFFSET 0)
SELECT * FROM products ORDER BY id LIMIT 5 OFFSET 0;

-- 3.4. Страница 2: 5 записей
SELECT * FROM products ORDER BY id LIMIT 5 OFFSET 5;

-- 3.5. Страница 3: 5 записей
SELECT * FROM products ORDER BY id LIMIT 5 OFFSET 10;

-- УРОВЕНЬ 2

-- 3.6. Пагинация с WHERE: вторая страница активных товаров (limit=5)
SELECT * FROM products
WHERE is_active = TRUE
ORDER BY id
LIMIT 5 OFFSET 5;


-- 3.7. Вторая страница товаров категории 'Electronics', отсортированных по цене (limit=5)
SELECT * FROM products
WHERE category = 'Electronics'
ORDER BY price DESC
LIMIT 5 OFFSET 5;

-- 3.8. Страница 2 (limit=10) с сортировкой по имени
SELECT * FROM products
ORDER BY name
LIMIT 10 OFFSET 10;

-- 3.9. Расчёт OFFSET через переменную: страница 3, limit=8 → OFFSET 16
SELECT * FROM products
ORDER BY id
LIMIT 8 OFFSET (3-1)*8;

-- 3.10. Пагинация с двумя условиями: только активные и с ценой > 100
SELECT * FROM products
WHERE is_active = TRUE AND price > 100
ORDER BY price DESC
LIMIT 5 OFFSET 5;

-- УРОВЕНЬ 3

-- 3.11. OFFSET 1000 (медленно) — показать проблему
EXPLAIN ANALYZE
SELECT * FROM products
ORDER BY id
limit 10 OFFSET 1000;

-- 3.12. Keyset Pagination (быстрая альтернатива) — по id
-- Предположим, последний id на странице 1 = 5
SELECT * FROM products
WHERE id > 5
ORDER BY id
LIMIT 5

-- 3.13. Keyset с сортировкой по нескольким полям (кортеж: category, id)
-- Страница 1
SELECT * FROM products
ORDER BY category, id, LIMIT 5;
-- Последний элемент: category='Accessories', id=3
-- Страница 2
SELECT * FROM products
WHERE(category, id) > ('Accessories, 3')
ORDER BY category, id LIMIT 5;

-- 3.14. Keyset с сортировкой по DESC (price DESC, id ASC)
-- Страница 1
SELECT * FROM products ORDER BY price DESC, id LIMIT 5;
-- Последний: price=1500, id=1
-- Страница 2: (price < 1500) OR (price = 1500 AND id > 1)
SELECT * FROM products
WHERE price < 1500.00 OR (price = 1500.00 AND id > 1)
ORDER BY price DESC, id LIMIT 5;

-- 4. КОМБИНАЦИИ WHERE + ORDER BY + LIMIT + OFFSET

-- УРОВЕНЬ 1

-- 4.1. Топ-5 самых дорогих товаров
SELECT * FROM products ORDER BY price DESC LIMIT 5;

-- 4.2. Топ-5 самых дешёвых товаров
SELECT * FROM products ORDER BY price LIMIT 5;

-- 4.3. Вторая страница (limit=3, offset=3) по id
SELECT * FROM products ORDER BY id LIMIT 3 OFFSET 3;

-- 4.4. Первые 10 товаров, отсортированных по имени
SELECT * FROM products ORDER BY name LIMIT 10;

-- УРОВЕНЬ 2

-- 4.5. Активные товары, топ-10 по цене (страница 1)
SELECT * FROM products
WHERE is_active = TRUE
ORDER BY price DESC
LIMIT 10 OFFSET 0 

-- 4.6. Активные товары, топ-10 по цене (страница 2)
SELECT * FROM products
WHERE is_active = TRUE
ORDER BY price DESC
LIMIT 10 OFFSET 10;

SELECT * FROM products
WHERE is_active = TRUE
ORDER BY price DESC
LIMIT 10 OFFSET 10;

-- 4.7. Товары из категорий 'Electronics' или 'Audio', отсортированные по цене, вторая страница (limit=5)
SELECT * FROM products
WHERE category IN ('Electronics', 'Audio')
ORDER BY price DESC
LIMIT 5 OFFSET 5;

-- 4.8. Товары с ценой > 100, отсортированные по категории и цене, первые 10
SELECT * FROM products
WHERE price > 100
ORDER BY category, price DESC
LIMIT 10;

-- УРОВЕНЬ 3 

-- 4.9. Пагинация с общим количеством (total) для фронтенда
WITH total AS (
    SELECT COUNT(*) AS total FROM products
)
SELECT p.*, t.total
FROM products p, total t
ORDER BY p.id
LIMIT 10 OFFSET 20;

-- 4.10. Keyset + WHERE: получить следующие 5 товаров после (category='Electronics', id=5)
SELECT * FROM products
WHERE (category, id) > ('Electronics', 5)
ORDER BY category, id LIMIT 5;

-- 4.11. Пагинация с группировкой по категории (топ-2 самых дорогих в каждой категории)
WITH ranked AS (
    SELECT *,
           ROW_NUMBER() OVER (PARTITION BY category ORDER BY price DESC) AS rn
    FROM products
)
SELECT * FROM ranked
WHERE rn <= 2
ORDER BY category, price DESC;