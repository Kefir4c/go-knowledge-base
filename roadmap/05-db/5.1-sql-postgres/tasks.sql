--В этом файле я буду решать задачи sql.
-- Буду брать задачи и решать их из разных сайтов, ИИ и т.п.

/*
 «Удаление дубликатов с приоритетом»
Условие:
В таблице users есть дубли по полю email. Нужно оставить только одну запись на каждый email — ту,
у которой самый поздний created_at. Если даты совпадают, то с наибольшим id.
Напиши запрос, который удаляет дубли, оставляя одну запись на email.
Таблица: users (id, email, name, created_at)
 */
WITH to_keep AS(
    SELECT DISTINCT ON (email)
        id
    FROM users
    ORDER BY email, created_at DESC, id DESC
)
DELETE FROM users
WHERE id NOT IN (SELECT id FROM to_keep);

/*
  «Воркер с SKIP LOCKED и возвратом данных»
Условие:
Таблица tasks (id, status, priority, created_at).
Нужно написать запрос для воркера, который:

Захватывает 10 задач со статусом pending.
Сортирует по приоритету (убывание) и дате создания.
Пропускает уже заблокированные строки (SKIP LOCKED).
Обновляет их статус на processing и возвращает обновлённые записи.
 */
WITH locked_tasks AS(
    SELECT id
    FROM tasks
    WHERE status = 'pending'
    ORDER BY priority DESC, created_at
    LIMIT 10
    FOR UPDATE SKIP LOCKED
)
UPDATE tasks
SET status = 'processing'
FROM locked_tasks
WHERE tasks.id = locked_tasks.id
RETURNING*;

/*
 «Скользящее среднее за 7 дней»
Условие:
Таблица sales с колонками sale_date (date) и amount (numeric).
Напиши запрос, который для каждой даты показывает сумму продаж за этот день и скользящее среднее за
последние 7 дней (включая текущий). Если данных за предыдущие дни нет – среднее считается по доступным дням.
Результат: sale_date, daily_sum, moving_avg_7
 */
WITH daily AS (
    SELECT
        sale_date,
        SUM(amount) AS daily_sum
    FROM sales
    GROUP BY sale_date
)
SELECT
    sale_date,
    daily_sum,
    AVG(daily_sum) OVER (ORDER BY sale_date ROWS BETWEEN 6 PRECEDING AND CURRENT ROW) AS moving_avg_7
FROM daily
ORDER BY sale_date;

/*
 Поиск аномальных скачков продаж»
Условие:
Таблица sales с полями sale_date и amount.
Напиши запрос, который находит дни, когда сумма продаж была более чем в 2 раза выше,
 чем в предыдущий день. Выведи эти даты, текущую сумму и предыдущую.

Результат: sale_date, current_day_sales, previous_day_sales, growth_ratio
 */
 WITH daily AS(
     SELECT
        sale_date,
        SUM(amount) AS sales
     FROM sales
     GROUP BY sale_date
 )
 SELECT
    sale_date,
    sales AS current_day_sales,
    LAG(sales) OVER (ORDER BY sale_date) AS previous_day_sales,
    ROUND(sales::DECIMAL/ NULLIF(LAG(sales) OVER (ORDER BY sale_date),0),2) AS growth_ratio
 FROM daily
 WHERE sales > 2 * COALESCE(LAG(sales) OVER (ORDER BY sale_date), 0)
 ORDER BY sale_date;

/*
 Есть таблица rates с курсами валют:
sql
CREATE TABLE rates (
    curr_id INT,        -- ID валюты
    date_rate DATE,     -- дата установки курса
    rate NUMERIC        -- значение курса
);
 Курс устанавливается не на каждый день, а действует до следующего изменения. Например:
curr_id	date_rate	rate
1	2025-01-01	90.0
1	2025-01-10	92.5
2	2025-01-05	1.10
Нужно для валюты 1 на дату 2025-01-07 получить действующий курс (90.0), а для валюты 1 на 2025-01-10 — уже 92.5.
Напиши запрос, который для заданной валюты и даты вернёт актуальный курс.
*/
SELECT rate
FROM rates
WHERE curr_id = 1
  AND date_rate <= '2025-01-07'
