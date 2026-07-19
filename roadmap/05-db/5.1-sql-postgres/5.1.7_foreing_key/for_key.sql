/*
ШАГ 1.7: СВЯЗИ ТАБЛИЦ (FOREIGN KEY)

ТЕОРИЯ

1. ЗАЧЕМ НУЖНЫ СВЯЗИ?
   - В реальных приложениях данные не хранятся в одной таблице.
   - Например: пользователь и его заказы. Если хранить всё в одной таблице, данные будут дублироваться (имя пользователя будет повторяться в каждом заказе).
   - Это приводит к избыточности, аномалиям при обновлении и увеличению размера базы.
   - Решение: разделить данные на отдельные таблицы и связать их через внешние ключи (FOREIGN KEY).

2. ВИДЫ СВЯЗЕЙ:
   - Один-к-одному (One-to-One): редкая связь, например, пользователь и его паспортные данные.
   - Один-ко-многим (One-to-Many): самый частый случай. Один пользователь может иметь много заказов.
   - Многие-ко-многим (Many-to-Many): требует связующей таблицы. Например, товары и заказы (один заказ может содержать много товаров, и один товар может быть во многих заказах).

3. FOREIGN KEY (внешний ключ):
   - Ограничение, которое обеспечивает ссылочную целостность.
   - Гарантирует, что значение в колонке (или колонках) существует в первичном ключе другой таблицы.
   - Синтаксис: FOREIGN KEY (child_column) REFERENCES parent_table(parent_column).

4. СТРАТЕГИИ ПРИ УДАЛЕНИИ (ON DELETE):
   - ON DELETE CASCADE: при удалении записи из родительской таблицы автоматически удаляются все связанные записи в дочерней.
   - ON DELETE RESTRICT (или NO ACTION): запрещает удаление записи из родительской таблицы, если есть связанные записи в дочерней.
   - ON DELETE SET NULL: при удалении родительской записи внешний ключ в дочерней таблице устанавливается в NULL (если колонка допускает NULL).
   - ON DELETE SET DEFAULT: при удалении родительской записи внешний ключ в дочерней таблице устанавливается в значение по умолчанию.

4.1. Если в родительской таблице мы добавим конструкцию ON DELETE SET NULL, а в связанном поле дочерней таблице будет указан параметр с NOT NULL.
То при удалении позиции из родительской таблице, у нас будет ошибка, т.к. постгрес не сможет записать значения NULL в NOT NULL

5. ПРИНЦИП РАБОТЫ:
   - При вставке или обновлении дочерней записи проверяется существование родительской записи.
   - Если родительской записи нет — операция отклоняется.
   - Это защищает базу от «сиротских» записей.

6. ИНДЕКСЫ НА ВНЕШНИЕ КЛЮЧИ:
   - PostgreSQL автоматически создаёт индекс на колонку внешнего ключа, если его нет.
   - Это ускоряет операции JOIN и проверки целостности.

7. СИНТАКСИС ЗАПИСЕЙ:
1. order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE — это короткая запись для одной колонки.

2. FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT —
это полная запись, которая нужна для сложных случаев и считается более хорошей практикой, так как позволяет давать имена ограничениям.

8. СВЯЗЬ С GO:
   - В Go-структурах это отражается как вложенные структуры или отдельные поля.
   - Например, Order содержит UserID (int64) и поле User *User (при загрузке связанных данных).
   - При работе с БД обычно выполняют отдельные запросы для получения родительской и дочерних записей, либо используют JOIN.

9. ПРИМЕР РЕАЛЬНОЙ СХЕМЫ:
   - users (id, name, email)
   - orders (id, user_id, amount, created_at)
   - order_items (id, order_id, product_id, quantity, price)
   - products (id, name, price, stock)
*/

-- 1. СОЗДАНИЕ ТАБЛИЦЫ users

CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    email TEXT UNIQUE NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- 2. СОЗДАНИЕ ТАБЛИЦЫ orders С ВНЕШНИМ КЛЮЧОМ

CREATE TABLE orders (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    amount NUMERIC(10,2) NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
-- Используем CASCADE для примера, позже покажем RESTRICT
);

