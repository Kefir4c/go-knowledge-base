/*
ШАГ 3.1: ПАРТИЦИОНИРОВАНИЕ

Этот блок полностью покрывает тему партиционирования в PostgreSQL.

ЧАСТЬ 1. ТЕОРИЯ

1.1. Что такое партиционирование?
  - Партиционирование — это разделение одной большой таблицы на множество меньших физических таблиц (партиций).
  - Пользователь работает с родительской таблицей, а PostgreSQL автоматически направляет запросы к нужным партициям.
  - Каждая партиция — это обычная таблица с ограничением CHECK, которое определяет, какие данные в неё попадают.

1.2. Типы партиционирования:
  - RANGE (по диапазону): 
    - Пример: лог-таблицы по датам (created_at).
    - Поддерживает операторы: <, <=, >, >=, BETWEEN, =.
    - Используется для временных рядов, архивов.
    - Поддерживает оператор = и IN.
  - LIST (по списку значений):
    - Пример: по регионам ('Europe', 'Asia').
    - Используется для категориальных данных.
  - HASH (по хешу):
    - Пример: по user_id, для равномерного распределения.
    - Поддерживает оператор = и IN.
    - Используется для балансировки нагрузки.
    - Модуль и остаток задаются при создании партиций.

1.3. Partition Pruning (отсечение партиций):
  - Это оптимизация планировщика, при которой он исключает партиции, которые не могут содержать данные, удовлетворяющие WHERE-условию.
  - Происходит на этапе планирования запроса.
  - Условия, при которых срабатывает Pruning:
    * Прямое сравнение с константой: WHERE created_at = '2026-01-01'
    * Диапазон: WHERE created_at BETWEEN '2026-01-01' AND '2026-01-31'
    * IN со списком констант: WHERE created_at IN ('2026-01-01', '2026-01-02')
    * IS NULL (если партиции для NULL определены)
  - Условия, при которых Pruning НЕ срабатывает:
    * Использование функций на колонке партиционирования: WHERE DATE(created_at) = '2026-01-01'
    * Неявное приведение типов: WHERE created_at = '2026-01-01'::text (если тип отличается)
    * OR с разными колонками: WHERE created_at = '2026-01-01' OR user_id = 1
    * Использование параметров, значения которых неизвестны на этапе планирования (подготовленные выражения)
    * Запросы, не содержащие условие на ключ партиционирования.

1.3.1 Partition Pruning — добавим нюанс про подготовленные выражения
Ты написал, что Pruning не срабатывает при использовании параметров, значения которых неизвестны на этапе планирования.
Это важный нюанс для Go-приложений, потому что мы часто используем подготовленные выражения (PREPARE).

Пример:
PREPARE get_orders(timestamptz) AS
SELECT * FROM orders WHERE created_at = $1;
На момент подготовки запроса значение $1 неизвестно, поэтому планировщик не может отсечь партиции.
Он выбирает план, который работает для всех возможных значений.

Решение:
Использовать EXECUTE с явным значением: EXECUTE get_orders('2026-01-01').
Или использовать простые запросы без подготовки, если это критично.
Или включить plan_cache_mode = force_custom_plan, чтобы планировщик пересматривал план при каждом выполнении.

Если я использую подготовленные выражения в приложении, планировщик может не видеть значения параметров на этапе
планирования, и Partition Pruning не сработает. Чтобы этого избежать, я использую обычные запросы с явными
значениями или управляю кешированием планов через plan_cache_mode при необходимости.

1.4. Управление партициями:
  - Добавление партиции: CREATE TABLE ... PARTITION OF ... FOR VALUES FROM ... TO ...
  - Отсоединение партиции (для архивации или модификации): ALTER TABLE ... DETACH PARTITION ...
  - Присоединение партиции: ALTER TABLE ... ATTACH PARTITION ...
  - Удаление партиции: DROP TABLE partition_name (быстрое освобождение места).
  - Изменение структуры партиции: ALTER TABLE ... (как с обычной таблицей).

1.5. Индексы на партиционированных таблицах:
  - Индекс на родительской таблице создаётся автоматически на всех существующих и будущих партициях.
  - Можно создавать индексы на отдельных партициях (для учета специфики данных).
  - Уникальный индекс и PRIMARY KEY должны включать ключ партиционирования.
  - Ограничение: нельзя создать уникальный индекс, если он не включает ключ партиционирования,
    потому что уникальность должна гарантироваться во всей таблице, а не в отдельной партиции.

1.5.1. Индексы — важный нюанс про создание индексов на отдельных партициях
Выше написано, что индекс на родительской таблице создаётся на всех партициях. Но есть фишка:
индексы создаются параллельно на всех партициях, и если таблица большая, это может
занять много времени и заблокировать операции записи (если не использовать CONCURRENTLY).

Ключевой нюанс:
Индекс на родительской таблице создаётся с CONCURRENTLY только если указать эту опцию.
Если партиций много, создание индекса может быть очень медленным.

Лучшая практика:
Создавать индексы на отдельных партициях (если они большие) с использованием CONCURRENTLY.
Индекс на родительской таблице использовать для новых партиций.

При создании индекса на партиционированной таблице используют CONCURRENTLY, чтобы не блокировать запись.
На больших партициях иногда создают индексы отдельно на каждой партиции, чтобы контролировать процесс.

1.6. FOREIGN KEY на партиционированных таблицах:
  - Можно создать FOREIGN KEY на родительскую таблицу, и он будет работать со всеми партициями.
  - Нельзя создать FOREIGN KEY, ссылающийся на отдельную партицию, потому что партиция может быть отсоединена.

1.7. Производительность:
  - Вставка: небольшие накладные расходы на выбор партиции, но обычно незаметно.
  - Выборка: значительное ускорение при фильтрации по ключу партиционирования (Partition Pruning).
  - DELETE: может быть очень быстрым, если удалять целую партицию (DROP TABLE).
  - UPDATE: если обновляется ключ партиционирования, строка перемещается в другую партицию (что эквивалентно DELETE + INSERT).
  - Запросы без фильтрации по ключу могут быть МЕДЛЕННЕЕ, чем на непартиционированной таблице, потому что нужно посетить все партиции и объединить результаты.

1.8. Когда использовать партиционирование:
  - Таблица > 10-50 млн строк.
  - Частые запросы с фильтрацией по ключу партиционирования.
  - Необходимость быстро удалять старые данные.
  - Возможность хранить разные партиции на разных носителях (SSD для свежих, HDD для архивных).
  - Параллельная загрузка данных (можно загружать разные партиции параллельно).

1.9. Когда НЕ использовать партиционирование:
  - Маленькие таблицы (< 1 млн строк) — оверхед больше пользы.
  - Редкие запросы с фильтрацией по ключу.
  - Частые UPDATE, меняющие ключ партиционирования.
  - Сложная архитектура с множеством внешних ключей.

1.10. Альтернативы партиционированию:
  - Обычные таблицы с индексами.
  - Использование табличных пространств для разделения данных.
  - Шардирование (горизонтальное масштабирование на разные серверы).

ФИШКИ
1: «Партиционирование и VACUUM — неожиданный эффект»
Суть: VACUUM на партиционированной таблице не проходит по всем партициям, если не указать конкретную партицию.
Он работает с родительской таблицей, но фактически вызывает VACUUM для каждой партиции отдельно.

VACUUM на партиционированной таблице выполняется для каждой партиции отдельно.
Чтобы ускорить процесс, я иногда запускаю VACUUM на отдельных партициях, особенно если они большие.

2: «Можно отсоединить партицию, чтобы изменить её структуру»
Суть: Если нужно изменить структуру партиции (добавить колонку, изменить тип),
её нельзя изменить, пока она является частью родительской таблицы.

Решение:
Отсоединить партицию: ALTER TABLE logs DETACH PARTITION logs_2026_01;
Изменить структуру: ALTER TABLE logs_2026_01 ADD COLUMN ...;
Присоединить обратно: ALTER TABLE logs ATTACH PARTITION logs_2026_01 FOR VALUES FROM ... TO ...;

Если нужно изменить структуру партиции, я отсоединяю её, модифицирую, а затем присоединяю обратно.
Это позволяет менять структуру без блокировки всей таблицы.

3: «Партиционирование и ROW EXCLUSIVE блокировки»
Суть: INSERT в партиционированную таблицу блокирует только целевую партицию, а не всю таблицу.
Это позволяет параллельно вставлять данные в разные партиции без конфликтов.

4: «Партиционирование и DETACH для моментальной архивации»
Суть: Вместо DELETE старых данных, можно использовать DETACH и переместить партицию в архивную таблицу.

Пример:
-- Отсоединить партицию
ALTER TABLE logs DETACH PARTITION logs_2025_01;
-- Создать архивную таблицу с такой же структурой
CREATE TABLE logs_archive (LIKE logs INCLUDING ALL);
-- Перенести данные (быстро, без переписывания)
ALTER TABLE logs_archive ATTACH PARTITION logs_2025_01 FOR VALUES FROM ... TO ...;

Для архивации старых данных я отсоединяю партицию и присоединяю её к архивной таблице.
Это быстрее и безопаснее, чем DELETE, и позволяет сохранить данные без потери.
*/
--ЧАСТЬ 2. ПРАКТИКА (БАЗОВЫЕ ПРИМЕРЫ)
-- 2.1. ПОДГОТОВКА ТАБЛИЦ
-- 2.1.1. Создаём партиционированную таблицу по дате (RANGE)
CREATE TABLE logs_partitioned (
    id BIGSERIAL,
    user_id INT NOT NULL,
    message TEXT NOT NULL,
    level TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
)PARTITION BY RANGE (created_at);

