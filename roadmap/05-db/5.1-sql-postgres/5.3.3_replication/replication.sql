/*
ШАГ 3.3: РЕПЛИКАЦИЯ И ВЫСОКАЯ ДОСТУПНОСТЬ (HA)

ЛОГИКА: Сервер с Postgres упал. Как сделать так, чтобы база не была единой точкой отказа?
Ответ: Репликация — копирование данных на другой сервер. Если мастер упал, реплика становится новым мастером.

ЧАСТЬ 1. ТЕОРИЯ 

1.1. ЧТО ТАКОЕ РЕПЛИКАЦИЯ?
Репликация — это процесс копирования данных с одного сервера PostgreSQL (мастер) на один или несколько других серверов (реплики).

Зачем нужна репликация:
  1. Отказоустойчивость (High Availability) — если мастер упал, реплика становится мастером.
  2. Балансировка нагрузки — чтение можно распределять между репликами.
  3. Бэкап — можно делать бэкап с реплики, не нагружая мастер.
  4. Аналитика — можно запускать тяжелые аналитические запросы на реплике.

1.2. MASTER-SLAVE (PRIMARY-STANDBY) АРХИТЕКТУРА
  - Master (Primary) — сервер, который принимает запись (INSERT, UPDATE, DELETE).
  - Slave (Standby) — сервер, который только читает (SELECT)(Hot). Получает данные от мастера через WAL.

  Терминология:
    - Primary = Master = Главный
    - Standby = Replica = Slave = Реплика
    - Hot Standby — реплика, которая принимает запросы на чтение.
    - Warm Standby — реплика, которая не принимает запросы (только ждёт переключения).

1.2.1. MASTER-SLAVE — добавим про Hot Standby и Streaming Replication
Важный нюанс: Hot Standby — это режим, при котором реплика применяет WAL и одновременно принимает запросы на чтение.
Это работает благодаря тому, что PostgreSQL на реплике использует механизм MVCC, чтобы показывать консистентный
снимок данных, даже если WAL применяется параллельно.

Что происходит на реплике:
* Реплика получает WAL-записи.
* Проигрывает их в фоновом процессе (wal receiver).
* Читатели видят только те данные, которые уже применены.
* Это позволяет распределять нагрузку на чтение без блокировок.

Ограничения:
* На реплике нельзя выполнять INSERT, UPDATE, DELETE (только чтение).
* Нельзя создавать временные таблицы в READ ONLY режиме.
* Некоторые типы блокировок (LOCK TABLE) работают иначе.

1.3. СИНХРОННАЯ vs АСИНХРОННАЯ РЕПЛИКАЦИЯ

  Характеристика        | Синхронная                     | Асинхронная
  ----------------------|--------------------------------|--------------------------
  Данные на реплике     | Гарантированно записаны        | Может быть задержка
  Производительность    | Медленнее (ждёт подтверждения) | Быстрее (не ждёт)
  Риск потери данных    | Низкий                         | Высокий (если мастер упал до отправки)
  Настройка             | synchronous_commit = on        | synchronous_commit = off
  Применение            | Финансы, критичные данные      | Логи, аналитика, некритичные данные

1.3.1.  добавим про synchronous_standby_names
Как выбрать, какая реплика синхронная.

Настройка синхронной реплики:
-- В postgresql.conf на мастере
synchronous_commit = on
synchronous_standby_names = 'replica1, replica2'
Как это работает:
* Мастер ждёт подтверждения от всех реплик, указанных в synchronous_standby_names.
* Если реплика недоступна, мастер зависает (или падает по таймауту).
* Можно использовать FIRST и ANY для управления количеством реплик:

-- Ждём подтверждения от первой доступной реплики
synchronous_standby_names = 'FIRST 1 (replica1, replica2)'
-- Ждём подтверждения от двух любых реплик
synchronous_standby_names = 'ANY 2 (replica1, replica2, replica3)'

1.4. WAL (WRITE-AHEAD LOG) — СЕРДЦЕ РЕПЛИКАЦИИ
  Что такое WAL?
    - Журнал предзаписи. Все изменения сначала пишутся в WAL, а потом в таблицы.
    - Это гарантирует, что данные не потеряются даже при сбое.

  Как работает репликация через WAL:
    1. Мастер пишет изменения в WAL.
    2. WAL-записи отправляются на реплику (Streaming Replication).
    3. Реплика проигрывает WAL и применяет изменения к своим данным.
    4. Реплика всегда отстаёт от мастера на несколько байт (или секунд).

1.4.1. WAL — добавим про WAL Archiving и Restore Command
Добавлю важный нюанс про архивацию:

Для восстановления после сбоя и для Point-in-Time Recovery (PITR) WAL-файлы нужно архивировать.

Настройка архивации:
-- В postgresql.conf
wal_level = replica
archive_mode = on
archive_command = 'cp %p /archive/%f'

Как это работает:
После того как WAL-сегмент заполнен, он архивируется в указанную директорию.
При восстановлении можно применить все архивированные WAL-файлы, чтобы вернуть базу в состояние на любой момент времени.

Restore Command:

Используется при восстановлении: из архива подтягиваются WAL-файлы.

restore_command = 'cp /archive/%f %p'

1.5. ТИПЫ РЕПЛИКАЦИИ
  Streaming Replication:
    - Непрерывная передача WAL по сети.
    - Почти реальное время (задержка миллисекунды).
    - Только для всей базы данных (нельзя реплицировать отдельные таблицы).

  Logical Replication:
    - Репликация на уровне таблиц.
    - Можно выбрать конкретные таблицы.
    - Поддерживает разные версии PostgreSQL.
    - Используется для миграций и интеграций.

  Log Shipping (устаревший):
    - Передача WAL-файлов по расписанию (например, каждые 5 минут).
    - Большая задержка.

1.6. FAILOVER (ПЕРЕКЛЮЧЕНИЕ)
  Failover — это процесс переключения на реплику при падении мастера.

  Ручной Failover:
    - Администратор вручную переключает реплику на мастер.
    - Требует времени и доступа к серверу.
    - Риск ошибки.

  Автоматический Failover:
    - Инструменты (Patroni, repmgr, Stolon) сами мониторят мастер и переключают.
    - Время переключения: 10-30 секунд.

1.7. ИНСТРУМЕНТЫ ДЛЯ HA
  Patroni:
    - Самый популярный инструмент для HA в PostgreSQL.
    - Использует etcd или Consul для хранения состояния кластера.
    - Автоматический failover.
    - Поддерживает автоматическое восстановление (pg_rewind).

  repmgr:
    - Инструмент от 2ndQuadrant.
    - Проще, чем Patroni.
    - Поддерживает ручной и автоматический failover.

  Stolon:
    - Написан на Go.
    - Использует etcd или Consul.

  PgBouncer (для балансировки):
    - Пулы соединений.
    - Может направлять запросы на разные серверы.

1.8. МОНИТОРИНГ РЕПЛИКАЦИИ
  Основные метрики:
    - Задержка репликации (replication lag) — насколько реплика отстаёт от мастера.
    - Состояние реплик (state) — streaming, catching up, etc.
    - Статус синхронизации (sync_state) — sync, async, potential.

  Системные представления:
    - pg_stat_replication — статус реплик (на мастере).
    - pg_stat_wal_receiver — статус получения WAL (на реплике).
    - pg_is_in_recovery() — true, если сервер — реплика.

1.9. БАЛАНСИРОВКА НАГРУЗКИ
  Чтение можно распределять между репликами, чтобы снизить нагрузку на мастер.
  Способы:
    - На уровне приложения (Go-код выбирает случайную реплику).
    - На уровне инфраструктуры (PgBouncer, HAProxy, DNS round-robin).
    - Использование read-only транзакций (SET TRANSACTION READ ONLY).

1.10. СВЯЗЬ С GO (КАК ПИСАТЬ СЕРВИС)

  В Go-сервисе нужно уметь:
    1. Разделять чтение и запись (читаем с реплик, пишем на мастер).
    2. Проверять доступность мастера и переключаться на реплику.
    3. Обрабатывать ошибки подключения и повторять запросы.

ФИШКИ

1: «Репликация и DDL — почему ALTER TABLE может вызвать проблемы»
Суть: При ALTER TABLE на мастере WAL запись отправляется на реплики. Но если на реплике
есть блокировка (например, длительный SELECT), применение WAL может заблокироваться и реплика отстанет.

Решение: Использовать ALTER TABLE ... CONCURRENTLY, чтобы минимизировать блокировки.

Общая мысль:
DDL-операции на мастере реплицируются через WAL и могут блокироваться на репликах из-за долгих запросов.
Надо использовать CONCURRENTLY для длительных DDL, чтобы минимизировать блокировки.

2: «Logical Replication vs Streaming Replication — подводные камни»
Суть: Logical Replication не реплицирует DDL, SEQUENCE, TRUNCATE.
Если ты изменишь структуру таблицы на мастере, подписка на реплике может сломаться.

Решение: Использовать ALTER TABLE с осторожностью и проверять статус подписки.

Общая мысль:
Logical Replication не реплицирует DDL и SEQUENCE. Если я изменяю структуру таблицы, мне нужно убедиться,
что подписка на реплике не сломается, или использовать Streaming Replication для полной репликации всей базы.

3: «Репликация и pg_rewind — восстановление без копирования всего кластера»
Суть: Если мастер упал и восстановился, но его WAL-позиция отличается от нового мастера,
pg_rewind позволяет синхронизировать его без полного копирования данных.

Как это работает:
* Старый мастер монтируется как реплика нового мастера.
* pg_rewind анализирует различия в WAL и перезаписывает только изменённые блоки.
* После этого старый мастер можно запустить как реплику.

Общая мысль:
pg_rewind позволяет синхронизировать старый мастер с новым без полного копирования данных.
Это экономит время и трафик, особенно при больших объёмах.

4: «Репликация и synchronous_commit — не все операции одинаковы»
Суть: synchronous_commit = on не гарантирует, что реплика применила WAL,
только что она получила и записала в свой WAL. Данные могут быть ещё не применены к таблицам.

Решение: Использовать synchronous_commit = remote_apply, чтобы ждать, пока данные будут применены на реплике.
-- Ждём, пока реплика применит данные
synchronous_commit = remote_apply

Общая мысль:
«synchronous_commit имеет уровни: on (ждёт записи в WAL на реплике), remote_apply (ждёт применения данных на реплике).
Для критичных операций я использую remote_apply, чтобы гарантировать, что данные доступны для чтения на реплике.»
*/
-- ЧАСТЬ 2. ПРАКТИКА