-- 3. ВСТАВКА ТЕСТОВЫХ ДАННЫХ

-- Добавляем пользователей
INSERT INTO users (name, email) VALUES
    ('Alice', 'alice@mail.com'),
    ('Bob', 'bob@mail.com'),
    ('Charlie', 'charlie@mail.com');

-- Добавляем заказы (принадлежат пользователям)
INSERT INTO orders (user_id, amount) VALUES
    (1, 100.50),
    (1, 200.00),
    (2, 50.25),
    (3, 300.00);

-- 4. ПРОВЕРКА ДАННЫХ
-- Все пользователи
SELECT * FROM users;

-- Все заказы
SELECT * FROM orders;

-- Заказы с именами пользователей (JOIN)
SELECT u.name, o.amount, o.created_at
FROM users u
JOIN orders o ON u.id = o.user_id
ORDER BY u.name, o.created_at;


-- 5. ПРИМЕРЫ С ГРАДАЦИЕЙ
-- УРОВЕНЬ 1: Базовые операции с внешним ключом

-- 5.1. Вставка корректного заказа (существующий user_id)
INSERT INTO orders (user_id, amount) VALUES (1, 75.00);

-- 5.2. Попытка вставить заказ с несуществующим user_id (ошибка)
-- INSERT INTO orders (user_id, amount) VALUES (999, 10.00); -- ОШИБКА!

-- 5.3. Выборка заказов конкретного пользователя
SELECT * FROM orders
WHERE user_id = 1;

-- 5.4. Выборка пользователей без заказов (LEFT JOIN)
SELECT u.*
FROM users u
LEFT JOIN orders o ON u.id = o.user_id
WHERE o.id IS NULL;

-- УРОВЕНЬ 2: Стратегии ON DELETE, обновление

-- 5.5. Создаём таблицу с ON DELETE RESTRICT
DROP TABLE IF EXISTS orders_restrict CASCADE;
CREATE TABLE IF NOT EXISTS orders_restrict (
 id BIGSERIAL PRIMARY KEY,
 user_id BIGINT NOT NULL,
 amount NUMERIC(10,2) NOT NULL,
 FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE RESTRICT
);

INSERT INTO orders_restrict (user_id, amount) VALUES (1, 50.00);

-- 5.6. Попытка удалить пользователя, у которого есть заказы в orders_restrict
-- DELETE FROM users WHERE id = 1; -- ОШИБКА! (RESTRICT)

-- 5.7. Удаление пользователя с CASCADE (orders удалятся)
DELETE FROM users WHERE id = 2;
SELECT * FROM users;

