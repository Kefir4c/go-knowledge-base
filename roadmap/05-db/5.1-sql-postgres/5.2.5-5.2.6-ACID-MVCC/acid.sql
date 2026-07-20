/*
ШАГ 2.5: ТРАНЗАКЦИИ И КОНКУРЕНТНОСТЬ (ACID)

Этот блок — один из самых важных для собеседования. Тебя будут спрашивать:
  - Что такое транзакция?
  - Какие есть уровни изоляции?
  - Что такое dirty read, non-repeatable read, phantom read?
  - Как работает MVCC в PostgreSQL?
  - Как избежать deadlock?
  - Чем отличается FOR UPDATE от SKIP LOCKED?

Ниже — максимально подробная теория с примерами.

1. ЧТО ТАКОЕ ТРАНЗАКЦИЯ?
Транзакция — это группа операций, которые выполняются как единое целое.
Либо все операции выполняются успешно, либо ни одна.

В SQL транзакция начинается с BEGIN (или START TRANSACTION) и заканчивается
COMMIT (фиксация) или ROLLBACK (откат).

Пример:
  BEGIN;
  UPDATE accounts SET balance = balance - 100 WHERE id = 1;
  UPDATE accounts SET balance = balance + 100 WHERE id = 2;
  COMMIT;  -- или ROLLBACK;

Если между BEGIN и COMMIT произошла ошибка, все изменения откатываются.

2. ACID — ЧЕТЫРЕ СВОЙСТВА ТРАНЗАКЦИЙ
ACID — это аббревиатура, которая описывает свойства надёжных транзакций.

  1. Atomicity (Атомарность)
     Транзакция неделима. Если одна операция не удалась, вся транзакция откатывается.
     Пример: перевод денег — списание и зачисление должны быть атомарны.

  2. Consistency (Согласованность)
     Транзакция переводит базу из одного корректного состояния в другое.
     Все ограничения (CHECK, FOREIGN KEY, UNIQUE) должны соблюдаться.
     Пример: баланс не может стать отрицательным (CHECK balance >= 0).

  3. Isolation (Изоляция)
     Параллельные транзакции не должны мешать друг другу.
     Уровни изоляции определяют, какие аномалии допустимы.
     PostgreSQL по умолчанию: READ COMMITTED.

  4. Durability (Долговечность)
     После COMMIT изменения сохраняются на диске и не пропадают даже при сбое.

3. АНОМАЛИИ ПРИ ПАРАЛЛЕЛЬНЫХ ТРАНЗАКЦИЯХ
При параллельном выполнении транзакций могут возникать аномалии:

  Dirty Read (Грязное чтение):
    Транзакция читает данные, которые ещё не зафиксированы другой транзакцией.
    Если та транзакция откатится, данные станут невалидными.
    В PostgreSQL НЕВОЗМОЖЕН даже на READ UNCOMMITTED, потому что
    PostgreSQL не поддерживает READ UNCOMMITTED (он работает как READ COMMITTED).

    Пример:
      Транзакция А: UPDATE accounts SET balance = 100 WHERE id = 1; (не закоммичено)
      Транзакция Б: SELECT balance FROM accounts WHERE id = 1;  -- читает 100
      Транзакция А: ROLLBACK;  -- баланс вернулся к старому значению
      Транзакция Б: использовала несуществующие данные.

  Non-Repeatable Read (Неповторяемое чтение):
    Транзакция читает одну и ту же строку дважды, а между чтениями другая
    транзакция изменяет её. Результаты отличаются.
    Решается уровнем REPEATABLE READ.

    Пример:
      Транзакция А: SELECT balance FROM accounts WHERE id = 1;  -- 1000
      Транзакция Б: UPDATE accounts SET balance = 900 WHERE id = 1; COMMIT;
      Транзакция А: SELECT balance FROM accounts WHERE id = 1;  -- 900 (отличается)

  Phantom Read (Фантомное чтение):
    Транзакция читает набор строк по условию, а между чтениями другая
    транзакция добавляет или удаляет строки, попадающие под условие.
    Решается уровнем SERIALIZABLE.

    Пример:
      Транзакция А: SELECT * FROM accounts WHERE balance > 500;  -- 2 строки
      Транзакция Б: INSERT INTO accounts (balance) VALUES (600); COMMIT;
      Транзакция А: SELECT * FROM accounts WHERE balance > 500;  -- 3 строки

4. УРОВНИ ИЗОЛЯЦИИ В POSTGRESQL
PostgreSQL поддерживает четыре уровня изоляции (стандарт SQL):

  Уровень               | Dirty Read | Non-Repeatable Read | Phantom Read
  ----------------------|------------|---------------------|-------------
  READ UNCOMMITTED      | Возможен   | Возможен            | Возможен
  READ COMMITTED (psql) | Нет        | Возможен            | Возможен
  REPEATABLE READ       | Нет        | Нет                 | Возможен (в стандарте), но в PostgreSQL НЕТ
  SERIALIZABLE          | Нет        | Нет                 | Нет

  В PostgreSQL:
    - READ UNCOMMITTED работает как READ COMMITTED.
    - REPEATABLE READ защищает и от Phantom Read (через MVCC).
    - SERIALIZABLE — самый строгий, может выдавать ошибки сериализации.

  Установка уровня изоляции:
    SET TRANSACTION ISOLATION LEVEL REPEATABLE READ;
    BEGIN;
    -- ваш запрос
    COMMIT;

5. MVCC (MULTI-VERSION CONCURRENCY CONTROL)
PostgreSQL использует MVCC для реализации изоляции без блокировок.

  Как это работает:
    - Каждая строка имеет скрытые системные колонки: xmin (ID транзакции,
    которая создала строку) и xmax (ID транзакции, которая удалила строку).
    - При UPDATE создаётся новая версия строки, старая помечается как удалённая (xmax).
    - Каждая транзакция видит только те версии строк, которые были закоммичены
    на момент её начала (snapshot).

  Это позволяет:
    - Читателям не блокировать писателей (и наоборот).
    - Получать консистентный снимок данных на момент начала транзакции.

  Плата за MVCC:
    - Старые версии строк (dead tuples) накапливаются и требуют VACUUM.
    - Размер таблицы может расти, если не выполнять VACUUM.

5.1. MVCC — добавим про xmin и xmax и "проблему 2 млрд"
Проблема: Transaction ID Wraparound (переполнение счётчика транзакций)
* xmin и xmax — это 32-битные числа. Максимальное значение — около 2 миллиардов.
* Когда счётчик транзакций достигает этого предела, он переполняется и начинается с 1.
* PostgreSQL использует «кольцевой» счётчик. Чтобы понять, какая транзакция старше, используется vacuum_freeze_min_age.

Симптомы:
* База внезапно начинает падать с ошибкой database is not accepting commands to avoid wraparound data loss in database "...".
* AUTOVACUUM не успевает «замораживать» старые строки.

Решение:
* Включить AUTOVACUUM.
* Мониторить возраст транзакций: SELECT age(datfrozenxid) FROM pg_database;
* Если возраст подходит к 500 млн — выполнить VACUUM FREEZE.

В целом:
В PostgreSQL есть скрытая проблема — wraparound transaction IDs. Если не выполнять VACUUM и не замораживать старые строки,
база может перестать принимать запросы. Поэтому всегда надо следить за возрастом транзакций через мониторинг.

6. БЛОКИРОВКИ (LOCKING)
Блокировки нужны, когда MVCC недостаточно (например, для обновления с проверкой).

  6.1. FOR UPDATE — пессимистичная блокировка
Блокирует строки, чтобы другие транзакции не могли их изменить или заблокировать.
    Пример:
      BEGIN;
      SELECT balance FROM accounts WHERE id = 1 FOR UPDATE;
      -- Проверяем баланс, обновляем
      UPDATE accounts SET balance = balance - 100 WHERE id = 1;
      COMMIT;

    Варианты:
      FOR UPDATE NOWAIT  — если строка заблокирована, сразу ошибка.
      FOR UPDATE SKIP LOCKED — пропустить заблокированные строки.

  6.2. SKIP LOCKED — очередь задач
    Используется для реализации очередей: несколько воркеров берут задачи,
    пропуская уже заблокированные.

    Пример:
      UPDATE tasks
      SET status = 'processing'
      WHERE id = (
          SELECT id FROM tasks
          WHERE status = 'pending'
          ORDER BY id
          LIMIT 1
          FOR UPDATE SKIP LOCKED
      )
      RETURNING *;

  6.3. Deadlock (взаимная блокировка)
    Две транзакции ждут друг друга.

    Пример:
      Транзакция A: UPDATE accounts SET balance = balance - 100 WHERE id = 1;
      Транзакция B: UPDATE accounts SET balance = balance - 50 WHERE id = 2;
      Транзакция A: UPDATE accounts SET balance = balance + 100 WHERE id = 2;  -- ждёт B
      Транзакция B: UPDATE accounts SET balance = balance + 50 WHERE id = 1;   -- ждёт A
      → Deadlock! PostgreSQL откатит одну из транзакций.

    Как избежать:
      - Всегда обновляй таблицы в одном порядке (например, по id).
      - Уменьшай время транзакций.
      - Используй оптимистичные блокировки.

7. ОПТИМИСТИЧНАЯ БЛОКИРОВКА (VERSIONING)
Вместо блокировки строк (FOR UPDATE) используется проверка версии.

  Пример:
    BEGIN;
    SELECT balance, version FROM accounts WHERE id = 1;  -- version = 1
    -- вычисляем новый баланс
    UPDATE accounts SET balance = 900, version = 2
    WHERE id = 1 AND version = 1;  -- проверяем, что версия не изменилась
    -- Если обновлено 0 строк → версия изменилась → повторяем транзакцию
    COMMIT;

  Преимущества:
    - Нет блокировок.
    - Лучшая производительность при низкой конкуренции.

  Недостатки:
    - При высокой конкуренции много повторных попыток.
    - Нужно реализовывать логику повторов на стороне приложения.

8. ФИШКИ
1: READ COMMITTED — самый опасный уровень для бизнес-логики
Суть: В READ COMMITTED каждое новое утверждение (SELECT, UPDATE) внутри транзакции видит новый снимок данных.
Это означает, что ты можешь прочитать строку, принять решение на основе неё, а потом обновить её, но к моменту UPDATE данные уже изменились.

Пример из жизни:
-- Транзакция А
BEGIN;
SELECT balance FROM accounts WHERE id = 1;  -- 1000
-- Транзакция Б
UPDATE accounts SET balance = 900 WHERE id = 1; COMMIT;
-- Транзакция А
UPDATE accounts SET balance = balance - 100 WHERE id = 1;  -- обновляет 900 → 800
COMMIT;
Транзакция А думала, что списывает 100 от 1000, но фактически списала 100 от 900. Это баг!
Как исправить: Использовать REPEATABLE READ или SELECT ... FOR UPDATE.

В READ COMMITTED данные могут меняться между запросами внутри транзакции. Это может приводить к логическим ошибкам,
если ты читаешь данные и потом обновляешь на их основе. Для таких операций я использую REPEATABLE READ или FOR UPDATE.

2: Serialization Failure — как с этим жить
Суть: На уровне SERIALIZABLE PostgreSQL может выбросить ошибку could not serialize access due to concurrent update или
serialization_failure. Это не баг, это защита от аномалий.

Что делать: Нужно повторять транзакцию в коде (Go, Java, Python).
Пример в Go:
for attempts := 0; attempts < 3; attempts++ {
    tx, _ := pool.Begin(ctx)
    tx.Exec(ctx, "SET TRANSACTION ISOLATION LEVEL SERIALIZABLE")
    err := tx.QueryRow(ctx, "UPDATE accounts ...").Scan()
    if err == nil {
        tx.Commit(ctx)
        return nil
    }
    tx.Rollback(ctx)
    if isSerializationError(err) {
        time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
        continue
    }
    return err
}
При использовании SERIALIZABLE нужно быть готовым к ошибкам сериализации. Всегда реализую повторные попытки
с экспоненциальной задержкой, чтобы автоматически обрабатывать такие ситуации.

3: FOR UPDATE и индексы — критическая зависимость
Суть: FOR UPDATE без индекса может заблокировать всю таблицу, а не только нужные строки.

Пример:
-- Нет индекса на status → блокировка всей таблицы!
SELECT * FROM tasks WHERE status = 'pending' FOR UPDATE;
Как исправить: Создать индекс на status.

Всегда надо проверять, есть ли индекс на условия в FOR UPDATE. Без индекса блокировка может
распространиться на всю таблицу, что приведёт к падению производительности.

4: «SKIP LOCKED + ORDER BY = идеальная очередь»
Суть: В очереди задач с SKIP LOCKED важно использовать ORDER BY, чтобы гарантировать,
что задачи обрабатываются в правильном порядке (например, по приоритету или по дате создания).

Пример:
BEGIN;
WITH locked AS (
    SELECT id FROM tasks
    WHERE status = 'pending'
    ORDER BY priority DESC, created_at  -- Важно!
    LIMIT 10
    FOR UPDATE SKIP LOCKED
)
UPDATE tasks SET status = 'processing'
FROM locked
WHERE tasks.id = locked.id
RETURNING *;
COMMIT;

Для очередей задач можно использую SKIP LOCKED с ORDER BY, чтобы гарантировать, что задачи обрабатываются в нужном порядке,
а воркеры не блокируют друг друга.
*/

