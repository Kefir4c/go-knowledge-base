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


-- 3. INDEX ONLY SCAN (все данные берутся из индекса, таблица не читается)
-- 3.1. Создаём покрывающий индекс.
CREATE INDEX idx_suppliers_active_cover ON suppliers(is_active) INCLUDE (name,email);

EXPLAIN (ANALYZE, BUFFERS)
SELECT name, email FROM suppliers WHERE is_active = true;

/*
Index Only Scan using idx_suppliers_active_cover on suppliers  (cost=0.14..8.15 rows=100 width=...)
  Heap Fetches: 0
  Buffers: shared hit=4
Heap Fetches: 0 — таблица не читалась. Это идеал.
*/

-- 3.2. Если в SELECT есть колонки, не входящие в INCLUDE — будет Index Scan.
EXPLAIN (ANALYZE, BUFFERS)
SELECT name, email, phone FROM suppliers WHERE is_active = true;

/*
Index Scan — потому что phone не в индексе, пришлось идти в таблицу.
*/

-- 4. BITMAP SCAN (комбинация индекса и битовой карты)
-- 4.1. Запрос, возвращающий много строк — база может выбрать Bitmap Scan.
-- Создаём индекс на статус (если нет).
CREATE INDEX idx_shipments_status ON shipments(status);

EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM shipments WHERE status = 'delivered';
/*
Если статус 'delivered' у 30% строк, то база может использовать Bitmap Heap Scan:
Bitmap Index Scan -> Bitmap Heap Scan.
Потому что Index Scan для большого количества строк невыгоден (пришлось бы много раз ходить в таблицу).
Bitmap сначала собирает битовую карту, потом читает строки блоками — быстрее.
*/

-- 4.2. Составной индекс с диапазоном.
CREATE INDEX idx_shipments_status_date ON shipments(status, shipment_date)

EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM shipments
WHERE status = 'delivered' AND shipment_date > '2023-01-01';
/*
Может быть:
- Index Scan (если строк немного)
- Bitmap Heap Scan (если строк много)
Смотрим на actual rows и стоимость.
*/

-- 5. СОРТИРОВКА (SORT) И ЕЁ ВЛИЯНИЕ
-- 5.1. Запрос с ORDER BY без индекса.
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM shipments ORDER BY shipment_date DESC;

/*
Увидим Sort (cost=... rows=...) — база сортирует все строки.
Если таблица большая, это дорого.
*/

-- 5.2. Создаём индекс, который поддерживает сортировку.
CREATE INDEX idx_shipments_shipment_date_desc ON shipments(shipment_date DESC);

EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM shipments ORDER BY shipment_date DESC;

/*
Index Scan Backward — база идёт по индексу в обратном порядке, сортировка не нужна.
Стоимость падает.
*/

-- 6. JOIN И ТИПЫ СОЕДИНЕНИЙ

-- 6.1. JOIN двух таблиц без индексов на внешние ключи.
EXPLAIN(ANALYZE, BUFFERS)
SELECT p.name, s.quantity, s.shipment_date
FROM products p
JOIN shipments s on p.id = s.product_id
where p.is_active = true;
/*
Может быть Hash Join или Nested Loop, но без индексов будет Seq Scan + Hash.
*/

-- 6.2. Создаём индексы на внешние ключи.
CREATE INDEX idx_shipments_product_id ON shipments(product_id);
CREATE INDEX idx_shipments_warehouse_id ON shipments(warehouse_id);

EXPLAIN (ANALYZE, BUFFERS)
SELECT p.name, s.quantity, s.shipment_date
FROM products p
JOIN shipments s ON p.id = s.product_id
WHERE p.is_active = true;

/*
Теперь может быть Nested Loop с Index Scan по shipments.product_id.
Быстрее.
*/

-- 7. ПОДЗАПРОСЫ И CTE

-- 7.1. Коррелированный подзапрос (медленный).
EXPLAIN (ANALYZE, BUFFERS)
SELECT u.name,
       (SELECT COUNT(*) FROM shipments s WHERE s.supplier_id = u.id) AS shipment_count
FROM suppliers u
WHERE u.is_active = true;

/*
План: Seq Scan по suppliers, для каждой строки выполняется подзапрос.
Видим Subquery Scan и Aggregate — медленно, если много поставщиков.
*/