ORDER BY date_rate DESC
LIMIT 1;

/*
 «Топ-3 самых дорогих заказа клиента»
Условие:
На собеседовании в крупную компанию могут дать задачу «найди топ-3 самых дорогих заказа клиента».

Таблицы:
users (user_id, name)
orders (order_id, user_id, total_amount, order_date)

Напиши запрос, который для каждого пользователя выводит три его самых дорогих заказа (по total_amount).
Если у пользователя меньше трёх заказов — выведи все, которые есть. Отсортируй результат по имени пользователя и сумме заказа по убыванию.
*/
WITH ranked_orders AS(
    SELECT
        u.user_id,
        u.name,
        o.order_id,
        o.total_amount,
        ROW_NUMBER() OVER (PARTITION BY u.user_id ORDER BY o.total_amount DESC) AS rnk
    FROM users
    JOIN order o ON u.user_id = o.user_id
)
SELECT
    user_id,
    name,
    order_id,
    total_amount,
    rnk AS rank
FROM ranked_orders
WHERE rnk <= 3
ORDER BY name, rank DESC;

/*
 «Обновление курса с привязкой к дате»
Условие:
Продолжаем тему валют. Есть таблица rates (из задачи №1). Нужно обновить курс для валюты 1 на текущую дату
(например, на текущ день установить курс 93.0). Если на эту дату уже есть запись — обновить её. Если нет — вставить новую.

Напиши один SQL-запрос, который делает это атомарно (без отдельной проверки SELECT).
*/
INSERT INTO rates (curr_id, date_rate, rate)
VALUES (1, CURRENT_DATE, 93)
ON CONFLICT (curr_id,date_rate)
DO UPDATE SET rate = excluded.rate
RETURNING *;

/*
 «Поиск аномальных скачков продаж»
Условие:
Таблица sales с полями sale_date (date) и amount (numeric).
Напиши запрос, который для каждого дня вычисляет:
продажи за текущий день,
продажи за предыдущий день,
разницу в процентах между ними.

Выведи только те дни, где продажи выросли более чем на 50% по сравнению с предыдущим днём.
Результат: sale_date, current_day_sales, prev_day_sales, pct_change
 */
WITH daily_sales AS(
    SELECT
        sale_date,
        SUM(amount) AS total
    FROM sales
    ORDER BY sale_date DESC
),
with_prev AS (
    SELECT
        sale_date,
        total AS current_day_sales,
        LAG(total) over (ORDER BY sale_date DESC) AS prev_day_sales
)
SELECT
    sale_date,
    current_day_sales,
    prev_day_sales,
    ROUND(
        (current_day_sales - prev_day_sales):: NUMERIC / NULLIF(prev_day_sales, 0) * 100,2
    ) AS pct_change
FROM with_prev
WHERE prev_day_sales IS NOT NULL
  AND (current_day_sales - prev_day_sales)::NUMERIC / NULLIF(prev_day_sales, 0) * 100 > 50
ORDER BY sale_date;

/*
 «Дедупликация с сохранением последней записи»
Условие:
Таблица logs с полями id, user_id, action, created_at.
В таблице есть дубли по (user_id, action). Нужно оставить только последнюю запись
(с максимальным created_at) для каждой пары (user_id, action).

Напиши запрос, который удаляет все дубли, оставляя одну запись на каждую пару.
 */
 WITH ranked_logs AS(
     SELECT
         id,
         user_id,
         action,
         created_at,
         ROW_NUMBER() OVER (PARTITION BY user_id ORDER BY created_at DESC) AS rn
     FROM logs
 )
DELETE FROM logs
WHERE id IN(
     SELECT id
     FROM ranked_logs
     WHERE rn > 1
     );

/*
 «Оптимизация через частичный индекс»
Условие:
Таблица orders с полями id, status, created_at, total_amount.
Частый запрос:
SELECT id, created_at, total_amount
FROM orders
WHERE status = 'pending'
  AND created_at > NOW() - INTERVAL '7 days'
ORDER BY created_at;

Напиши индекс, который ускорит этот запрос. Объясни, почему ты выбрал именно такой индекс, и как проверить, что он работает.
 */
 CREATE INDEX idx_orders_created_status ON orders (created_at,status)