-- Таблица счетов (для переводов)
CREATE TABLE accounts (
    id BIGSERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    balance NUMERIC(10,2) NOT NULL CHECK (balance >= 0),
    version INT DEFAULT 1,  -- для оптимистичной блокировки
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Таблица задач (для SKIP LOCKED)
CREATE TABLE tasks (
    id BIGSERIAL PRIMARY KEY,
    task_data TEXT NOT NULL,
    status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'processing', 'done')),
    assigned_to TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Таблица продуктов (для заказов)
CREATE TABLE products (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    stock INT NOT NULL CHECK (stock >= 0),
    price NUMERIC(10,2) NOT NULL
);

-- Таблица заказов (для демонстрации транзакций)
CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    product_id BIGINT REFERENCES products(id),
    quantity INT NOT NULL CHECK (quantity > 0),
    total_price NUMERIC(10,2) NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Заполняем тестовыми данными
INSERT INTO accounts (user_id, balance) VALUES
    (1, 1000.00),
    (2, 500.00),
    (3, 2000.00),
    (4, 0.00);

INSERT INTO tasks (task_data) VALUES
    ('task_1'), ('task_2'), ('task_3'), ('task_4'), ('task_5'),
    ('task_6'), ('task_7'), ('task_8'), ('task_9'), ('task_10');