-- 2.1.2. Создаём партиции по месяцам
CREATE TABLE logs_2026_01 PARTITION OF logs_partitioned
    FOR VALUES FROM ('2026-01-01') TO ('2026-02-01');

CREATE TABLE logs_2026_02 PARTITION OF logs_partitioned
    FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

CREATE TABLE logs_2026_03 PARTITION OF logs_partitioned
    FOR VALUES FROM ('2026-03-01') TO ('2026-04-01');

-- 2.1.3. Индексы на каждой партиции (можно создать на родительской)
CREATE INDEX idx_logs_2026_01_user_id ON logs_2026_01(user_id);
CREATE INDEX idx_logs_2026_01_created_at ON logs_2026_01(created_at);

CREATE INDEX idx_logs_2026_02_user_id ON logs_2026_02(user_id);
CREATE INDEX idx_logs_2026_02_created_at ON logs_2026_02(created_at);

CREATE INDEX idx_logs_2026_03_user_id ON logs_2026_03(user_id);
CREATE INDEX idx_logs_2026_03_created_at ON logs_2026_03(created_at);

-- 2.1.4. Обычная таблица для сравнения
CREATE TABLE logs_unpartitioned (
    id BIGSERIAL,
    user_id INT NOT NULL,
    message TEXT NOT NULL,
    level TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_logs_unpartitioned_created_at ON logs_unpartitioned (created_at);
CREATE INDEX idx_logs_unpartitioned_user_id ON logs_unpartitioned (user_id);

-- 2.2. ВСТАВКА ДАННЫХ
-- 2.2.1. Тестовые данные (явные)
INSERT INTO logs_partitioned (user_id, message, level, created_at) VALUES
    (1, 'User logged in', 'INFO', '2026-01-15 10:00:00'),
    (2, 'Payment processed', 'INFO', '2026-02-20 14:30:00'),
    (3, 'Database error', 'ERROR', '2026-03-05 09:15:00'),
    (4, 'User logged out', 'INFO', '2026-01-25 18:45:00'),
    (5, 'File uploaded', 'DEBUG', '2026-02-10 12:00:00'),
    (1, 'Profile updated', 'INFO', '2026-03-15 11:20:00');

INSERT INTO logs_unpartitioned (user_id, message, level, created_at) VALUES
    (1, 'User logged in', 'INFO', '2026-01-15 10:00:00'),
    (2, 'Payment processed', 'INFO', '2026-02-20 14:30:00'),
    (3, 'Database error', 'ERROR', '2026-03-05 09:15:00'),
    (4, 'User logged out', 'INFO', '2026-01-25 18:45:00'),
    (5, 'File uploaded', 'DEBUG', '2026-02-10 12:00:00'),
    (1, 'Profile updated', 'INFO', '2026-03-15 11:20:00');

-- 2.2.2. Большие объёмы (по 100 000 записей) для тестов производительности
INSERT INTO logs_partitioned (user_id, message, level, created_at)
SELECT
    (random() * 1000)::INT,
    'log_' || generate_series,
    CASE WHEN random() > 0.8 THEN 'ERROR' ELSE 'INFO' END,
    NOW() - (random() * 365 || ' days')::interval
FROM generate_series(1, 100000);

INSERT INTO logs_unpartitioned (user_id, message, level, created_at)
SELECT
    (random() * 1000)::INT,
    'log_' || generate_series,
    CASE WHEN random() > 0.8 THEN 'ERROR' ELSE 'INFO' END,
    NOW() - (random() * 365 || ' days')::interval
FROM generate_series(1, 100000);

-- 2.3. PARTITION PRUNING
-- 2.3.1. Запрос с фильтром по created_at (используется только партиция за январь)
EXPLAIN(ANALYZE, BUFFERS)
SELECT COUNT(*) FROM logs_partitioned
WHERE created_at BETWEEN '2026-01-01' AND '2026-01-31';
-- В плане будет видно: Seq Scan on logs_2026_01 (или Index Scan), 
--но не будет упоминания других партиций.

-- 2.3.2. Запрос без фильтра по ключу (все партиции)
EXPLAIN (ANALYZE, BUFFERS)
SELECT COUNT(*) FROM logs_partitioned
WHERE user_id = 42;
-- Будет Append (или Gather) по всем партициям.

-- 2.3.3. Сравнение с непартиционированной таблицей
EXPLAIN (ANALYZE, BUFFERS)
SELECT COUNT(*) FROM logs_unpartitioned
WHERE created_at BETWEEN '2026-01-01' AND '2026-01-31';
-- Будет Index Scan или Seq Scan по всей таблице.

-- 2.4. LIST PARTITIONING
CREATE TABLE logs_by_level (
    id BIGSERIAL,
    user_id INT NOT NULL,
    message TEXT NOT NULL,
    level TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
) PARTITION BY LIST (level);

CREATE TABLE logs_info PARTITION OF logs_by_level FOR VALUES IN ('INFO');
CREATE TABLE logs_debug PARTITION OF logs_by_level FOR VALUES IN ('DEBUG');
CREATE TABLE logs_warning PARTITION OF logs_by_level FOR VALUES IN ('WARNING');
CREATE TABLE logs_error PARTITION OF logs_by_level FOR VALUES IN ('ERROR');

INSERT INTO logs_by_level (user_id, message, level) VALUES
    (1, 'User logged in', 'INFO'),
    (2, 'Payment processed', 'INFO'),
    (3, 'Database error', 'ERROR'),
    (4, 'User logged out', 'INFO'),
    (5, 'File uploaded', 'DEBUG');

-- Проверка: SELECT tableoid::regclass, * FROM logs_by_level;

-- 2.4.1. Запрос с фильтром по level (используется только партиция ERROR)
EXPLAIN ANALYZE SELECT * FROM logs_by_level WHERE level = 'ERROR';

-- 2.5. HASH PARTITIONING

CREATE TABLE logs_by_user_hash (
    id BIGSERIAL,
    user_id INT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
) PARTITION BY HASH (user_id);

CREATE TABLE logs_hash_0 PARTITION OF logs_by_user_hash
    FOR VALUES WITH (MODULUS 4, REMAINDER 0);
CREATE TABLE logs_hash_1 PARTITION OF logs_by_user_hash
    FOR VALUES WITH (MODULUS 4, REMAINDER 1);
CREATE TABLE logs_hash_2 PARTITION OF logs_by_user_hash
    FOR VALUES WITH (MODULUS 4, REMAINDER 2);
CREATE TABLE logs_hash_3 PARTITION OF logs_by_user_hash
    FOR VALUES WITH (MODULUS 4, REMAINDER 3);

INSERT INTO logs_by_user_hash (user_id, message) VALUES
    (1, 'User 1'), (2, 'User 2'), (3, 'User 3'), (4, 'User 4'),
    (5, 'User 5'), (6, 'User 6'), (7, 'User 7'), (8, 'User 8');

-- Проверка распределения
SELECT tableoid::regclass, user_id FROM logs_by_user_hash ORDER BY user_id;

-- 2.5.1. Запрос с фильтром по user_id (используется одна партиция)
EXPLAIN ANALYZE SELECT * FROM logs_by_user_hash WHERE user_id = 3;

-- 2.6. УПРАВЛЕНИЕ ПАРТИЦИЯМИ
-- 2.6.1. Добавление новой партиции
CREATE TABLE logs_2026_04 PARTITION OF logs_partitioned
  FOR VALUES FROM ('2026-04-01') TO ('2026-05-01');
CREATE INDEX idx_logs_2026_04_user_id ON logs_2026_04(user_id);

-- 2.6.2. Удаление старой партиции (быстрое освобождение места)
DROP TABLE logs_2026_01;

-- 2.6.3. Отсоединение партиции (архивация)
ALTER TABLE logs_partitioned DETACH PARTITION logs_2026_02;

-- 2.6.4. Присоединение партиции обратно
ALTER TABLE logs_partitioned ATTACH PARTITION logs_2026_02
  FOR VALUES FROM ('2026-02-01') TO ('2026-03-01');

-- 2.7. ИНДЕКСЫ И ОГРАНИЧЕНИЯ  

-- 2.7.1. Индекс на родительской таблице (создаётся на всех партициях)
CREATE INDEX idx_logs_partitioned_user_id ON logs_partitioned (user_id);

-- 2.7.2. Уникальный индекс (должен включать ключ партиционирования)
CREATE UNIQUE INDEX idx_logs_partatiined_id ON logs_partitoned

-- 2.7.3. PRIMARY KEY
ALTER TABLE logs_partitioned ADD PRIMARY KEY (id, created_id);

-- 2.7.4. FOREIGN KEY (можно ссылаться на партиционированную таблицу)
CREATE TABLE logs_errors(
    log_id BIGINT REFERENCES logs_partitioned(id, created_id)
);

-- 2.8. МОНИТОРИНГ ПАРТИЦИЙ
-- 2.8.1. Список партиций и их размеры
SELECT
    schemaname,
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS total_size
FROM pg_tables
WHERE tablename LIKE 'logs_%'
ORDER BY tablename;

-- 2.8.2. Количество строк в каждой партиции
SELECT
    relname AS partition_name,
    n_live_tup AS rows
FROM pg_stat_user_tables
WHERE relname LIKE 'logs_%'
ORDER BY relname;

-- 2.8.3. Границы партиций
SELECT
    child.relname AS partition_name,
    pg_get_expr(child.relpartbound, child.oid) AS bound
FROM pg_inherits
JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
JOIN pg_class child ON pg_inherits.inhrelid = child.oid
WHERE parent.relname = 'logs_partitioned';

-- 2.9. АВТОМАТИЗАЦИЯ (ФУНКЦИИ)
-- 2.9.1. Создание партиции на следующий месяц
CREATE OR REPLACE FUNCTION create_monthly_partition()
RETURNS VOID AS $$
DECLARE
    next_month TEXT;
    next_month_start DATE;
    next_month_end DATE;
BEGIN
    next_month_start := DATE_TRUNC('month', NOW() + INTERVAL '1 month')::DATE;
    next_month_end := DATE_TRUNC('month', NOW() + INTERVAL '2 months')::DATE;
    next_month := TO_CHAR(next_month_start, 'YYYY_MM');

    IF NOT EXISTS (
        SELECT 1 FROM pg_class
        WHERE relname = 'logs_' || next_month
        AND relkind = 'r'
    ) THEN
        EXECUTE format('
            CREATE TABLE logs_%s PARTITION OF logs_partitioned
            FOR VALUES FROM (%L) TO (%L)
        ', next_month, next_month_start, next_month_end);

        EXECUTE format('
            CREATE INDEX idx_logs_%s_user_id ON logs_%s (user_id)
        ', next_month, next_month);

        RAISE NOTICE 'Created partition logs_%', next_month;
    END IF;
END;
$$ LANGUAGE plpgsql;

SELECT create_monthly_partition();

-- 2.9.2. Удаление партиций старше N месяцев
CREATE OR REPLACE FUNCTION drop_old_partitions(
    parent_table TEXT,
    months_to_keep INT DEFAULT 3
) RETURNS VOID AS $$
DECLARE
    partition_record RECORD;
    cutoff_date DATE;
    upper_bound DATE;
BEGIN
    cutoff_date := DATE_TRUNC('month', NOW() - (months_to_keep || ' months')::interval)::DATE;

    FOR partition_record IN
        SELECT
            child.relname AS partition_name,
            pg_get_expr(child.relpartbound, child.oid) AS bound_expr
        FROM pg_inherits
        JOIN pg_class parent ON pg_inherits.inhparent = parent.oid
        JOIN pg_class child ON pg_inherits.inhrelid = child.oid
        WHERE parent.relname = parent_table
    LOOP
        upper_bound := (regexp_matches(partition_record.bound_expr, 'TO \(''([^'']+)''\)'))[1]::DATE;

        IF upper_bound < cutoff_date THEN
            EXECUTE format('DROP TABLE %I', partition_record.partition_name);
            RAISE NOTICE 'Dropped partition % (upper_bound: %)', partition_record.partition_name, upper_bound;
        END IF;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

SELECT drop_old_partitions('logs_partitioned', 3);

-- 2.10. СРАВНИТЕЛЬНЫЙ АНАЛИЗ ПРОИЗВОДИТЕЛЬНОСТИ

-- 2.10.1. Запрос с отсечением (партиционированная)
EXPLAIN (ANALYZE, BUFFERS) SELECT COUNT(*) FROM logs_partitioned
WHERE created_at BETWEEN '2026-01-01' AND '2026-01-31';

-- 2.10.2. Запрос без отсечения (партиционированная)
EXPLAIN (ANALYZE, BUFFERS) SELECT COUNT(*) FROM logs_partitioned
WHERE user_id = 42;

-- 2.10.3. Запрос на непартиционированной таблице
EXPLAIN (ANALYZE, BUFFERS) SELECT COUNT(*) FROM logs_unpartitioned
WHERE created_at BETWEEN '2026-01-01' AND '2026-01-31';

-- 2.10.4. DELETE с фильтром по дате (партиционированная)
EXPLAIN (ANALYZE, BUFFERS) DELETE FROM logs_partitioned
WHERE created_at < '2026-01-01';

-- 2.10.5. DELETE с тем же условием на непартиционированной
EXPLAIN (ANALYZE, BUFFERS) DELETE FROM logs_unpartitioned
WHERE created_at < '2026-01-01';

-- 2.11. ИСПОЛЬЗОВАНИЕ pg_partman (расширение)

-- Установка расширения (требуется суперпользователь)
-- CREATE EXTENSION IF NOT EXISTS pg_partman;

-- Настройка автоматического создания партиций
-- SELECT partman.create_parent(
--     p_parent_table := 'public.logs_partitioned',
--     p_control := 'created_at',
--     p_type := 'range',
--     p_interval := 'monthly'
-- );

-- Настройка автоматического удаления старых партиций
-- UPDATE partman.part_config
-- SET retention = '3 months', retention_keep_table = true
-- WHERE parent_table = 'public.logs_partitioned';

-- 2.12. ПРИМЕРЫ (СЛОЖНЫЕ СЦЕНАРИИ)

-- 2.12.1. Составное партиционирование (RANGE по дате + LIST по региону)
-- В PostgreSQL нет встроенного составного, но можно комбинировать: сначала RANGE, затем LIST на подпартициях.
-- Создаём родительскую таблицу

CREATE TABLE logs_advanced (
    id BIGSERIAL,
    user_id INT NOT NULL,
    region TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
) PARTITION BY RANGE (created_at);

CREATE TABLE logs_adv_2026_01 PARTITION OF logs_advanced
  FOR VALUE FROM ('2026-01-01') TO ('2026-02-01')
  PARTITION BY LIST (region);

-- Создаём подпартиции по регионам для января
CREATE TABLE logs_adv_2026_01_europe PARTITION OF logs_adv_2026_01
    FOR VALUES IN ('Europe');
CREATE TABLE logs_adv_2026_01_asia PARTITION OF logs_adv_2026_01
    FOR VALUES IN ('Asia');
-- и т.д.

-- 2.12.2. Использование партиционирования с оконными функциями
SELECT
  user_id,
  created_at,
  ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at) AS log_order
FROM logs_partitioned
WHERE created_at >= '2026-01-01'
ORDER BY user_id, created_at;

-- 2.12.3. Запрос с объединением партиций (UNION ALL)
SELECT * FROM logs_2026_01
UNION ALL
SELECT * FROM logs_2026_02;