-- 2.1. ПРОВЕРКА СТАТУСА РЕПЛИК
-- 2.1.1. Проверка, является ли сервер репликой
SELECT pg_is_in_recovery();
-- true = это реплика, false = это мастер

-- 2.1.2. Просмотр статуса реплик (на мастере)
SELECT
    pid,
    usename,
    application_name,
    client_addr,
    state,
    sync_state,
    write_lag,
    flush_lag,
    replay_lag
FROM pg_stat_replication;

-- 2.1.3. На мастере: количество реплик
SELECT COUNT(*) AS replica_count
FROM ps_stat_replication;

-- 2.2. МОНИТОРИНГ ЗАДЕРЖКИ (LAG)
-- 2.2.1. Задержка в байтах (на реплике)
SELECT
    pg_last_wal_receive_lsn() AS receive_lsn,
    pg_last_wal_replay_lsn() AS replay_lsn,
    pg_wal_lsn_diff(pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn()) AS lag_bytes;

-- 2.2.2. Задержка в секундах (на мастере)
SELECT
    application_name,
    state,
    EXTRACT(EPOCH FROM replay_lag) AS replay_lag_seconds
FROM pg_stat_replication
WHERE state = 'streaming';

-- 2.2.3. Проверка, что реплика синхронизирована (на реплике)
SELECT
    pg_is_in_recovery() AS is_replica,
    pg_last_wal_replay_lsn() AS last_replay,
    pg_current_wal_lsn() AS current_wal; -- не работает на реплике, только на мастере