INSERT INTO products (name, stock, price) VALUES
    ('Laptop', 10, 1000.00),
    ('Mouse', 50, 25.99),
    ('Keyboard', 30, 45.00),
    ('Monitor', 15, 320.00);


-- 1. БАЗОВЫЕ ТРАНЗАКЦИИ (BEGIN, COMMIT, ROLLBACK)
-- 1.1 Успешный перевод
BEGIN;
UPDATE accounts SET balance = balance - 100 WHERE id = 1;
UPDATE accounts SET balance = balance + 100 WHERE id = 2;
COMMIT;

-- 1.2 Перевод с откатом
BEGIN;
UPDATE accounts SET balance = balance - 2000 WHERE id = 1;
ROLLBACK;
-- Пояснение: откатываем все изменения, баланс не изменился.

-- 1.3 Транзакция с SAVEPOINT (частичный откат)
BEGIN;
UPDATE accounts SET balance = balance - 50 WHERE id = 3;
SAVEPOINT before_second_update;
UPDATE accounts SET balance = balance - 200 WHERE id = 3;
ROLLBACK TO SAVEPOINT before_second_update;
COMMIT;
-- Пояснение: первое обновление зафиксировалось, второе откатилось.

--- 2. FOR UPDATE (ПЕССИМИСТИЧНАЯ БЛОКИРОВКА)
-- 2.1 Блокировка одной строки
BEGIN;
SELECT balance FROM accounts WHERE id = 1 FOR UPDATE;
-- Другие транзакции не могут изменить строку id=1
UPDATE accounts SET balance = balance - 100 WHERE id = 1;
COMMIT;
-- Пояснение: пока мы не коммитим, строка заблокирована.

