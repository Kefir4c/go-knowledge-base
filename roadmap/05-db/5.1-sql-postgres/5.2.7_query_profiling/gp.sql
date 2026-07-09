/*
ШАГ 2.7: ПРОФИЛИРОВАНИЕ ЗАПРОСОВ

1. ЗАЧЕМ НУЖНО ПРОФИЛИРОВАНИЕ?
   - В продакшене база данных работает с реальной нагрузкой.
   - Запросы, которые были быстры на тестовых данных, могут тормозить на больших объёмах.
   - Нужно уметь находить проблемные запросы без остановки сервиса.
   - Профилирование помогает выявить:
     * Медленные запросы (по времени выполнения)
     * Часто выполняющиеся запросы (нагрузка на БД)
     * Запросы с большим I/O (чтение с диска)
     * Запросы, использующие временные файлы (сортировки, хеши)
     * Запросы с неоптимальными планами выполнения

2. ДВА ОСНОВНЫХ ИНСТРУМЕНТА:
   - log_min_duration_statement: логирует запросы, которые выполняются дольше заданного времени.
   - pg_stat_statements: собирает детальную статистику по всем запросам (или по выбранным).

3. КАК РАБОТАЕТ log_min_duration_statement
   - Параметр в postgresql.conf или устанавливается через SET.
   - Значение в миллисекундах. Например, 1000 означает логировать запросы дольше 1 секунды.
   - Если установить 0, логируются все запросы.
   - Логи пишутся в stderr (обычно в файл лога PostgreSQL).
   - Полезно для поиска запросов-кандидатов на оптимизацию.
   - Недостаток: логи содержат только сам запрос и время, без агрегированной статистики.

4. КАК РАБОТАЕТ pg_stat_statements
   - Расширение, которое собирает статистику выполнения запросов.
   - Должно быть добавлено в shared_preload_libraries.
   - Собирает:
     * queryid (идентификатор запроса, одинаковый для одинаковых запросов с разными параметрами)
     * calls (количество вызовов)
     * total_exec_time (общее время выполнения в миллисекундах)
     * mean_exec_time (среднее время)
     * min_exec_time, max_exec_time
     * rows (количество затронутых строк)
     * shared_blks_hit (чтение из кэша)
     * shared_blks_read (чтение с диска)
     * temp_blks_read/written (использование временных файлов)
   - Статистика накапливается с момента последнего сброса.
   - Можно сбросить через pg_stat_statements_reset().
   - Позволяет найти запросы по различным критериям: самые медленные, самые частые, с большим I/O и т.д.

5. КАК ИНТЕРПРЕТИРОВАТЬ СТАТИСТИКУ
   - total_exec_time > 10000 ms (10 сек) — явный кандидат на оптимизацию.
   - mean_exec_time > 1000 ms и calls > 1000 — запрос тормозит и выполняется часто.
   - shared_blks_read >> shared_blks_hit — много чтений с диска, возможно, нужен индекс.
   - temp_blks_read/written > 0 — запрос использует временные файлы (сортировка, хеш-джойн), возможно, не хватает work_mem.
   - rows / calls — много строк на вызов, возможно, нужен LIMIT или фильтрация.
   - stddev_exec_time высокий — время выполнения нестабильно, возможно, зависит от данных.

6. НАСТРОЙКА ДЛЯ ПРОДАКШЕНА (рекомендации)
   - Включить log_min_duration_statement = 500 или 1000, чтобы видеть медленные запросы.
   - Включить pg_stat_statements и настроить:
     * pg_stat_statements.max = 5000 (максимальное число записей)
     * pg_stat_statements.track = all (отслеживать все запросы)
     * pg_stat_statements.track_utility = on (отслеживать DDL, VACUUM и т.д.)
   - Периодически сбрасывать статистику и анализировать изменения.
   - Использовать внешние инструменты: pgbadger (для логов), Grafana + Prometheus (для мониторинга).

7. СВЯЗЬ С GO
   - В Go-сервисе можно логировать медленные запросы с помощью middleware или обёртки над драйвером.
   - Пример: измерять время выполнения каждого запроса и логировать, если > threshold.
   - Использовать контекст с таймаутом, чтобы прерывать долгие запросы.
   - Настроить пул соединений (pgxpool) с параметрами таймаута.
   - Логировать queryid из pg_stat_statements для возможности корреляции с логами приложения.
*/

