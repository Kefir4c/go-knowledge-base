/*
ТЕМА: ПОРЯДОК ВЫПОЛНЕНИЯ SQL-ЗАПРОСА (SQL LIFECYCLE)

ТЕОРИЯ

1. ЧТО ТАКОЕ ПОРЯДОК ВЫПОЛНЕНИЯ?
   - SQL — декларативный язык: ты говоришь ЧТО получить, а не КАК.
   - База данных интерпретирует твой запрос по строгим правилам.
   - Реальный порядок операций отличается от порядка написания.

2. ПОЛНЫЙ ПОРЯДОК ВЫПОЛНЕНИЯ (логический пайплайн):
   1.  FROM        --> выбираются исходные таблицы
   2.  JOIN        --> объединяются таблицы по условию ON
   3.  WHERE       --> фильтруются строки (до группировки)
   4.  GROUP BY    --> строки группируются по указанным колонкам
   5.  HAVING      --> группы фильтруются (после группировки)
   6.  SELECT      --> вычисляются выражения, даются алиасы (AS)
   7.  ORDER BY    --> строки сортируются (можно использовать алиасы)
   8.  LIMIT / OFFSET --> ограничивается количество строк

* Это логический порядок. Физический план может отличаться, но результат должен быть эквивалентен.

Если в запросе присутсвует CTE (Common Table Expression) — это временная таблица,
которая определяется до основного запроса.
Сперва  выполняется CTE, а потом уже основной запрос.

3. КЛЮЧЕВЫЕ ПРАВИЛА (ЗАПОМНИТЬ НАВСЕГДА):
   - WHERE и HAVING выполняется ДО SELECT, поэтому в WHERE и HAVING НЕЛЬЗЯ использовать алиасы (AS).
   - Агрегатные функции (SUM, AVG, COUNT) можно использовать в SELECT, HAVING и ORDER BY,
     но НЕ в WHERE (кроме подзапросов).
   - HAVING выполняется ПОСЛЕ GROUP BY, поэтому в HAVING можно использовать агрегаты.
   - ORDER BY выполняется ПОСЛЕ SELECT, поэтому в ORDER BY МОЖНО использовать алиасы.
   - LIMIT выполняется ПОСЛЕ ORDER BY, поэтому он применяется к уже отсортированному набору.

4. ПОЧЕМУ ЭТО ВАЖНО ДЛЯ ПРОИЗВОДИТЕЛЬНОСТИ:
   - Фильтрация в WHERE уменьшает количество строк до группировки → быстрее.
   - Фильтрация в HAVING работает уже после группировки → если можно перенести условие в WHERE, делай так.
   - Большой OFFSET в LIMIT заставляет базу отсортировать все строки и отбросить первые N — это дорого.

5. РАСПРОСТРАНЁННЫЕ ОШИБКИ:
   - Ошибка 1: SELECT name, salary * 1.1 AS increased FROM employees WHERE increased > 1000;
     * Ошибка: колонка increased не существует на этапе WHERE.
     * Исправление: WHERE salary * 1.1 > 1000.

   - Ошибка 2: SELECT department_id, AVG(salary) FROM employees WHERE AVG(salary) > 5000 GROUP BY department_id;
     * Ошибка: агрегатная функция AVG в WHERE.
     * Исправление: GROUP BY department_id HAVING AVG(salary) > 5000.

   - Ошибка 3: SELECT department_id, COUNT(*) AS cnt FROM employees GROUP BY department_id ORDER BY cnt;
     * Здесь всё работает, потому что ORDER BY видит алиас cnt (после SELECT).

6. ФИШКИ ПО ПАЙПЛАЙНУ:
1: «Логический порядок vs Физический план»
Ты написал FROM → JOIN → WHERE → .... Это логический порядок, который гарантирует результат. Но физический план выполнения может быть совсем другим!
Пример:
У тебя есть запрос:
SELECT * FROM users u JOIN orders o ON u.id = o.user_id WHERE u.name = 'Alice';
Логически: сначала FROM users, потом JOIN orders, потом WHERE.

Физически (если есть индекс на name):
Сначала будет Index Scan по users.name = 'Alice' (найдётся 1 строка).
Потом по этой одной строке будет Nested Loop Join с таблицей orders по индексу orders.user_id.
Где здесь JOIN перед WHERE? Его нет! Оптимизатор переставил операции, чтобы было быстрее.

Физический план может отличаться из-за оптимизатора, он может выполнить фильтрацию до JOIN'а,
если это выгодно. Поэтому важно смотреть EXPLAIN, чтобы понять, что происходит на самом деле.

2: CTE — это не просто подзапрос, это «барьер» для оптимизатора
Уточненение по раннему выводу: «CTE выполняется до основного запроса». В PostgreSQL CTE может быть материализована
(вычислена один раз и сохранена), а может быть встроена в основной запрос (как подзапрос) — в зависимости от версии и поведения.

До PostgreSQL 12: CTE всегда материализовалась. Это было безопасно, но иногда медленно.
С PostgreSQL 12: По умолчанию CTE не материализуется, если она используется только один раз. Она встраивается в основной запрос, и оптимизатор может переставлять операции внутри неё.

Пример (где это важно):
WITH active_users AS (
    SELECT * FROM users WHERE is_active = true
)
SELECT * FROM active_users WHERE name = 'Alice';
Если CTE встроится, оптимизатор может сначала применить WHERE name = 'Alice', а потом уже is_active = true (если это выгоднее).
Если CTE материализуется — сначала создастся таблица всех активных пользователей (может быть много), а потом из неё выберут Алису.
Если нужно принудительно материализовать CTE, использую MATERIALIZED или OFFSET 0 внутри.

3: LIMIT и OFFSET — это не только про строки
Ты знаешь, что OFFSET тормозит. Но есть ещё одна причина, о которой мало кто говорит: OFFSET не гарантирует стабильности, если данные меняются.
Пример:
Ты делаешь пагинацию с OFFSET 10 LIMIT 10. Пока ты читаешь первые 10 страниц, кто-то вставляет новую запись в начало списка.
Тогда при переходе на вторую страницу ты можешь увидеть дубликат или пропустить запись, потому что сдвиг сместился.
Решение:
Использовать keyset pagination (поиск по ключу):
SELECT * FROM users WHERE id > :last_id ORDER BY id LIMIT 10;
Это стабильно, быстро и не зависит от вставок/удалений.

Кроме проблем с производительностью, OFFSET создаёт проблемы с консистентностью при пагинации в живых данных.
Желательно использовать keyset pagination (поиск по последнему значению), потому что это стабильно и быстро, особенно на больших таблицах.

6. СВЯЗЬ С GO:
   - В Go ты пишешь запросы в коде. Если используешь алиас в WHERE — получишь ошибку от драйвера.
   - Для пагинации с большим OFFSET лучше использовать "cursor" (WHERE id > last_id) вместо OFFSET.
   - Понимание порядка помогает писать эффективные запросы и объяснять их на собеседованиях.
*/
-- 0. СОЗДАНИЕ ТАБЛИЦ (НОВЫЕ)
-- Отделы
CREATE TABLE departments (
id BIGSERIAL PRIMARY KEY,
name TEXT NOT NULL,
location TEXT NOT NULL
);