-- 7.2. Переписываем через JOIN и GROUP BY (быстрее).
EXPLAIN (ANALYZE, BUFFERS)
SELECT u.name, COUNT(s.id) AS shipment_count
FROM suppliers u
LEFT JOIN shipments s ON u.id = s.supplier_id
WHERE u.is_active = true
GROUP BY u.id, u.name;

/*
Группировка и JOIN часто быстрее коррелированного подзапроса.
План — Hash Join + GroupAggregate.
*/

-- 9. ИНДЕКС ПО ВЫРАЖЕНИЮ

-- 9.1. Регистронезависимый поиск.
CREATE INDEX idx_suppliers_email_lower ON suppliers(LOWER(email));

EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM suppliers WHERE LOWER(email) = 'roga@mail.ru';

/*
Index Scan using idx_suppliers_email_lower — работает быстро.
Если бы индекса не было, был бы Seq Scan.
*/

-- 10. ОГРОМНЫЙ OFFSET (ПАГИНАЦИЯ)

-- 10.1. Большой OFFSET заставляет сортировать и отбрасывать много строк.
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM shipments
ORDER BY shipment_date DESC
OFFSET 5000 LIMIT 10;

/*
Sort — сначала сортирует все строки, потом Limit.
Стоимость растёт с OFFSET.
*/

-- 10.2. Альтернатива — "keyset pagination" (поиск по последнему значению).
-- Предположим, мы знаем последнюю дату с предыдущей страницы.
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM shipments
WHERE shipment_date < '2023-06-01'
ORDER BY shipment_date DESC
LIMIT 10;

-- 11. СРАВНЕНИЕ ПЛАНОВ ДО И ПОСЛЕ ИНДЕКСА

-- 11.1. Запрос без индекса.
DROP INDEX IF EXISTS idx_products_category_price;

EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM products WHERE category = 'Electronics' AND price > 100;

-- 11.2. Создаём индекс.
CREATE INDEX idx_products_category_price ON products(category, price);

EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM products WHERE category = 'Electronics' AND price > 100;

/*
Видим Index Scan, стоимость падает.
*/

-- 12. КАК ПОНЯТЬ, ЧТО ИНДЕКС НЕ ИСПОЛЬЗУЕТСЯ (ДАЖЕ ЕСЛИ ОН ЕСТЬ)

-- 12.1. Условие с функцией без индекса по выражению.
-- Есть индекс на email, но запрос с UPPER.
CREATE INDEX idx_suppliers_email ON suppliers(email);

EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM suppliers WHERE UPPER(email) = 'ROGA@MAIL.RU';

/*
Seq Scan — потому что условие обёрнуто в функцию, а индекс на сырой email.
Решение: индекс по выражению (LOWER или UPPER).
*/

-- 12.2. LIKE с ведущим '%'.
EXPLAIN (ANALYZE, BUFFERS)
SELECT * FROM suppliers WHERE email LIKE '% main.ru'

/*
Seq Scan — потому что LIKE с ведущим '%' не использует B-Tree индекс.
Решение: полнотекстовый поиск или триграмный индекс (pg_trgm).
*/

-- 13. ВЫВОДЫ И ШПАРГАЛКА

/*
Seq Scan — плохо, если таблица большая.
Index Scan — хорошо, но читает таблицу.
Index Only Scan — отлично, всё из индекса.
Bitmap Heap Scan — хорошо для выборки большого количества строк.
Nested Loop — хорошо, если одна таблица маленькая, и есть индекс на большой.
Hash Join — хорошо, если обе таблицы большие, но нет подходящих индексов.
Merge Join — хорошо, если обе таблицы отсортированы.

ПРИЗНАКИ ПРОБЛЕМ:
- cost очень высокий
- rows сильно отличается от фактического
- много Buffers: read (дисковые чтения)
- Sort без индекса
- Seq Scan на большой таблице

ЧТО ДЕЛАТЬ:
- Создать индекс
- Сделать индекс покрывающим (INCLUDE)
- Сделать частичный индекс
- Сделать индекс по выражению
- Переписать запрос (убрать коррелированные подзапросы, заменить OFFSET на keyset)
*/