-- 0. ПОДГОТОВКА ТЕСТОВЫХ ДАННЫХ

-- Создаём таблицу для тестов
DROP TABLE IF EXISTS test_queries CASCADE;
CREATE TABLE test_queries (
    id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id INT,
    amount NUMERIC(10,2),
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Заполняем 100 000 случайных записей
INSERT INTO test_queries (user_id, amount)
SELECT
    (random() * 1000)::INT,
    (random() * 10000)::NUMERIC(10,2)
FROM generate_series(1, 100000);

-- Создаём индексы для ускорения (если нужно)
CREATE INDEX idx_test_queries_user_id ON test_queries(user_id);
CREATE INDEX idx_test_queries_created_at ON test_queries(created_at);

-- Выполняем несколько запросов для накопления статистики
SELECT * FROM test_queries WHERE user_id = 42;
SELECT * FROM test_queries WHERE created_at > NOW() - INTERVAL '1 day';
SELECT user_id, SUM(amount) FROM test_queries GROUP BY user_id;
SELECT COUNT(*) FROM test_queries;

-- 1. НАСТРОЙКА ЛОГИРОВАНИЯ МЕДЛЕННЫХ ЗАПРОСОВ (log_min_duration_statement)

-- 1.1. Проверяем текущие настройки
SHOW log_min_duration_statement;
SHOW log_statement;

-- 1.2. Включаем логирование запросов, выполняющихся дольше 1 секунды
-- (можно изменить на 500 мс, 100 мс и т.д.)
SET log_min_duration_statement = 1000

-- 1.3. Для постоянной настройки нужно править postgresql.conf:
-- log_min_duration_statement = 1000
-- log_statement = 'all'  (если нужно логировать все запросы)

-- 1.4. Проверяем, что настройка применилась
SHOW log_min_duration_statement;

-- 2. РАСШИРЕНИЕ pg_stat_statements
-- 2.1. Проверяем, установлено ли расширение
SELECT * FROM pg_available_extensions WHERE name = 'pg_stat_statements'

-- 2.2. Устанавливаем расширение (требуется права суперпользователя)
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- 2.3. Проверяем, что расширение активно
SELECT * FROM pg_extension WHERE extname = 'pg_stat_statements';

-- 2.4. Настройка параметров для сбора статистики (postgresql.conf)
-- shared_preload_libraries = 'pg_stat_statements'
-- pg_stat_statements.max = 5000
-- pg_stat_statements.track = all

-- После изменения нужно перезапустить PostgreSQL

-- 3. АНАЛИЗ СТАТИСТИКИ ЗАПРОСОВ

-- 3.1. Самые частые запросы (по количеству вызовов)
SELECT
    queryid,
    query,
    calls,
    total_exec_time,
    mean_exec_time,
    stddev_exec_time,
    rows,
    shared_blks_hit,
    shared_blks_read
FROM pg_stat_statements
ORDER BY calls DESC
LIMIT 10;

-- 3.2. Самые медленные запросы (по общему времени выполнения)
SELECT
    queryid,
    query,
    calls,
    total_exec_time / 1000 AS total_seconds,
    mean_exec_time,
    min_exec_time,
    max_exec_time
FROM pg_stat_statements
ORDER BY total_exec_time DESC
LIMIT 10;

-- 3.3. Самые медленные по среднему времени выполнения
SELECT
    queryid,
    query,
    calls,
    mean_exec_time,
    total_exec_time,
    min_exec_time,
    max_exec_time
FROM pg_stat_statements
ORDER BY mean_exec_time DESC
LIMIT 10;

-- 3.4. Запросы с наибольшим количеством чтений с диска
SELECT
    queryid,
    query,
    calls,
    shared_blks_read,
    shared_blks_read / nullif(calls, 0) AS blks_read_per_call,
    total_exec_time
FROM pg_stat_statements
ORDER BY shared_blks_read DESC
LIMIT 10;

-- 3.5. Запросы с большим количеством затронутых строк
SELECT
    queryid,
    query,
    calls,
    rows,
    COALESCE(rows / nullif(calls, 0),0) AS rows_per_call,
    total_exec_time
FROM pg_stat_statements
ORDER BY rows DESC
LIMIT 10;

-- 3.6. Запросы с высоким стандартным отклонением времени выполнения
SELECT
    queryid,
    query,
    calls,
    mean_exec_time,
    stddev_exec_time,
    max_exec_time
FROM pg_stat_statements
WHERE stddev_exec_time > mean_exec_time * 0.5
ORDER BY stddev_exec_time DESC
LIMIT 10;

-- 3.7. Комбинированный анализ: медленные и частые
SELECT
    queryid,
    query,
    calls,
    mean_exec_time,
    total_exec_time,
    (total_exec_time / calls) AS avg_time_per_call
FROM pg_stat_statements
WHERE calls > 1000
ORDER BY total_exec_time DESC
LIMIT 10;

-- 4. ФИЛЬТРАЦИЯ ПО БАЗЕ ДАННЫХ И ПОЛЬЗОВАТЕЛЮ

-- 4.1. Только запросы к текущей БД
SELECT query, calls, total_exec_time
FROM pg_stat_statements
WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
ORDER BY total_exec_time DESC
LIMIT 10;

-- 4.2. Запросы конкретного пользователя
SELECT query, calls, total_exec_time
FROM pg_stat_statements
WHERE userid = (SELECT usesysid FROM pg_user WHERE usename = 'postgres')
ORDER BY total_exec_time DESC
LIMIT 10;

-- 5. СБРОС СТАТИСТИКИ

-- 5.1. Сброс всей статистики
SELECT pg_stat_statements_reset();

-- 5.2. Сброс статистики для конкретного запроса (по queryid)
SELECT pg_stat_statements_reset(queryid);

-- 6. ПОИСК КОНКРЕТНЫХ ПРОБЛЕМНЫХ ЗАПРОСОВ
-- 6.1. Запросы, которые выполняются часто, но медленно (avg > 100ms, calls > 1000)
SELECT
    queryid,
    query,
    calls,
    mean_exec_time,
    total_exec_time
FROM pg_stat_statements
WHERE mean_exec_time > 100
  AND calls > 1000
ORDER BY total_exec_time DESC
LIMIT 10;

-- 6.2. Запросы с большим количеством чтений из кэша (shared_blks_hit)
-- Это может указывать на неоптимальные индексы
SELECT
    queryid,
    query,
    calls,
    shared_blks_hit / nullif(calls, 0) AS hit_per_call,
    shared_blks_read,
    total_exec_time
FROM pg_stat_statements
WHERE shared_blks_hit > 0
ORDER BY hit_per_call DESC
LIMIT 10;

-- 6.3. Запросы с высокими затратами ввода-вывода (I/O)
SELECT
    queryid,
    query,
    calls,
    shared_blks_read,
    shared_blks_dirtied,
    shared_blks_written,
    total_exec_time
FROM pg_stat_statements
ORDER BY shared_blks_read + shared_blks_written DESC
LIMIT 10;

-- 6.4. Запросы с медленным ответом из-за временных файлов (temp_blks)
SELECT
    queryid,
    query,
    calls,
    temp_blks_read,
    temp_blks_written,
    total_exec_time
FROM pg_stat_statements
ORDER BY temp_blks_read + temp_blks_written DESC
LIMIT 10;

-- 7. СОЗДАНИЕ ПРЕДСТАВЛЕНИЯ ДЛЯ МОНИТОРИНГА
-- 7.1. Создаём удобное представление для мониторинга
CREATE OR REPLACE VIEW query_stats AS
SELECT
    queryid,
    query,
    calls,
    total_exec_time,
    mean_exec_time,
    min_exec_time,
    max_exec_time,
    shared_blks_hit,
    shared_blks_read,
    temp_blks_read,
    temp_blks_written,
    rows,
    (total_exec_time / nullif(calls, 0)) AS avg_time_per_call
FROM pg_stat_statements
WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database());