-- 5.8. Создание таблицы с ON DELETE SET NULL (если колонка допускает NULL)
DROP TABLE IF EXISTS orders_set_null CASCADE;
CREATE TABLE orders_set_null(
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT,
    amount NUMERIC(10,2) NOT NULL,
 FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

INSERT INTO orders_set_null (user_id, amount) VALUES (1, 30.00);
DELETE FROM users WHERE id = 1;
SELECT * FROM orders_set_null; -- user_id стал NULL

-- УРОВЕНЬ 3 (SENIOR): Комбинации с несколькими таблицами, каскадное обновление

-- 5.9. Многотабличная связь: создаём товары и заказы-товары
DROP TABLE IF EXISTS orders_item CASCADE;
DROP TABLE IF EXISTS products CASCADE;

CREATE TABLE products(
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    price NUMERIC(10,2) NOT NULL
);

CREATE TABLE order_items (
    id BIGSERIAL PRIMARY KEY,
    order_id BIGINT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id BIGINT NOT NULL REFERENCES products(id) ON DELETE RESTRICT,
    quantity INT NOT NULL CHECK ( quantity > 0 ),
    price NUMERIC(10,2) NOT NULL -- цена на момент заказа
);

INSERT INTO products (name, price) VALUES
    ('Laptop', 1200.00),
    ('Mouse', 25.99),
    ('Keyboard', 45.00);

INSERT INTO orders (user_id, amount) VALUES (3, 100.00); -- order_id = 5 (например)
INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
    (5, 1, 1, 1200.00),
    (5, 2, 2, 25.99);

-- 5.10. Выборка заказа с товарами (JOIN нескольких таблиц)
SELECT o.id AS order_id, u.name AS user_name, p.name AS products_name, oi.quantity, oi.price
FROM orders o
JOIN users u ON o.user_id = u.id
JOIN order_items oi ON o.id = oi.order_id
JOIN products p ON oi.product_id = p.id
WHERE o.id = 5;

-- 5.11. Попытка удалить продукт, используемый в заказе (RESTRICT)
-- DELETE FROM products WHERE id = 1; -- ОШИБКА! (есть связь в order_items)

-- 5.12. Каскадное удаление заказа (удалит и order_items)
DELETE FROM orders WHERE id = 5;
SELECT * FROM order_items WHERE order_id = 5; -- пусто

--ЗАДАНИЕ 1: Топ-5 продуктов по среднему рейтингу

-- Предполагается, что таблицы users и products уже существуют.
-- Удаляем старую таблицу, если есть:
DROP TABLE IF EXISTS reviews CASCADE;

CREATE TABLE reviews(
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL,
    product_id BIGINT NOT NULL,
    rating SMALLINT NOT NULL CHECK (rating BETWEEN 1 AND 5),
    text TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE
);
-- Создаём индексы для ускорения запросов
CREATE INDEX idx_reviews_product_id ON reviews(product_id);
CREATE INDEX idx_reviews_user_id ON reviews(user_id);

-- 2. ДОБАВЛЯЕМ ТЕСТОВЫЕ ДАННЫЕ

-- Добавляем ещё несколько продуктов, если их мало
INSERT INTO products (name, price) VALUES
    ('Smartphone', 800.00),
    ('Tablet', 350.00),
    ('Headphones', 150.00),
    ('Monitor', 450.00),
    ('Keyboard', 120.00)
ON CONFLICT (id) DO NOTHING;

-- Добавляем отзывы (пользователи должны существовать; предположим, что есть пользователи с id 1,2,3)

INSERT INTO reviews (user_id, product_id, rating, text) VALUES
    (1, 1, 5, 'Отличный товар!'),
    (1, 2, 4, 'Хороший, но дороговат'),
    (2, 1, 4, 'Неплохой'),
    (2, 3, 5, 'Лучшие наушники'),
    (3, 1, 3, 'Средний'),
    (3, 4, 5, 'Супер монитор'),
    (1, 4, 4, 'Хороший монитор'),
    (2, 4, 5, 'Отличный монитор'),
    (3, 5, 2, 'Не очень клавиатура'),
    (1, 5, 3, 'Нормально'),
    (2, 2, 4, 'Удобный планшет'),
    (3, 3, 5, 'Шикарные наушники'),
    (1, 3, 4, 'Хорошие наушники');

-- 3. ЗАПРОС: ТОП-5 ПРОДУКТОВ ПО СРЕДНЕМУ РЕЙТИНГУ

SELECT
    p.id AS product_id,
    p.name AS product_name,
    ROUND(AVG(r.rating), 2) AS avg_rating,
    COUNT(r.id) AS review_count
FROM products p
JOIN reviews r ON p.id = r.product_id
GROUP BY p.id, p.name
HAVING COUNT(r.id) >= 2    -- исключаем продукты с малым числом отзывов (опционально)
ORDER BY avg_rating DESC, review_count DESC
LIMIT 5;

--ЗАДАЧА: Топ-5 преподавателей по средней оценке студентов

-- 1. СОЗДАНИЕ ТАБЛИЦ
-- ===========================================================================

DROP TABLE IF EXISTS grades CASCADE;
DROP TABLE IF EXISTS courses CASCADE;
DROP TABLE IF EXISTS students CASCADE;
DROP TABLE IF EXISTS teachers CASCADE;

-- Преподаватели
CREATE TABLE teachers (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

-- Курсы (у каждого преподавателя может быть несколько курсов)
CREATE TABLE courses (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL,
    teacher_id BIGINT NOT NULL,
    FOREIGN KEY (teacher_id) REFERENCES teachers(id) ON DELETE CASCADE
);

-- Студенты
CREATE TABLE students (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL
);

-- Оценки (связь студента, курса и оценки)
CREATE TABLE grades (
    id BIGSERIAL PRIMARY KEY,
    student_id BIGINT NOT NULL,
    course_id BIGINT NOT NULL,
    grade INT NOT NULL CHECK (grade BETWEEN 1 AND 5),
    FOREIGN KEY (student_id) REFERENCES students(id) ON DELETE CASCADE,
    FOREIGN KEY (course_id) REFERENCES courses(id) ON DELETE CASCADE
);

-- Индексы для ускорения
CREATE INDEX idx_grades_course_id ON grades(course_id);
CREATE INDEX idx_courses_teacher_id ON courses(teacher_id);

-- 2. ЗАПОЛНЕНИЕ ТЕСТОВЫМИ ДАННЫМИ

-- Преподаватели
INSERT INTO teachers (name) VALUES
    ('Иван Петров'),
    ('Мария Смирнова'),
    ('Алексей Иванов'),
    ('Елена Кузнецова'),
    ('Дмитрий Соколов');

-- Курсы (каждый преподаватель ведёт 2-3 курса)
INSERT INTO courses (name, teacher_id) VALUES
    ('Математика', 1),
    ('Физика', 1),
    ('Программирование', 2),
    ('Базы данных', 2),
    ('Алгоритмы', 3),
    ('Дискретная математика', 3),
    ('История', 4),
    ('Философия', 4),
    ('Английский', 5),
    ('Немецкий', 5);

-- Студенты
INSERT INTO students (name) VALUES
    ('Анна'), ('Борис'), ('Виктор'), ('Галина'), ('Дмитрий');

-- Оценки (случайные, но с некоторой тенденцией)
INSERT INTO grades (student_id, course_id, grade) VALUES
    (1, 1, 5), (1, 2, 4), (1, 3, 5), (1, 4, 4), (1, 5, 3),
    (2, 1, 4), (2, 2, 5), (2, 3, 4), (2, 6, 4), (2, 7, 3),
    (3, 3, 5), (3, 4, 5), (3, 5, 4), (3, 8, 4), (3, 9, 4),
    (4, 1, 3), (4, 2, 3), (4, 5, 4), (4, 6, 5), (4, 10, 5),
    (5, 3, 4), (5, 4, 3), (5, 7, 5), (5, 8, 5), (5, 9, 4);

-- 3. ЗАПРОС: ТОП-5 ПРЕПОДАВАТЕЛЕЙ ПО СРЕДНЕЙ ОЦЕНКЕ

WITH teacher_stats AS (
    SELECT
        t.id AS teacher_id,
        t.name AS teacher_name,
        AVG(g.grade) AS avg_grade,
        COUNT(g.id) AS grade_count,
        COUNT(DISTINCT c.id) AS course_count
    FROM teachers t
        JOIN courses c ON t.id = c.teacher_id
        JOIN grades g ON c.id = g.course_id
    GROUP BY t.id, t.name
    HAVING COUNT(DISTINCT c.id) >= 2   -- только преподаватели с >=2 курсами
)
SELECT
    teacher_id,
    teacher_name,
    ROUND(avg_grade, 2) AS avg_grade,
    grade_count,
    course_count
FROM teacher_stats
ORDER BY avg_grade DESC, grade_count DESC
LIMIT 5;

-- 4. ДОПОЛНИТЕЛЬНЫЕ ЗАПРОСЫ (ДЛЯ ЗАКРЕПЛЕНИЯ)

-- 4.1. Средняя оценка по каждому курсу (для сравнения)
SELECT
    c.name AS course_name,
    t.name AS teacher_name,
    ROUND(AVG(g.grade), 2) AS avg_grade,
    COUNT(g.id) AS grade_count
FROM courses c
         JOIN teachers t ON c.teacher_id = t.id
         JOIN grades g ON c.id = g.course_id
GROUP BY c.id, c.name, t.name
ORDER BY avg_grade DESC;

-- 4.2. Студенты с наибольшим средним баллом
SELECT
    s.name AS student_name,
    ROUND(AVG(g.grade), 2) AS avg_grade
FROM students s
         JOIN grades g ON s.id = g.student_id
GROUP BY s.id, s.name
ORDER BY avg_grade DESC
LIMIT 5;