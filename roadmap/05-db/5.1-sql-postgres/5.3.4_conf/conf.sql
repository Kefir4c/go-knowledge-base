/*
ШАГ 3.4: ТЮНИНГ КОНФИГУРАЦИИ POSTGRESQL (postgresql.conf)
ЧАСТЬ 1. ТЕОРИЯ (ПОЛНАЯ)

1.1. ЗАЧЕМ НУЖЕН ТЮНИНГ?

PostgreSQL по умолчанию настроен на минимальное потребление ресурсов,
чтобы работать на слабом железе. На сервере с 64 ГБ RAM эти настройки
приводят к тому, что база использует только 128 МБ RAM (shared_buffers),
что вызывает постоянное чтение с диска и тормоза.

Правильный тюнинг может увеличить производительность в 10-100 раз.

1.2. ПАРАМЕТРЫ ПАМЯТИ

  shared_buffers:
    - Самый важный параметр.
    - Кэш данных в памяти PostgreSQL (буферный кэш).
    - Рекомендация: 25% от общей RAM (для выделенного сервера).
    - Максимальное значение: не более 50% RAM (чтобы оставить память ОС для кэша).
    - Пример для 64 ГБ: 16 ГБ.
    - Влияет на: скорость чтения, количество дисковых I/O.
    - Менять без перезагрузки: нельзя (требуется restart).

  work_mem:
    - Память для операций сортировки, хеш-таблиц, JOIN.
    - Выделяется на каждую операцию (не на соединение!).
    - Рекомендация: 4-16 МБ для OLTP, 64-256 МБ для аналитики.
    - Важно: при большом количестве параллельных операций может переполнить память.
    - Пример для 64 ГБ: 64 МБ.
    - Менять без перезагрузки: можно (SET work_mem = '64MB';)

  maintenance_work_mem:
    - Память для VACUUM, CREATE INDEX, REINDEX, ALTER TABLE.
    - Выделяется на каждую операцию обслуживания.
    - Рекомендация: 5-10% от RAM для сервера с частым VACUUM.
    - Пример для 64 ГБ: 1-2 ГБ.
    - Менять без перезагрузки: можно (SET maintenance_work_mem = '1GB';)

  effective_cache_size:
    - Размер кэша файловой системы, доступного PostgreSQL.
    - Это не реальный кэш, а оценка для планировщика.
    - Рекомендация: 75% от общей RAM.
    - Пример для 64 ГБ: 48 ГБ.
    - Влияет на выбор плана запроса (использовать индекс или seq scan).

  wal_buffers:
    - Буфер для WAL (Write-Ahead Log).
    - Рекомендация: 16-64 МБ.
    - Менять без перезагрузки: нельзя (требуется restart).

  huge_pages:
    - Использование больших страниц памяти (2 МБ вместо 4 КБ).
    - Улучшает производительность при большом shared_buffers.
    - Рекомендация: включить, если ОС поддерживает.
    - Менять без перезагрузки: нельзя (требуется restart).

1.3. ПАРАМЕТРЫ I/O

  checkpoint_timeout:
    - Максимальное время между контрольными точками.
    - Рекомендация: 5-15 минут (для больших серверов — 10-15 мин).
    - Влияет на: количество WAL-данных, нагрузку на диск.

  checkpoint_completion_target:
    - Доля времени между чекпоинтами для завершения записи.
    - Рекомендация: 0.9 (распределяет нагрузку плавно).
    - Чем выше, тем равномернее запись на диск.

  wal_keep_size:
    - Размер WAL-файлов, хранимых для репликации.
    - Рекомендация: 64-256 МБ или больше (в зависимости от реплик).

  max_wal_senders:
    - Максимальное количество соединений для репликации.
    - Рекомендация: 10-20 (если есть реплики).

1.4. ПАРАМЕТРЫ СОЕДИНЕНИЙ

  max_connections:
    - Максимальное количество подключений к БД.
    - Рекомендация: 100-300 (для веб-приложений).
    - Слишком большое значение увеличивает потребление памяти.
    - Расчёт: примерно 2-5 МБ на соединение.

  superuser_reserved_connections:
    - Количество соединений, зарезервированных для суперпользователя.
    - Рекомендация: 3-5.

1.5. КАК РАССЧИТЫВАТЬ ПАРАМЕТРЫ

  Формулы для сервера с RAM = 64 ГБ:
    shared_buffers = 16 ГБ (25% RAM)
    effective_cache_size = 48 ГБ (75% RAM)
    maintenance_work_mem = 2 ГБ (3% RAM)
    work_mem = 64 МБ
    wal_buffers = 64 МБ
    max_connections = 200

  Общая память, потребляемая PostgreSQL:
    shared_buffers + (work_mem * max_connections) + maintenance_work_mem + wal_buffers
    = 16 ГБ + (64 МБ * 200) + 2 ГБ + 64 МБ = 16 + 12.8 + 2 + 0.064 ≈ 31 ГБ

  Это безопасно для сервера с 64 ГБ.

1.6. МОНИТОРИНГ ЭФФЕКТИВНОСТИ

  Важные метрики:
    - Cache hit ratio (доля чтений из кэша) — должна быть > 95% (для OLTP)
    - Checkpoint frequency — не слишком часто (каждые 5-15 минут)
    - IOPS и latency — не должны быть высокими

  Запросы для проверки:
    - SHOW shared_buffers;
    - SHOW work_mem;
    - SHOW effective_cache_size;
    - SELECT * FROM pg_stat_bgwriter;
*/