-- 2.2 Блокировка нескольких строк
BEGIN;
SELECT balance FROM accounts WHERE id IN (1, 2) FOR UPDATE;
UPDATE accounts SET balance = balance - 50 WHERE id = 1;
UPDATE accounts SET balance = balance - 50 WHERE id = 2;
COMMIT;

-- Пояснение: блокируем сразу несколько строк.

-- 2.3 NOWAIT — не ждать
BEGIN;
SELECT balance FROM accounts WHERE id = 1 FOR UPDATE NOWAIT;
-- Если строка заблокирована → ошибка "could not obtain lock"
COMMIT;
-- Пояснение: сразу ошибка, а не ожидание.

-- 2.4 LIMIT + FOR UPDATE (для пагинации с блокировкой)
BEGIN;
SELECT * FROM accounts ORDER BY id LIMIT 10 FOR UPDATE;
-- Блокируем первые 10 строк для обработки
COMMIT;
-- Пояснение: позволяет обрабатывать пачки строк.

-- 2.5 FOR UPDATE OF — блокировка только указанных таблиц
BEGIN;
SELECT a.balance, o.quantity
FROM accounts a
JOIN orders o ON a.id = o.user_id
WHERE a.id = 1
FOR UPDATE OF a; -- блокируем только accounts, orders не блокируем
COMMIT;
-- Пояснение: полезно при JOIN, чтобы не блокировать лишние таблицы.