-- Сотрудники
CREATE TABLE employees (
id BIGSERIAL PRIMARY KEY,
name TEXT NOT NULL,
department_id BIGINT NOT NULL REFERENCES departments(id) ON DELETE CASCADE,
hire_date DATE NOT NULL,
birth_date DATE NOT NULL,
email TEXT UNIQUE
);

-- Зарплаты (исторические записи)
CREATE TABLE salaries (
id BIGSERIAL PRIMARY KEY,
employee_id BIGINT NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
amount NUMERIC(10,2) NOT NULL,
effective_from DATE NOT NULL,
effective_to DATE
);

-- Проекты
CREATE TABLE projects (
id BIGSERIAL PRIMARY KEY,
name TEXT NOT NULL,
budget NUMERIC(15,2) NOT NULL,
start_date DATE NOT NULL,
end_date DATE,
department_id BIGINT NOT NULL REFERENCES departments(id) ON DELETE CASCADE
);

-- Задачи (связь сотрудников и проектов)
CREATE TABLE tasks (
id BIGSERIAL PRIMARY KEY,
employee_id BIGINT NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
project_id BIGINT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
hours INT NOT NULL CHECK (hours > 0),
completed BOOLEAN DEFAULT FALSE
);

-- 1. НАПОЛНЕНИЕ ТЕСТОВЫМИ ДАННЫМИ

