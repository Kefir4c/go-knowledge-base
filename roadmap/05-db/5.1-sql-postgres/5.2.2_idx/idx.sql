/*
=============================================================================
ШАГ 2.2: ИНДЕКСЫ (B-TREE, СОСТАВНЫЕ, ЧАСТИЧНЫЕ, ПО ВЫРАЖЕНИЮ)
=============================================================================

ТЕОРИЯ (РАСШИРЕННАЯ)
--------------------

1. ЧТО ТАКОЕ ИНДЕКС?
   - Индекс — это структура данных, которая ускоряет поиск строк в таблице.
   - Похож на оглавление в книге: вместо того чтобы читать всю книгу,
     ты смотришь оглавление и переходишь на нужную страницу.
   - Без индекса база делает полный перебор таблицы (Seq Scan) — медленно.
   - С индексом база выполняет Index Scan — быстро находит нужные строки.

2. ТИПЫ ИНДЕКСОВ (В POSTGRESQL):
   - B-tree (сбалансированное дерево) — это самый распространенный тип индекса в PostgreSQL.
Он поддерживает все стандартные операции сравнения (>, <, >=, <=, =, <>) и может использоваться
с большинством типов данных. B-tree индексы могут быть использованы для сортировки,
ограничений уникальности и поиска по диапазону значений.

   - Hash — Hash-индексы предназначены для обеспечения быстрого доступа к данным по равенству.
Они менее эффективны, чем B-tree индексы, и не поддерживают сортировку или поиск по диапазону
значений. Из-за своих ограничений, Hash-индексы редко используются на практике.

   - GIN — GIN-индексы применяются для полнотекстового поиска и поиска по массивам, JSON и триграммам.
Они обеспечивают высокую производительность при поиске в больших объемах данных.

   - GiST — GiST-индексы являются обобщенными и многоцелевыми, предназначены для работы с сложными
типами данных, такими как геометрические объекты, текст и массивы. Они позволяют быстро
выполнять поиск по пространственным, текстовым и иерархическим данным.

    - SP-GiST - SP-GiST индексы предназначены для работы с непересекающимися и неравномерно
распределенными данными. Они эффективны для поиска в геометрических и IP-адресных данных.

   - BRIN — BRIN-индексы используются для компактного представления больших объемов данных,
особенно когда значения в таблице имеют определенный порядок. Они эффективны для хранения
и обработки временных рядов и географических данных.

   Мы фокусируемся на B-Tree, потому что он используется в 90% случаев.

3. КОГДА ИНДЕКС НЕ ИСПОЛЬЗУЕТСЯ (даже если он есть):
   - Если условие WHERE содержит функцию на колонке (например, UPPER(name) = 'ALEX') —
     если нет индекса по выражению.
   - Если условие содержит LIKE с ведущим '%' (например, name LIKE '%smith').
   - Если выборка возвращает > 10-20% строк таблицы (тогда индекс невыгоден,
     база выберет Seq Scan).
   - Если таблица очень маленькая (несколько сотен строк) — быстрее прочитать всю.

4. ВИДЫ ИНДЕКСОВ ПО СТРУКТУРЕ:
   - Обычный (один столбец) — самый простой.
   - Составной (по нескольким колонкам) — порядок колонок важен!
     * Индекс (a, b) ускоряет WHERE a = ... и WHERE a = ... AND b = ...
     * НЕ ускоряет WHERE b = ... (если a не указана).
   - Уникальный (UNIQUE INDEX) — гарантирует уникальность значений, работает как ограничение.
   - Частичный (Partial Index) — только для подмножества строк (WHERE условие).
     * Например, только для активных пользователей (active = true).
   - Индекс по выражению — например, LOWER(email) для регистронезависимого поиска.

5. КАК ПРОВЕРИТЬ, ИСПОЛЬЗУЕТСЯ ЛИ ИНДЕКС?
   - Используй EXPLAIN ANALYZE перед запросом.
   - Ищи в плане Index Scan, Index Only Scan, Bitmap Heap Scan.
   - Если видишь Seq Scan — индекс не используется (или его нет).

6. СВЯЗЬ С GO:
   - Когда ты в Go выполняешь запрос SELECT ... WHERE email = ?, и он тормозит,
     первое, что нужно сделать — создать индекс на email.
   - После создания индекса перезапускать приложение не нужно — индекс сразу доступен.
   - Важно: индексы замедляют INSERT, UPDATE, DELETE (потому что индекс тоже надо обновлять).
     Поэтому индексы создают только под реальные запросы, не "на всякий случай".

7. ПРИМЕРЫ РЕАЛЬНЫХ СЦЕНАРИЕВ:
   - Поиск по email в таблице пользователей (обычный индекс).
   - Поиск по дате + статусу заказа (составной индекс).
   - Поиск только среди активных товаров (частичный индекс).
   - Регистронезависимый поиск по названию (индекс по LOWER(name)).

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
    (random() * 9 + 1)::int AS product_id,
    (random() * 3 + 1)::int AS warehouse_id,
    (random() * 4 + 1)::int AS supplier_id,
    (random() * 100 + 1)::int AS quantity,
    NOW() - (random() * 365) * INTERVAL '1 day' AS shipment_date,
    CASE
        WHEN random() > 0.3 THEN NOW() - (random() * 30) * INTERVAL '1 day'
        ELSE NULL
        END AS delivery_date,
    CASE
        WHEN random() < 0.1 THEN 'cancelled'
        WHEN random() < 0.3 THEN 'pending'
        WHEN random() < 0.6 THEN 'in_transit'
        ELSE 'delivered'
        END AS status
FROM generate_series(1, 500000);

-- 2. ЗАПРОСЫ БЕЗ ИНДЕКСОВ (ДЛЯ СРАВНЕНИЯ)
-- 2.1. Поиск по SKU (артикулу) — уникальное поле
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM products WHERE sku = 'NB-002';
-- Ожидаем Seq Scan (т.к. индекса нет) — будет медленно на 10k+ строк.

-- 2.2. Поиск по дате поставки (диапазон)
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM shipments
WHERE shipment_date BETWEEN '2023-01-01' AND '2023-12-31';
-- Ожидаем Seq Scan

-- 2.3. Поиск по статусу и дате (составной фильтр)
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM shipments
WHERE status = 'delivered' AND shipment_date = '2023-06-01';

-- 2.4. Регистронезависимый поиск по email поставщика
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM suppliers WHERE LOWER(email) = 'roga@mail.ru';

-- 2.5. Поиск только по активным товарам (частичный фильтр)
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM products WHERE is_active = TRUE AND price > 100;

-- 3. СОЗДАНИЕ ИНДЕКСОВ (ПО ОЧЕРЕДИ, С ЗАМЕРОМ ВРЕМЕНИ)
-- 3.1. Обычный B-Tree индекс на SKU (уникальный, т.к. поле UNIQUE, но индекс не создаётся
-- автоматически на UNIQUE? В PostgreSQL UNIQUE создаёт уникальный индекс, но мы создадим явно для демонстрации)
CREATE INDEX idx_products_sku ON products(sku);

-- 3.2. Индекс на дату поставки
CREATE INDEX idx_shipments_shipment_date ON shipments(shipment_date);

-- 3.3. Составной индекс (статус + дата) — порядок важен!
-- Если мы часто ищем по статусу И дате, то индекс (status, shipment_date) ускорит оба условия.
-- Но если будем искать только по дате, этот индекс не поможет — нужен отдельный.
CREATE INDEX idx_shipments_status_date ON shipments(status, shipment_date);

-- 3.4. Индекс по выражению (для регистронезависимого поиска email)
CREATE INDEX idx_suppliers_email_lower ON suppliers(LOWER(email));

-- 3.5. Частичный индекс — только для активных товаров
CREATE INDEX idx_products_active_price ON products(price) WHERE is_active = TRUE;

-- 4. ПРОВЕРКА, ЧТО ИНДЕКСЫ РАБОТАЮТ (EXPLAIN ANALYZE)

-- 4.1. Поиск по SKU — теперь должен использовать Index Scan
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM products WHERE sku = 'NB-002';
-- Ожидаем Index Scan using idx_products_sku

-- 4.2. Поиск по дате — Index Scan или Index Only Scan
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM shipments
WHERE shipment_date BETWEEN '2023-01-01' AND '2023-12-31';
-- Ожидаем Index Scan using idx_shipments_shipment_date

-- 4.3. Поиск по статусу + дате — составной индекс
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM shipments
WHERE status = 'delivered' AND shipment_date > '2023-06-01';
-- Ожидаем Index Scan using idx_shipments_status_date

-- 4.4. Регистронезависимый поиск — индекс по выражению
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM suppliers WHERE LOWER(email) = 'roga@mail.ru';
-- Ожидаем Index Scan using idx_suppliers_email_lower

-- 4.5. Поиск по активным товарам с ценой — частичный индекс
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM products WHERE is_active = TRUE AND price > 100;
-- Ожидаем Index Scan using idx_products_active_price

-- 5. СРАВНЕНИЕ СКОРОСТИ (БЕЗ ИНДЕКСА VS С ИНДЕКСОМ)
-- Чтобы увидеть разницу, замерь время выполнения до и после создания индекса.
-- Например, включи тайминг в psql: \timing on
-- Затем выполни запрос 4.3 и посмотри время.

-- 6. ДОПОЛНИТЕЛЬНЫЕ НЮАНСЫ
-- 6.1. Если индекс не используется (Seq Scan) — проверьте:
-- - Не стоит ли условие WHERE на колонке, которая участвует в индексе, но с функцией (например, UPPER(name)) — без индекса по выражению.
-- - Не используете ли LIKE с ведущим '%'.
-- - Не выбираете ли вы слишком много строк (> 20% таблицы) — тогда Seq Scan выгоднее.

-- 6.2. Составной индекс (a, b) используется для:
-- - WHERE a = ...
-- - WHERE a = ... AND b = ...
-- НЕ используется для:
-- - WHERE b = ... (без a)

-- 6.3. Чтобы удалить индекс:
-- DROP INDEX idx_products_sku;

-- 6.4. Уникальный индекс автоматически создаётся при добавлении UNIQUE-ограничения,
-- но можно и явно: CREATE UNIQUE INDEX idx_unique_email ON users(email);

-- Задрачиваем индексы :)

-- УРОВЕНЬ 1: ОДИН СТОЛБЕЦ (ПРОСТЫЕ)
---Задача 1
    --- SELECT * FROM suppliers WHERE email = 'roga@mail.ru';
    CREATE INDEX idx_suppliers_email ON suppliers(email);
---Задача 2
    SELECT * FROM shipments WHERE shipment_date > '2023-01-01';
    CREATE INDEX idx_shipments_shipment_date ON shipments(shipment_date);
---Задача 3
    SELECT * FROM products WHERE category = 'Electronics';
    CREATE INDEX idx_products_category ON products(category);

-- УРОВЕНЬ 2: СОСТАВНЫЕ (ПОРЯДОК ВАЖЕН!)
---Задача 4
    SELECT * FROM shipments WHERE status = 'delivered' AND shipment_date = '2023-01-01';
    CREATE INDEX idx_shipments_status_shipment_date ON shipments(status, shipment_date);
---Задача 5
    SELECT * FROM shipments WHERE product_id = 5 AND warehouse_id = 3;
    CREATE INDEX idx_shipments_product_warehouse ON shipments(product_id, warehouse_id);
---Задача 6
    SELECT * FROM products WHERE category = 'Electronics' AND price > 100;
    CREATE INDEX idx_products_category_price ON products(category, price);

-- УРОВЕНЬ 3: ЧАСТИЧНЫЕ (ТОЛЬКО ДЛЯ ПОДМНОЖЕСТВА)
---Задача 7
-- (поставщиков с is_active = false почти никогда не ищут по email)
    SELECT * FROM suppliers WHERE is_active = true AND email = 'roga@mail.ru';
    CREATE INDEX idx_suppliers_active_email ON suppliers(email) WHERE is_active = true;
---Задача 8
-- (заказов со статусом pending всего 5%, остальные не нужны в индексе)
    SELECT * FROM shipments WHERE status = 'pending' AND shipment_date > NOW() - INTERVAL '1 day';
    CREATE INDEX idx_shipments_pending_date ON shipments(shipment_date) WHERE status = 'pending';

-- УРОВЕНЬ 4: ПО ВЫРАЖЕНИЮ (ФУНКЦИИ)
---Задача 9
    SELECT * FROM suppliers WHERE LOWER(email) = 'roga@mail.ru';
    CREATE INDEX idx_suppliers_email_lower ON suppliers(LOWER(email));
---Задача 10
    SELECT * FROM shipments WHERE EXTRACT(YEAR FROM shipment_date) = 2023;
    CREATE INDEX idx_shipments_year ON shipments(EXTRACT(YEAR FROM shipment_date));

--- УРОВЕНЬ 5: ПОКРЫВАЮЩИЕ (INCLUDE) – ДЛЯ INDEX ONLY SCAN
---Задача 11
-- (хотим, чтобы все данные брались из индекса, без обращения к таблице)
    SELECT id, name, email FROM suppliers WHERE is_active = true;
    CREATE INDEX idx_suppliers_active_cover ON suppliers(is_active) INCLUDE (name, email);
---Задача 12
-- (обычно поиск по первичному ключу и так быстр, но добавим покрывающий для примера)
    SELECT product_id, quantity, shipment_date FROM shipments WHERE id = 12345;
    CREATE INDEX idx_shipments_id_cover ON shipments(id) INCLUDE (product_id, quantity, shipment_date);
-- Но так как id и так первичный ключ, индекс не нужен. Для демонстрации можно сделать на другой колонке:
-- Например, частый запрос: SELECT product_id, quantity FROM shipments WHERE warehouse_id = 3;
-- Тогда индекс: CREATE INDEX idx_shipments_warehouse_cover ON shipments(warehouse_id) INCLUDE (product_id, quantity);

-- Поэтому переформулируем задачу 12 под реальный случай:
-- Запрос: SELECT product_id, quantity FROM shipments WHERE warehouse_id = 3 AND shipment_date > '2023-01-01';
-- Но тут два условия, лучше составной. Для покрывающего сделаем:
-- Запрос: SELECT product_id, quantity FROM shipments WHERE warehouse_id = 3;
-- Ответ (правильный для этой задачи):
    CREATE INDEX idx_shipments_warehouse_cover ON shipments(warehouse_id) INCLUDE (product_id, quantity);

-- 2. HASH – ТОЛЬКО ДЛЯ РАВЕНСТВА (=)
-- Используется, если ищешь только по равенству (=) и НЕ нужно сортировать (ORDER BY)
-- Быстрее B-Tree для оператора =, но медленнее для всего остального.

-- Пример: быстрый поиск телефона поставщика (точное совпадение)
CREATE INDEX idx_suppliers_phone_hash ON suppliers USING HASH (phone);
-- Проверка: EXPLAIN SELECT * FROM suppliers WHERE phone = '+7-999-111-22-33';

-- 3. GIN – ДЛЯ JSONB, МАССИВОВ, ПОЛНОТЕКСТОВОГО ПОИСКА

-- 3.1. GIN для JSONB (поиск внутри документа)
ALTER TABLE products ADD COLUMN attributes JSONB;
CREATE INDEX idx_products_attributes ON products USING GIN (attributes);

-- 3.2. GIN для массива (например, теги товаров)
ALTER TABLE products ADD COLUMN tags TEXT[];
CREATE INDEX idx_products_tags_gin ON products USING GIN (tags);

-- 3.3. GIN для полнотекстового поиска (tsvector)
ALTER TABLE  products ADD COLUMN search_vector tsvector
GENERATED ALWAYS AS (to_tsvector('russian', name || ' ' || COALESCE(category, ''))) STORED;
CREATE INDEX idx_products_search_gin ON products USING GIN (search_vector);
-- Пример запроса: SELECT * FROM products WHERE search_vector @@ to_tsquery('russian', 'ноутбук & pro');

-- 4.1. Добавляем геолокацию складам
ALTER TABLE warehouses ADD COLUMN location GEOGRAPHY(POINT, 4326);
CREATE INDEX idx_warehouses_location_gist ON warehouses USING GiST (location);
-- Пример запроса: SELECT * FROM warehouses WHERE ST_DWithin(location, ST_MakePoint(37.6, 55.7), 10000);

-- 4.2. GiST для диапазонов (например, доступность дат)
CREATE TABLE availability (
product_id BIGINT REFERENCES products(id) ON DELETE CASCADE,
period DATERANGE,
EXCLUDE USING gist (product_id WITH =, period WITH &&)
);
-- Здесь GiST используется для запрета пересечения периодов (Constraint Exclusion)
-- 5. SP-GiST – ДЛЯ НЕСБАЛАНСИРОВАННЫХ ДАННЫХ (IP, ТЕЛЕФОНЫ, ТОЧКИ)

-- 5.1. SP-GiST для IP-адресов
CREATE TABLE access_log (
id BIGSERIAL PRIMARY KEY,
ip INET,
access_time TIMESTAMPTZ DEFAULT NOW()
);
CREATE INDEX idx_access_log_ip_spgist ON access_log USING SPGiST (ip);

-- Пример: SELECT * FROM access_log WHERE ip <<= '192.168.1.0/24';

-- 5.2. SP-GiST для координат (многомерные точки)
CREATE TABLE delivery_points (
id BIGSERIAL PRIMARY KEY,
coord POINT
);
CREATE INDEX idx_delivery_points_spgist ON delivery_points USING SPGiST (coord);

-- 6. BRIN – ДЛЯ ОГРОМНЫХ ТАБЛИЦ С ЕСТЕСТВЕННЫМ ПОРЯДКОМ
-- BRIN работает только для таблиц, где данные физически отсортированы
-- (например, по дате, по возрастающему id)

-- 6.1. BRIN по дате поставки (shipment_date в shipments)
CREATE INDEX idx_shipments_date_brin ON shipments USING BRIN (shipment_date);

-- 6.2. BRIN по id (если id всегда возрастает)
CREATE INDEX idx_products_id_brin ON products USING BRIN (id);

-- Проверка: EXPLAIN SELECT * FROM shipments WHERE shipment_date BETWEEN '2023-01-01' AND '2023-12-31';

-- 7. СОСТАВНЫЕ ИНДЕКСЫ ДЛЯ РАЗНЫХ ТИПОВ (GIN + B-TREE)
-- Например, ищем по категории (B-Tree) и внутри JSONB (GIN) одновременно
-- Создаём два индекса, а не один (составные GIN почти не используют)
CREATE INDEX idx_products_category ON products(category);    -- B-Tree
CREATE INDEX idx_products_attributes ON products USING GIN (attributes); -- GIN

-- 8. ЧАСТИЧНЫЕ ИНДЕКСЫ ДЛЯ РАЗНЫХ ТИПОВ

-- 8.1. Частичный GIN (только для активных товаров с JSONB)
CREATE INDEX idx_products_active_attributes_gin ON products USING GIN (attributes) WHERE is_active = true;

-- 8.2. Частичный BRIN (только для доставленных поставок)
CREATE INDEX idx_shipments_delivered_brin ON shipments USING BRIN (shipment_date) WHERE status = 'delivered';

-- 9. ИНДЕКСЫ БЕЗ ИМЕНИ (PostgreSQL автоматически создаёт уникальный индекс для PRIMARY KEY и UNIQUE)
-- Для PRIMARY KEY и UNIQUE база создаёт B-Tree индекс автоматически
-- Например:
-- PRIMARY KEY (id) → уже есть B-Tree
-- UNIQUE (email) → уже есть B-Tree