--- 3. SKIP LOCKED (ОЧЕРЕДЬ ЗАДАЧ)
-- 3.1 Базовый воркер с SKIP LOCKED
BEGIN;
UPDATE tasks
SET status = 'processing', assigned_to = 'worker_1'
WHERE id = (
  SELECT id
  FROM tasks
  WHERE status = 'pending'
  ORDER BY id
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
RETURNING *;
COMMIT;
-- Пояснение: если задача уже заблокирована — берём следующую.

-- 3.2 Массовая обработка с SKIP LOCKED
BEGIN;
UPDATE tasks
SET status = 'processing', assigned_to = 'worker_1'
WHERE id IN (
    SELECT id
    FROM tasks
    WHERE status = 'pending'
    ORDER BY id
    LIMIT 5
    FOR UPDATE SKIP LOCKED
)
RETURNING *;
COMMIT;
-- Пояснение: берём сразу 5 задач.

-- 3.3 SKIP LOCKED с приоритетом
BEGIN;
UPDATE tasks
SET status = 'processing', assigned_to = 'worker_1'
WHERE id = (
  SELECT id
  FROM tasks
  WHERE status = 'pending'
  ORDER BY status DESC, id
  LIMIT 1
  FOR UPDATE SKIP LOCKED
)
RETURNING *;
COMMIT;
--Пояснение: сначала задачи с высоким приоритетом.

-- 3.4 SKIP LOCKED + RETURNING для логирования
BEGIN;
UPDATE tasks
SET status = 'processing', assigned_to = 'worker_1'
WHERE id = (
    SELECT id
    FROM tasks
    WHERE status = 'pending'
    ORDER BY id
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING id, task_data, status;
COMMIT;
-- Пояснение: RETURNING возвращает обновлённую строку.

--- 4. ОПТИМИСТИЧНАЯ БЛОКИРОВКА (VERSIONING)
-- 4.1 Базовое обновление с версией
BEGIN;
SELECT balance, version FROM accounts WHERE id = 1; --version = 1
UPDATE accounts
SET balance = balance - 100, version = version + 1
WHERE id = 1 AND version = 1;
-- Если обновлено 0 строк → конфликт
COMMIT;
-- Пояснение: только одна транзакция может обновить строку.

-- 4.2 Проверка через RETURNING
BEGIN;
UPDATE accounts
SET balance = balance - 100, version = version + 1
WHERE id = 1 AND version = 1
RETURNING *;
COMMIT;
-- Пояснение: если строка не обновилась → RETURNING не вернёт строки.

-- 4.3 Оптимистичная блокировка для заказа товара
BEGIN;
SELECT stock, version FROM products WHERE id = 1; -- version=1
UPDATE products
SET stock = stock - 3, version = version + 1
WHERE id = 1 AND version = 1;
-- Если конфликт → повторяем транзакцию
COMMIT;
-- Пояснение: проверяем, что остаток не изменился.

--- 5. УРОВНИ ИЗОЛЯЦИИ
-- 5.1 Установка уровня READ COMMITTED (по умолчанию)
SET TRANSACTION ISOLATION LEVEL READ COMMITTED;
BEGIN;
SELECT balance FROM accounts WHERE id = 1;
-- Другая транзакция обновила баланс и закоммитила
SELECT balance FROM accounts WHERE id = 1; -- новое значение (если закоммитили)
COMMIT;
-- Пояснение: видим изменения других транзакций после их COMMIT.

-- 5.2 REPEATABLE READ (snapshot)
SET TRANSACTION ISOLATION LEVEL REPEATABLE READ;
BEGIN;
SELECT balance FROM accounts WHERE id = 1;
-- Другая транзакция обновила баланс и закоммитила
SELECT balance FROM accounts WHERE id = 1; -- старое значение (snapshot)
COMMIT;
-- Пояснение: транзакция видит данные на момент своего начала.

-- 5.3 SERIALIZABLE (самый строгий)
SET TRANSACTION ISOLATION LEVEL SERIALIZABLE;
BEGIN;
SELECT balance FROM accounts WHERE id = 1;
-- Другая транзакция обновила ту же строку
-- Одна из транзакций получит ошибку "could not serialize access"
COMMIT;
-- Пояснение: предотвращает все аномалии, но может падать с ошибкой.

-- 5.4 Демонстрация Non-Repeatable Read (READ COMMITTED)
-- Сессия 1 (READ COMMITTED)
BEGIN;
SELECT balance FROM accounts WHERE id = 1; -- 1000

-- Сессия 2
UPDATE accounts SET balance = 900 WHERE id = 1;
COMMIT;

-- Сессия 1 (повторное чтение)
SELECT balance FROM accounts WHERE id = 1; -- 900 (отличается!)
COMMIT;
-- Пояснение: в READ COMMITTED данные могут измениться между чтениями.

-- 5.5 Демонстрация Non-Repeatable Read (REPEATABLE READ)
-- Сессия 1 (REPEATABLE READ)
BEGIN;
SELECT balance FROM accounts WHERE id = 1; -- 1000

-- Сессия 2
UPDATE accounts SET balance = 900 WHERE id = 1;
COMMIT;

-- Сессия 1 (повторное чтение)
SELECT balance FROM accounts WHERE id = 1; -- 1000 (snapshot!)
COMMIT;
-- Пояснение: в REPEATABLE READ данные не меняются.

--6. DEADLOCK
-- 6.1 Пример deadlock (запускать параллельно)
-- Сессия 1:
BEGIN;
UPDATE accounts SET balance = balance - 100 WHERE id = 1;
UPDATE accounts SET balance = balance + 100 WHERE id = 2;
COMMIT;

-- Сессия 2 (параллельно):
BEGIN;
UPDATE accounts SET balance = balance - 50 WHERE id = 2;
UPDATE accounts SET balance = balance + 50 WHERE id = 1;
COMMIT;
-- Пояснение: обе ждут друг друга → deadlock. PostgreSQL откатит одну.

-- 6.2 Исправление deadlock (правильный порядок)
-- Сессия 1:
BEGIN;
UPDATE accounts SET balance = balance - 100 WHERE id = 1;
UPDATE accounts SET balance = balance + 100 WHERE id = 2;
COMMIT;

-- Сессия 2 (правильный порядок):
BEGIN;
UPDATE accounts SET balance = balance - 50 WHERE id = 1;
UPDATE accounts SET balance = balance + 50 WHERE id = 2;
COMMIT;
-- Пояснение: всегда обновляем в порядке возрастания id.

-- 6.3 Deadlock с несколькими таблицами
-- Сессия 1:
BEGIN;
UPDATE orders SET status = 'processing' WHERE id = 1;
UPDATE order_items SET status = 'processing' WHERE order_id = 1;
COMMIT;

-- Сессия 2 (обратный порядок):
BEGIN;
UPDATE order_items SET status = 'processing' WHERE order_id = 1;
UPDATE orders SET status = 'processing' WHERE id = 1;
COMMIT;
-- Пояснение: deadlock из-за разного порядка таблиц.
-- Всегда сначала обновлять orders, потом order_items.

--6.4. Deadlock с FOR UPDATE
-- Сессия 1
BEGIN;
SELECT * FROM accounts WHERE id = 1 FOR UPDATE;
SELECT * FROM accounts WHERE id = 2 FOR UPDATE;
UPDATE accounts SET balance = balance - 100 WHERE id = 1;
UPDATE accounts SET balance = balance + 100 WHERE id = 2;
COMMIT;

-- Сессия 2 (параллельно)
BEGIN;
SELECT * FROM accounts WHERE id = 2 FOR UPDATE;
SELECT * FROM accounts WHERE id = 1 FOR UPDATE;
UPDATE accounts SET balance = balance - 50 WHERE id = 2;
UPDATE accounts SET balance = balance + 50 WHERE id = 1;
COMMIT;

--6.5. Deadlock с FOR UPDATE NOWAIT (сразу ошибка)
-- Сессия 1
BEGIN;
SELECT * FROM accounts WHERE id = 1 FOR UPDATE;
-- Держим блокировку 10 секунд
SELECT pg_sleep(10);
COMMIT;

-- Сессия 2 (параллельно)
BEGIN;
SELECT * FROM accounts WHERE id = 1 FOR UPDATE NOWAIT;
-- Ошибка: could not obtain lock on row in relation "accounts"
COMMIT;

--6.6. Deadlock с INSERT и уникальным индексом
-- Сессия 1
INSERT INTO accounts (user_id, balance) VALUE (100, 500) ON CONFLICT (user_id)
DO UPDATE SET balance = accounts.balance + 500;
COMMIT;

-- Сессия 2 (параллельно)
BEGIN;
INSERT INTO accounts (user_id, balance) VALUE (100, 300) ON CONFLICT (user_id)
DO UPDATE SET balance = accounts.balance + 300;
COMMIT;

---7. MVCC И VACUUM
-- 7.1 Смотрим скрытые колонки (xmin, xmax)
SELECT xmin, xmax, cmin, cmax, * FROM accounts;

-- 7.2 Количество мёртвых строк (dead tuples)
SELECT relname, n_dead_tup, n_live_tup, last_vacuum
FROM pg_stat_user_tables
WHERE relname = 'accounts';

-- 7.3 Запуск VACUUM
VACUUM ANALYZE accounts;
-- Пояснение: очищает мёртвые строки, обновляет статистику.

-- 7.4 VACUUM FULL (освобождает место, блокирует таблицу)
VACUUM FULL accounts;
-- Пояснение: полная очистка, но блокирует таблицу (не использовать в проде).

-- 7.5 Включение логирования VACUUM
SET log_autovacuum_min_duration = 0;
-- Пояснение: видим, когда и что делает autovacuum.

-- 7.6 Проверка размера таблицы до и после VACUUM
SELECT pg_size_pretty(pg_total_relation_size('accounts'));
VACUUM accounts;
SELECT pg_size_pretty(pg_total_relation_size('accounts'));

---8. ПРОДВИНУТЫЕ ПАТТЕРНЫ 
-- 9.1 Массовое обновление с эксклюзивной блокировкой таблицы (LOCK TABLE)
LOCK TABLE tasks IN EXCLUSIVE MODE;
UPDATE tasks SET status = 'processing' WHERE status = 'pending';
UNLOCK TABLES;
-- Пояснение: блокируем всю таблицу для массового обновления.

-- 9.2 WITH HOLD (пассивная блокировка)
BEGIN;
DECLARE cur CURSOR WITH HOLD FOR SELECT id, balance FROM accounts;
COMMIT; -- курсор продолжает существовать
FETCH NEXT FROM cur; -- работаем с данными вне транзакции
CLOSE cur; -- закрываем, когда больше не нужен

-- 9.3 Условная блокировка (только если баланс > 0)
BEGIN;
SELECT balance FROM accounts WHERE id = 1 FOR UPDATE;
UPDATE accounts SET balance = balance - 100
WHERE id = 1 AND balance >= 100;
COMMIT;
-- Пояснение: обновляем только если баланс достаточно.

-- 9.4 SKIP LOCKED с условием (только задачи с приоритетом > 5)
BEGIN;
UPDATE tasks
SET status = 'processing'
WHERE id = (
    SELECT id
    FROM tasks
    WHERE status = 'pending' AND priority > 5
    ORDER BY priority DESC
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING *;
COMMIT;
-- Пояснение: берём задачи с приоритетом > 5.

-- 9.5 Партиционирование таблицы задач для SKIP LOCKED
-- 1. Создаем партиционированную таблицу (с правильным синтаксисом)
CREATE TABLE tasks_partitioned (
    id BIGSERIAL,
    status TEXT,
    priority INT,
    created_at TIMESTAMPTZ
) PARTITION BY RANGE (priority);

-- 2. Создаем партиции
CREATE TABLE tasks_partitioned_1 PARTITION OF tasks_partitioned
    FOR VALUES FROM (1) TO (10);
CREATE TABLE tasks_partitioned_2 PARTITION OF tasks_partitioned
    FOR VALUES FROM (10) TO (20);
CREATE TABLE tasks_partitioned_3 PARTITION OF tasks_partitioned
    FOR VALUES FROM (20) TO (30);

-- 3. Каждый воркер работает со своей партицией (через условие на priority)
-- Воркер 1:
BEGIN;
WITH locked AS (
    SELECT id FROM tasks_partitioned
    WHERE priority BETWEEN 1 AND 10
      AND status = 'pending'
    ORDER BY created_at
    LIMIT 10
    FOR UPDATE SKIP LOCKED
)
UPDATE tasks_partitioned
SET status = 'processing'
FROM locked
WHERE tasks_partitioned.id = locked.id
RETURNING *;
COMMIT;