-- 2.3. НАСТРОЙКА РЕПЛИКАЦИИ
-- 2.3.1. Создание пользователя для репликации (на мастере)
CREATE USER replica_user WITH REPLICATION ENCRYPTED PASSWORD 'strong_password';

-- 2.3.2. Включение синхронной репликации
SET synchronous_commit = on;
SET synchronous_standby_names = 'standby1, standby2';

-- 2.3.3. Отключение синхронной репликации
SET synchronous_commit = off;

-- 2.3.4. Переключение в режим только для чтения (на реплике)
SET default_transaction_read_only = on;

-- 2.3.5. Повышение реплики до мастера (ручной failover)
SELECT pg_promote();

-- 2.4. РАБОТА С WAL (ДЛЯ АРХИТЕКТОРОВ)

-- 2.4.1. Просмотр текущей позиции в WAL
SELECT pg_current_wal_lsn();

-- 2.4.2. Просмотр последних WAL-файлов
SELECT * FROM pg_ls_waldir() ORDER BY modification DESC LIMIT 10;

-- 2.4.3. Просмотр размера WAL
SELECT pg_wal_lsn_diff(pg_current_wal_lsn(), '0/0') AS wal_size_bytes;

-- 2.4.4. Принудительная архивация WAL
SELECT pg_switch_wal();

-- 2.5. ДИАГНОСТИКА РЕПЛИКАЦИИ

-- 2.5.1. Проверка настроек репликации на мастере
SHOW wal_level;
SHOW max_wal_senders;
SHOW wal_keep_size;
SHOW synchronous_commit;
SHOW synchronous_standby_names;

-- 2.5.2. Проверка настроек на реплике
SHOW hot_standby;
SHOW primary_conninfo;
SHOW restore_command;

-- 2.5.3. Просмотр слотов репликации
SELECT * FROM pg_replication_slots;

