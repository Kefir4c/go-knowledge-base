/*
Топ-3 товара по категориям
Условие:
Есть таблицы:

products (product_id, name, category_id)

categories (category_id, category_name)

order_items (order_id, product_id, quantity, price, order_date)

Нужно для каждой категории вывести три товара с самой большой общей выручкой
 (quantity * price) за последний квартал (3 месяца от текущей даты). 
 Если в категории меньше 3 товаров — вывести все.

Результат:
category_name, product_name, total_revenue, rank_in_category
*/

WITH product_revenue AS(
    SELECT
        p.product_id,
        p.name AS product_name,
        p.category_id,
        c.category_name,
        SUM(oi.quantity * io.price) AS total_revenue
    FROM products p
    JOIN order_items oi ON p.product_id = oi.product_id
    JOIN categories c ON p.category_id = c.category_id    
    WHERE io.order_date >= NOW() - INTERVAL '3 mounths'
    GROUP BY p.product_id, p.name, p.category_id, c.category_name
),
ranked AS(
    SELECT
        category_name,
        product_name,
        total_revenue,
        ROW_NUMBER() OVER(PARTITION BY category_id ORDER BY total_revenue DESC) AS rank_in_category
        FROM product_revenue
)
SELECT
    category_name,
    product_name,
    total_revenue,
    rank_in_category
FROM ranked
WHERE rank_in_category <= 3
ORDER BY category_name, rank_in_category;     