INSERT INTO departments (name, location) VALUES
('IT', 'Moscow'),
('HR', 'Saint Petersburg'),
('Finance', 'Moscow'),
('Marketing', 'Kazan');

INSERT INTO employees (name, department_id, hire_date, birth_date, email) VALUES
('Maria', 1, '2021-03-20', '1992-08-25', 'maria@it.ru'),
('Olga', 2, '2022-06-10', '1995-03-15', 'olga@hr.ru'),
('Elena', 3, '2020-09-15', '1991-11-30', 'elena@fin.ru'),
('Sergey', 4, '2021-12-01', '1993-04-05', 'sergey@mkt.ru');

INSERT INTO salaries (employee_id, amount, effective_from, effective_to) VALUES
(1, 120000, '2020-01-15', '2021-01-14'),
(1, 140000, '2021-01-15', NULL),
(2, 110000, '2022-03-20', NULL),
(3, 150000, '2019-07-01', '2020-06-30'),
(3, 170000, '2020-07-01', NULL);

INSERT INTO projects (name, budget, start_date, end_date, department_id) VALUES
('Cloud Migration', 1000000, '2023-01-01', '2023-12-31', 1),
('Staff Training', 200000, '2023-03-01', '2023-08-31', 2),
('Budget Planning', 300000, '2022-10-01', '2023-09-30', 3);

INSERT INTO tasks (employee_id, project_id, hours, completed) VALUES
(1, 1, 200, TRUE),
(1, 2, 150, TRUE),
(2, 2, 120, FALSE),
(3, 3, 100, TRUE);

-- 2. ПРИМЕРЫ, ДЕМОНСТРИРУЮЩИЕ ПОРЯДОК ВЫПОЛНЕНИЯ

-- 2.1. ОШИБКА: ИСПОЛЬЗОВАНИЕ АЛИАСА В WHERE
-- Следующий запрос ВЫЗОВЕТ ОШИБКУ, потому что алиас total_salary не существует
-- на этапе WHERE (WHERE выполняется до SELECT).

-- SELECT
--     e.name,
--     s.amount * 1.1 AS total_salary
-- FROM employees e
-- JOIN salaries s ON e.id = s.employee_id
-- WHERE total_salary > 100000   -- ОШИБКА: total_salary не известен
-- AND s.effective_to IS NULL;

-- ПРАВИЛЬНЫЙ ВАРИАНТ (повторяем выражение в WHERE):
SELECT
    e.name,
    s.amount * 1.1 AS total_salary
FROM employees e
JOIN salaries s ON e.id = s.employee_id
WHERE s.amount * 1.1 > 100000 AND s.effective_to IS NULL;

-- 2.2. РАЗНИЦА МЕЖДУ WHERE И HAVING (с агрегатами)
-- Задача: найти отделы, в которых средняя зарплата (текущая) превышает 100000,
-- и вывести эту среднюю, а также количество сотрудников.

-- Неправильно (агрегат в WHERE):
-- SELECT department_id, AVG(s.amount) AS avg_salary, COUNT(*) AS cnt
-- FROM employees e
-- JOIN salaries s ON e.id = s.employee_id
-- WHERE s.effective_to IS NULL
--   AND AVG(s.amount) > 100000   -- ОШИБКА! агрегат в WHERE
-- GROUP BY department_id;

-- Правильно (агрегат в HAVING):
SELECT
    d.name AS department_name,
    ROUND(AVG(s.amount),2) AS avg_salary,
    COUNT(*) AS employee_count