-- ЧАСТЬ 2. ПРАКТИКА (НАСТРОЙКА POSTGRESQL.CONF)

-- 2.1. ПРОВЕРКА ТЕКУЩИХ НАСТРОЕК
SHOW shared_buffers;
SHOW work_mem;
SHOW maintenance_work_mem;
SHOW effective_cache_size;
SHOW wal_buffers;
SHOW max_connections;
SHOW checkpoint_timeout;

-- 2.2. ПРИМЕР КОНФИГА ДЛЯ СЕРВЕРА С 64 ГБ RAM

-- 2.2. ПРИМЕР КОНФИГА ДЛЯ СЕРВЕРА С 64 ГБ RAM
-- ===========================================================================

/*
Файл: /etc/postgresql/15/main/postgresql.conf

# =============================================================================
# ПАМЯТЬ
# =============================================================================

shared_buffers = 16GB                    # 25% от RAM
effective_cache_size = 48GB              # 75% от RAM
maintenance_work_mem = 2GB               # 3% от RAM
work_mem = 64MB                          # на операцию (не на соединение!)
wal_buffers = 64MB
huge_pages = on                          # если ОС поддерживает

# I/O И ЧЕКПОИНТЫ

checkpoint_timeout = 10min
checkpoint_completion_target = 0.9
wal_keep_size = 256MB
max_wal_senders = 10
wal_compression = on

# СОЕДИНЕНИЯ

max_connections = 200
superuser_reserved_connections = 5

# ЛОГИРОВАНИЕ

log_min_duration_statement = 1000ms       # логировать медленные запросы
log_statement = 'ddl'                     # логировать изменения структуры
log_line_prefix = '%t [%p] %u %d '        # время, PID, пользователь, БД

# ПРОЧЕЕ

random_page_cost = 1.1                   # для SSD (или 4 для HDD)
effective_io_concurrency = 200           # для SSD
max_parallel_workers = 8
max_parallel_workers_per_gather = 4
*/


-- 2.3. ИЗМЕНЕНИЕ НАСТРОЕК БЕЗ ПЕРЕЗАГРУЗКИ

-- Изменить work_mem для текущей сессии
SET work_mem = '128MB';

-- Изменить maintenance_work_mem для текущей сессии
SET maintenance_work_mem = '2GB';

-- Проверить изменения
SHOW work_mem;
SHOW maintenance_work_mem;

-- Изменить для всей БД (сохраняется в БД, не требует restart)
ALTER DATABASE mydb SET work_mem = '64MB';
ALTER DATABASE mydb SET maintenance_work_mem = '2GB';

-- Изменить для конкретного пользователя
ALTER ROLE myuser SET work_mem = '128MB';

-- 2.4. МОНИТОРИНГ ПРОИЗВОДИТЕЛЬНОСТИ

-- 2.4.1. Cache hit ratio (доля чтений из кэша)
-- Должна быть > 95% для OLTP
SELECT
    'cache_hit_ratio' AS name,
    round(
        sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) * 100.0,
        2
    ) AS value
FROM pg_statio_user_tables;

-- 2.4.2. Статистика чекпоинтов
SELECT
    checkpoints_timed,
    checkpoints_req,
    checkpoint_write_time,
    checkpoint_sync_time,
    buffers_checkpoint,
    buffers_clean
FROM pg_stat_bgwriter;

-- 2.4.3. Размер shared_buffers (в блоках и байтах)
SELECT
    setting AS buffers,
    current_setting('shared_buffers') AS size,
    pg_size_pretty(pg_settings.setting::int * 8192) AS pretty_size
FROM pg_settings
WHERE name = 'shared_buffers';

-- 2.4.4. Количество соединений
SELECT
    COUNT(*) AS total_connections,
    COUNT(*) FILTER (WHERE state = 'active') AS active,
    COUNT(*) FILTER (WHERE state = 'idle') AS idle
FROM pg_stat_activity;