-- 7.2. Использование представления
SELECT * FROM query_stats ORDER BY total_exec_time DESC LIMIT 10;
SELECT * FROM query_stats WHERE calls > 1000 ORDER BY avg_time_per_call DESC LIMIT 10;

-- 8. ИНТЕГРАЦИЯ С ЛОГАМИ (log_min_duration_statement)

-- 8.1. Логирование запросов, выполняющихся дольше 100 мс
SET log_min_duration_statement = 100;

-- 8.2. Логирование запросов, возвращающих много строк
SET log_statement = 'all';  -- логировать все
-- или включить параметр log_min_duration_statement и анализировать логи

-- 8.3. Анализ логов с помощью pgBadger (внешний инструмент)
-- pgbadger -f stderr -o report.html /var/log/postgresql/postgresql.log

-- 9. ПРИМЕРЫ ВЫЯВЛЕНИЯ ПРОБЛЕМНЫХ ЗАПРОСОВ
-- 9.1. Найти запрос с большим total_exec_time, но с малым calls
SELECT query, calls, total_exec_time, mean_exec_time
FROM query_stats
WHERE calls < 100 AND total_exec_time > 10000
ORDER BY total_exec_time DESC;

-- 9.2. Найти запрос, который медленно выполняется из-за full table scan
SELECT query, calls, mean_exec_time, shared_blks_read
FROM query_stats
WHERE query ILIKE '%SELECT%' AND query ILIKE '%FROM%' AND shared_blks_read > 10000
ORDER BY shared_blks_read DESC;