-- 2.5.4. Удаление слота репликации (если реплика больше не нужна)
SELECT pg_drop_replication_slot('slot_name');

-- 2.6. FAILOVER (ПЕРЕКЛЮЧЕНИЕ)

-- 2.6.1. Проверка, можно ли повысить реплику до мастера (на реплике)
SELECT pg_wal_replay_pause();  -- приостановить воспроизведение
SELECT pg_wal_replay_resume(); -- возобновить воспроизведение

-- 2.6.2. Проверка целостности данных после failover
-- Сравнение количества записей на мастере и реплике (пример)
SELECT 'master' AS server, COUNT(*) FROM users
UNION ALL
SELECT 'replica' AS server, COUNT(*) FROM users;

-- 2.6.3. Тестирование failover (не запускать на продакшене!)
-- На реплике
SELECT pg_promote(true, 60); -- форсировать переключение с таймаутом 60 сек

-- 2.7. БАЛАНСИРОВКА НАГРУЗКИ (ЧТЕНИЕ С РЕПЛИК)

-- 2.7.1. Создание пользователя только для чтения с реплик
CREATE USER readonly_user WITH PASSWORD 'readonly_password';
GRANT CONNECT ON DATABASE mydb TO readonly_user;
GRANT USAGE ON SCHEMA public TO readonly_user;
GRANT SELECT ON ALL TABLES IN SCHEMA public TO readonly_user;

-- 2.7.2. Принудительное использование реплики для запроса
-- (можно через комментарий, чтобы отслеживать)
/* REPLICA */ SELECT * FROM user WHERE id = 1;

-- 2.7.3. Транзакция только для чтения (на реплике)
BEGIN;
SET TRANSACTION READ ONLY
SELECT * FROM users;
COMMIT;

-- 2.8. УСТРАНЕНИЕ ПРОБЛЕМ

-- 2.8.1. Реплика отстаёт (lag > 1 час)
-- Проверить, что мастер отправляет WAL
SELECT pg_current_wal_lsn();

-- Проверить, что реплика получает WAL
SELECT pg_last_wal_receive_lsn();

-- Принудительно переключить WAL-файл
SELECT pg_switch_wal();

-- 2.8.2. Реплика перестала получать данные (state = 'catchup')
-- Проверить слоты репликации
SELECT * FROM pg_replication_slots WHERE active = false;

-- Пересоздать слот репликации (если повреждён)
-- SELECT pg_drop_replication_slot('slot_name');
-- Затем пересоздать

-- 2.8.3. Мастер упал, реплика не становится мастером
-- Проверить, что реплика в режиме восстановления
SELECT pg_is_in_recovery();

-- Если true, принудительно повысить
SELECT pg_promote();

-- 2.9. ПРИМЕРЫ

-- 2.9.1. Проверка консистентности данных между мастером и репликой
-- Сравнение хешей таблиц
SELECT md5(STRING_AGG(id::text || name, ',' ORDER BY id)) AS table_hash
FROM users;

-- 2.9.2. Восстановление реплики после длительного отставания
-- На реплике
SELECT pg_wal_replay_pause();
-- Дождаться, пока реплика догонит мастер
SELECT pg_last_wal_receive_lsn(), pg_last_wal_replay_lsn();
-- Возобновить
SELECT pg_wal_replay_resume();

-- 2.9.3. Каскадная репликация (реплика от реплики)
-- На первой реплике (которая будет источником для второй)
-- В postgresql.conf:
-- wal_level = replica
-- max_wal_senders = 10

-- На второй реплике:
-- primary_conninfo = 'host=replica1 port=5432 user=replica_user password=...'

-- 2.9.4. Логическая репликация (таблицы)
-- На мастере
CREATE PUBLICATION my_pub FOR TABLE users, orders;

-- На реплике
CREATE SUBSCRIPTION my_sub
CONNECTION 'host=master port=5432 dbname=mydb user=replica_user password=...'
PUBLICATION my_pub;

-- Проверка
SELECT * FROM pg_publication;
SELECT * FROM pg_subscription;

-- 2.9.5. Мониторинг задержки с созданием оповещений
CREATE OR REPLACE FUNCTION check_replication_lag()
RETURNS VOID AS $$
DECLARE
    lag_seconds NUMERIC;
BEGIN
    SELECT EXTRACT(EPOCH FROM replay_lag) INTO lag_seconds
    FROM pg_stat_replication
    WHERE state = 'streaming'
    LIMIT 1;

    IF lag_seconds > 60 THEN
        RAISE WARNING 'Replication lag is % seconds!', lag_seconds;
        -- Здесь можно отправить оповещение (через pg_notify или внешний скрипт)
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Запуск проверки (можно по расписанию через cron)
SELECT check_replication_lag();