FROM employees e
JOIN salaries s ON e.id = s.employee_id
JOIN departments d ON e.department_id = d.id
WHERE s.effective_to IS NULL
GROUP BY d.id, d.name
HAVING ROUND(AVG(s.amount),2) > 100000
ORDER BY avg_salary DESC;

-- 2.3. ИСПОЛЬЗОВАНИЕ АЛИАСА В ORDER BY (РАБОТАЕТ)
-- Здесь алиас avg_salary создаётся в SELECT и доступен в ORDER BY.
SELECT
    d.name AS department_name,
    AVG(s.amount) AS avg_salary
FROM employees e
         JOIN salaries s ON e.id = s.employee_id
         JOIN departments d ON e.department_id = d.id
WHERE s.effective_to IS NULL
GROUP BY d.id, d.name
ORDER BY avg_salary DESC;   -- алиас работает

-- 2.4. СЛОЖНЫЙ ЗАПРОС С РАЗНЫМИ ЭТАПАМИ
-- Найти проекты, в которых суммарное количество часов (по задачам) больше 200,
-- и показать название проекта, общее количество задач и бюджет проекта,
-- отсортировать по бюджету.

SELECT
    p.name AS project_name,
    COUNT(t.id) AS tack_count,
    p.budget
FROM projects p
LEFT JOIN tasks t ON p.id = t.project_id
WHERE p.end_date >= '2023-01-01'
GROUP BY p.id, p.name, p.budget
HAVING COALESCE(SUM(t.hours),0) > 200
ORDER BY p.budget DESC ;

-- 2.5. ДЕМОНСТРАЦИЯ РАЗНИЦЫ МЕЖДУ WHERE И HAVING (производительность)
-- Если мы хотим отфильтровать сотрудников по дате найма (до группировки),
-- лучше использовать WHERE, чтобы уменьшить количество строк до группировки.

-- ПЛОХО (фильтр в HAVING, хотя можно в WHERE):
-- SELECT department_id, AVG(s.amount) AS avg_salary
-- FROM employees e
-- JOIN salaries s ON e.id = s.employee_id
-- WHERE s.effective_to IS NULL
-- GROUP BY department_id
-- HAVING MIN(e.hire_date) >= '2020-01-01';   -- фильтрует группы, но сначала агрегировались все строки

-- ХОРОШО (фильтр в WHERE):
SELECT
    d.name AS department_name,
    AVG(s.amount) AS avg_salary
FROM employees e
JOIN salaries s ON e.id = s.employee_id
JOIN departments d ON e.department_id = d.id
WHERE s.effective_to IS NULL AND e.hire_date >= '2020-01-01'
GROUP BY d.id, d.name;

-- 2.6. ИСПОЛЬЗОВАНИЕ ПОДЗАПРОСА И ПОРЯДОК ВЫПОЛНЕНИЯ
-- Подзапрос выполняется как отдельный "черновик" на этапе FROM.
-- Внешний запрос применяет свой порядок выполнения к результату подзапроса.
SELECT
    dept_status.department_name,
    dept_status.avg_salary
FROM (
    SELECT
        d.name AS department_name,
        ROUND(AVG(s.amount),2) AS avg_salary
FROM employees e
JOIN salaries s ON e.id = s.employee_id
JOIN departments d ON e.department_id = d.id
WHERE s.effective_to IS NULL
GROUP BY d.id, d.name
     ) AS dept_status
WHERE dept_status.avg_salary > 100000
ORDER BY dept_status.avg_salary DESC;

-- 2.7. БОЛЬШОЙ OFFSET (ПРОБЛЕМА ПРОИЗВОДИТЕЛЬНОСТИ)
-- LIMIT и OFFSET выполняются в самом конце. Если OFFSET большой, база всё равно
-- сортирует все строки и только потом отбрасывает первые N.
-- Для демонстрации используем EXPLAIN (но без реальных данных, покажем синтаксис).
EXPLAIN (ANALYZE)
SELECT  e.name,s.amount
FROM employees e
JOIN salaries s ON e.id = s.employee_id
WHERE s.effective_to IS NULL
ORDER BY s.amount DESC
LIMIT 10 OFFSET 1000; -- база сначала отсортирует все, потом отбросит 1000
-- Вместо большого OFFSET используют "keyset pagination" (WHERE s.amount < last_amount)
-- или "seek method" (WHERE id > last_id)