-- 2.5. ПРОВЕРКА ЭФФЕКТИВНОСТИ НАСТРОЕК
-- 2.5.1. Найти запросы с высоким потреблением work_mem
SELECT
    query,
    calls,
    temp_blks_read,
    temp_blks_written,
    total_exec_time
FROM pg_stat_statements
WHERE temp_blks_read > 0
ORDER BY temp_blks_read DESC
LIMIT 10;

-- 2.6. РАСЧЁТ ПАРАМЕТРОВ ДЛЯ РАЗНЫХ СЕРВЕРОВ

/*
Сервер с 16 ГБ RAM (небольшой проект):
  shared_buffers = 4GB
  effective_cache_size = 12GB
  maintenance_work_mem = 512MB
  work_mem = 16MB
  max_connections = 100

Сервер с 32 ГБ RAM (средний проект):
  shared_buffers = 8GB
  effective_cache_size = 24GB
  maintenance_work_mem = 1GB
  work_mem = 32MB
  max_connections = 150

Сервер с 64 ГБ RAM (крупный проект):
  shared_buffers = 16GB
  effective_cache_size = 48GB
  maintenance_work_mem = 2GB
  work_mem = 64MB
  max_connections = 200

Сервер с 128 ГБ RAM (high-load):
  shared_buffers = 32GB
  effective_cache_size = 96GB
  maintenance_work_mem = 4GB
  work_mem = 128MB
  max_connections = 300
*/

-- 2.7. ТЮНИНГ ДЛЯ КОНКРЕТНОЙ НАГРУЗКИ
-- 2.7.1. OLTP (много маленьких запросов, высокая частота)
--   shared_buffers = 25% RAM
--   work_mem = 8-16 MB
--   checkpoint_timeout = 5-10 min
--   random_page_cost = 1.1 (для SSD)

-- 2.7.2. OLAP (большие аналитические запросы, редкие)
--   shared_buffers = 50% RAM
--   work_mem = 256-1024 MB
--   checkpoint_timeout = 15-30 min
--   maintenance_work_mem = 5-10% RAM

-- 2.7.3. Смешанная нагрузка
--   shared_buffers = 30% RAM
--   work_mem = 32-64 MB
--   checkpoint_timeout = 10-15 min

-- 2.8. ПРОВЕРКА, ЧТО НАСТРОЙКИ ПРИМЕНИЛИСЬ

-- После изменения postgresql.conf нужно выполнить:
-- sudo systemctl restart postgresql

-- Или перезагрузить конфиг без перезапуска:
-- SELECT pg_reload_conf();

-- Проверить изменения:
SELECT
    name,
    setting,
    unit,
    source,
    pending_restart
FROM pg_settings
WHERE name IN (
    'shared_buffers',
    'work_mem',
    'maintenance_work_mem',
    'effective_cache_size',
    'wal_buffers',
    'max_connections',
    'checkpoint_timeout'
)
ORDER BY name;

-- pending_restart = true означает, что требуется перезапуск

-- 2.9. АНАЛИЗ ЭФФЕКТИВНОСТИ НАСТРОЕК С ПОМОЩЬЮ EXPLAIN

-- Сравните время выполнения запроса до и после изменения настроек

-- Без индексов (зависит от shared_buffers)
EXPLAIN (ANALYZE, BUFFERS)
SELECT COUNT(*) FROM logs_partitioned WHERE user_id = 42;

-- Создаём индекс
CREATE INDEX idx_logs_user_id ON logs_partitioned (user_id);

-- Снова смотрим план (теперь Index Only Scan)
EXPLAIN (ANALYZE, BUFFERS)
SELECT COUNT(*) FROM logs_partitioned WHERE user_id = 42;

-- Сравните количество буферов (shared_buffers) до и после

-- 2.10. ПРИМЕРЫ (СЛОЖНЫЕ НАСТРОЙКИ)

-- 2.10.1. Настройка autovacuum для больших таблиц
-- В postgresql.conf:
-- autovacuum_vacuum_scale_factor = 0.05
-- autovacuum_analyze_scale_factor = 0.02
-- autovacuum_vacuum_threshold = 1000

-- 2.10.2. Настройка для таблицы с интенсивными обновлениями
ALTER TABLE notification_settings SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_threshold = 1000
);

-- 2.10.3. Настройка для партиционированной таблицы
ALTER TABLE logs_partitioned SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_vacuum_threshold = 5000
);

-- 2.10.4. Проверка эффективности autovacuum
SELECT
    relname,
    n_live_tup,
    n_dead_tup,
    last_vacuum,
    last_autovacuum,
    autovacuum_count
FROM pg_stat_user_tables
WHERE n_dead_tup > 1000
ORDER BY n_dead_tup DESC;