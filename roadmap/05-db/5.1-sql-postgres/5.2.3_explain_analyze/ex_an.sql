/*
ШАГ 2.3: ДИАГНОСТИКА (EXPLAIN ANALYZE)

1. ЧТО ТАКОЕ EXPLAIN?
   - EXPLAIN показывает план выполнения запроса, который построил оптимизатор.
   - Это не фактическое выполнение, а прогноз (стоимость, количество строк).
   - EXPLAIN ANALYZE — реально выполняет запрос и показывает фактические цифры.

2. ЗАЧЕМ ЭТО НУЖНО?
   - Чтобы понять, используется ли индекс, или идёт полный перебор (Seq Scan).
   - Чтобы увидеть, где запрос тратит больше всего времени.
   - Чтобы сравнить разные варианты запросов и выбрать оптимальный.

3. ОСНОВНЫЕ ТИПЫ СКАНОВ (ЧТО ИЩЕМ В ПЛАНЕ):
   - Seq Scan (Sequential Scan) — полное сканирование таблицы. Медленно на больших таблицах.
   - Index Scan — чтение строки по индексу, потом дополнительное чтение из таблицы (heap).
   - Index Only Scan — все нужные колонки есть в индексе, таблица не читается. Быстро! INCLUDE()
   - Bitmap Heap Scan — комбинация индекса и битовой карты (используется при выборке большого числа строк).
   - Nested Loop, Hash Join, Merge Join — способы соединения таблиц.

4. ЧТО ОЗНАЧАЮТ ЦИФРЫ В ПЛАНЕ (cost, rows, width):
   - cost=0.00..1.01 — оценочная стоимость. Первое число (0.00) — стоимость начала, второе (1.01) — стоимость завершения. Чем меньше, тем быстрее.
   - rows=10 — прогнозируемое количество строк, которые вернёт операция.
   - width=32 — средний размер строки в байтах.
   - actual time=0.123..0.456 — реальное время выполнения (в миллисекундах). Появляется только в ANALYZE.
   - loops=1 — сколько раз выполнялся этот узел плана (обычно 1, но может быть больше при вложенных циклах).
   - Buffers: shared hit=100 read=50 — сколько страниц взято из кэша (hit) и сколько из диска (read). Чем меньше read, тем лучше.

5. КАК ИСПОЛЬЗОВАТЬ EXPLAIN В GO:
   - Ты можешь выполнить "EXPLAIN (ANALYZE, BUFFERS, FORMAT JSON) SELECT ..." и получить структурированный вывод в Go, который можно парсить.
   - Но на собеседовании достаточно уметь читать текстовый вывод.
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

-- 1. SEQ SCAN (полное сканирование таблицы)

-- 1.1. Запрос без индекса на колонку, по которой идёт фильтр.
-- Убедимся, что на suppliers.email нет индекса (если есть — удалим для теста).
EXPLAIN (ANALYZE, BUFFERS )
SELECT * FROM suppliers WHERE email = 'roga@mail.ru';
/*
Ожидаем:
Seq Scan on suppliers  (cost=0.00..12.00 rows=1 width=...)
  Filter: (email = 'roga@mail.ru')
  Buffers: shared read=...
Если таблица большая (100k+), стоимость будет высокая.
*/

-- 1.2. Запрос без индекса с LIKE (при поиске по началу строки) — тоже Seq Scan.
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM suppliers WHERE name LIKE 'ООО%';
--Seq Scan — потому что нет индекса на name.

-- 2. INDEX SCAN (поиск по индексу, потом чтение таблицы)
-- 2.1. Создаём индекс и выполняем тот же запрос.
CREATE INDEX idx_suppliers_email ON suppliers(email);
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM suppliers WHERE email = 'roga@mail.ru';
/*
Index Scan using idx_suppliers_email on suppliers  (cost=0.28..8.29 rows=1 width=...)
  Index Cond: (email = 'roga@mail.ru')
  Buffers: shared hit=3
Видим Index Scan. Стоимость упала, время сократилось.
*/

-- 2.2. Запрос с диапазоном — тоже Index Scan.
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM shipments
WHERE shipment_date BETWEEN '2023-01-01' AND '2023-12-31';
-- Если есть индекс на shipment_date — будет Index Scan.
-- Если нет — Seq Scan.