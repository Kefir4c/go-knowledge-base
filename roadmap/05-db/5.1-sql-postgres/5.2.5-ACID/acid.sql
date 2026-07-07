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
    - Каждая строка имеет скрытые системные колонки: xmin (ID транзакции, которая создала строку) и xmax (ID транзакции, которая удалила строку).
    - При UPDATE создаётся новая версия строки, старая помечается как удалённая (xmax).
    - Каждая транзакция видит только те версии строк, которые были закоммичены на момент её начала (snapshot).

  Это позволяет:
    - Читателям не блокировать писателей (и наоборот).
    - Получать консистентный снимок данных на момент начала транзакции.

  Плата за MVCC:
    - Старые версии строк (dead tuples) накапливаются и требуют VACUUM.
    - Размер таблицы может расти, если не выполнять VACUUM.

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


-- 2. БАЗОВЫЕ ТРАНЗАКЦИИ (BEGIN, COMMIT, ROLLBACK)	ы