-- 9.3. Найти запрос с большим количеством временных файлов
SELECT query, calls, temp_blks_read, temp_blks_written
FROM query_stats
WHERE temp_blks_read + temp_blks_written > 0
ORDER BY temp_blks_read + temp_blks_written DESC;

-- 10. АНАЛИЗ ЗАПРОСОВ ПО ВРЕМЕНИ СУТОК

-- (Требуется отдельная таблица для истории статистики)

-- 10.1. Создаём таблицу для хранения срезов статистики
CREATE TABLE IF NOT EXISTS stat_history (
    snapshot_time TIMESTAMPTZ DEFAULT NOW(),
    queryid BIGINT,
    query TEXT,
    calls BIGINT,
    total_exec_time DOUBLE PRECISION,
    mean_exec_time DOUBLE PRECISION,
    rows BIGINT,
    shared_blks_read BIGINT
);

-- 10.2. Заполняем историю (запускать периодически)
INSERT INTO stat_history (queryid, query, calls, total_exec_time, mean_exec_time, rows, shared_blks_read)
SELECT
    queryid,
    query,
    calls,
    total_exec_time,
    mean_exec_time,
    rows,
    shared_blks_read
FROM query_stats;

-- 10.3. Анализ изменений (например, рост среднего времени)
SELECT
    query,
    calls,
    mean_exec_time,
    shared_blks_read
FROM stat_history
ORDER BY snapshot_time DESC
LIMIT 10;

-- 11. НАСТРОЙКА ДЛЯ ПРОДАКШЕНА


-- log_min_duration_statement = 500  (или 1000, в зависимости от требований)
-- pg_stat_statements.max = 5000
-- pg_stat_statements.track = all
-- pg_stat_statements.track_utility = on
-- Перед анализом, если нужно, сбросить статистику:
-- SELECT pg_stat_statements_reset();
-- Использовать pgbadger для визуализации логов
-- Для Go-сервиса: логировать медленные запросы отдельно через middleware
-- В Go можно установить таймауты на запросы к БД и логировать их