-- 3. ДОПОЛНИТЕЛЬНЫЕ ПРИМЕРЫ ДЛЯ ЗАКРЕПЛЕНИЯ
-- 3.1. Найти сотрудников, у которых зарплата выше средней по отделу.
-- Здесь используется коррелированный подзапрос, который выполняется на этапе SELECT? Нет,
-- подзапрос в WHERE выполняется на этапе WHERE, но он не использует алиасы.
SELECT e1.name, e1.department_id, s1.amount
FROM employees e1
JOIN salaries s1 ON e1.id = s1.employee_id
WHERE s1.effective_to IS NULL
AND s1.amount > (
    SELECT AVG(s2.amount)
    FROm employees e2
    JOIN salaries s2 ON e2.id = s2.employee_id
    WHERE s2.effective_to IS NULL
    AND e2.department_id = e1.department_id
    )
ORDER BY e1.department_id, s1.amount DESC;

-- 3.2. Группировка по нескольким колонкам (отдел + год найма).
-- Покажем, как порядок выполнения влияет на доступность колонок.
SELECT
    d.name AS department_name,
    EXTRACT(YEAR  FROM e.hire_date) AS hire_year,
    COUNT(*) AS employees_count,
    ROUND(AVG(s.amount),0) AS avg_salary
FROM employees e
JOIN departments d ON e.department_id = d.id
JOIN salaries s ON e.id = s.employee_id
WHERE s.effective_to IS NULL
GROUP BY d.id, d.name, EXTRACT(YEAR  FROM e.hire_date)
ORDER BY department_name, hire_year;
-- Здесь выражение EXTRACT(YEAR FROM hire_date) повторяется в GROUP BY, потому что
-- на этапе GROUP BY алиас hire_year ещё не существует. Но в ORDER BY уже можно использовать hire_year.

-- 3.3. HAVING с сложным условием (подзапрос в HAVING).
-- Найти отделы, у которых средняя зарплата больше, чем средняя зарплата по всем отделам.
SELECT
    d.name AS depatment_name,
    AVG(s.amount) AS avg_salary
FROM employees e
JOIN salaries s ON e.id = s.employee_id
JOIN departments d ON e.department_id = d.id
WHERE s.effective_to IS NULL
GROUP BY d.id, d.name
HAVING AVG(amount) > (SELECT AVG(s2.amount) FROM employees e2
                      JOIN salaries s2 ON e2.id = s2.employee_id
                      WHERE s2.effective_to IS NULL);

-- 3.4. Использование алиаса в ORDER BY вместе с DISTINCT (DISTINCT выполняется на этапе SELECT).
-- DISTINCT применяется до ORDER BY, поэтому алиас виден.

SELECT DISTINCT
    d.name AS department_name,
    EXTRACT(YEAR FROM e.hire_date) AS hire_year
FROM employees e
         JOIN departments d ON e.department_id = d.id
ORDER BY department_name, hire_year;

-- 4. КОНТРОЛЬНЫЙ ЗАПРОС (ВСЁ ВМЕСТЕ)
-- Найти проекты, в которых участвовало больше 2 сотрудников,
-- вывести название проекта, количество сотрудников, среднее число часов на сотрудника,
-- и отсортировать по убыванию среднего числа часов.
SELECT
    p.name AS project_name,
    COUNT(DISTINCT t.employee_id) AS employee_count,
    AVG(t.hours) AS avg_hours
FROM projects p
JOIN tasks t ON p.id = t.project_id
WHERE p.start_date >= '2022-01-01'
GROUP BY p.id, p.name
HAVING COUNT(DISTINCT t.employee_id) > 2
ORDER BY avg_hours DESC;