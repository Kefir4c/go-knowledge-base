package __2_4_CTE

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

/*
РЕАЛИЗАЦИЯ С ОКОННЫМИ ФУНКЦИЯМИ В GO
Цель: показать, как выполнять сложные SQL-запросы с оконными функциями
из Go-кода, обрабатывать результаты и выводить их в удобном формате.
В этом примере мы реализуем отчёт "Топ-2 продукта по количеству поставок
в каждой категории" (задача 10 из практики).
*/

// ProductRank — структура для хранения результата запроса
type ProductRank struct {
	Category      string
	ProductName   string
	ShipmentCount int
	Rank          int // ранг внутри категории (1 или 2)
}

func main() {
	ctx := context.Background()

	// 1. Подключение к базе данных
	// Замените параметры на свои: user, password, host, port, dbname
	connStr := "postgres://user:password@localhost:5432/your_db?sslmode=disable"
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("Ошибка подключения к БД: %v", err)
	}
	defer pool.Close()

	fmt.Println("ТОП-2 ПРОДУКТА ПО КОЛИЧЕСТВУ ПОСТАВОК В КАЖДОЙ КАТЕГОРИИ\n")

	// 2. Определяем SQL-запрос с оконной функцией ROW_NUMBER()
	// Запрос полностью соответствует решению задачи 10.
	query := `
		WITH product_counts AS (
			SELECT product_id, COUNT(*) AS shipment_count
			FROM shipments
			GROUP BY product_id
		),
		ranked AS (
			SELECT
				p.category,
				p.name AS product_name,
				pc.shipment_count,
				ROW_NUMBER() OVER (PARTITION BY p.category ORDER BY pc.shipment_count DESC) AS rn
			FROM products p
			JOIN product_counts pc ON p.id = pc.product_id
		)
		SELECT category, product_name, shipment_count, rn
		FROM ranked
		WHERE rn <= 2
		ORDER BY category, rn;
	`

	// 3. Выполняем запрос
	rows, err := pool.Query(ctx, query)
	if err != nil {
		log.Fatalf("Ошибка выполнения запроса: %v", err)
	}
	defer rows.Close()

	// 4. Сканируем строки в слайс структур
	var report []ProductRank
	for rows.Next() {
		var pr ProductRank
		err := rows.Scan(
			&pr.Category,
			&pr.ProductName,
			&pr.ShipmentCount,
			&pr.Rank,
		)
		if err != nil {
			log.Fatalf("Ошибка сканирования строки: %v", err)
		}
		report = append(report, pr)
	}

	// 5. Выводим результаты в красивом виде
	if len(report) == 0 {
		fmt.Println("Нет данных для отображения.")
		return
	}
	fmt.Printf("Запрос выполнен, получено строк: %d\n\n", len(report))

	// Группируем вывод по категориям для наглядности
	currentCategory := ""
	for _, pr := range report {
		if pr.Category != currentCategory {
			currentCategory = pr.Category
			fmt.Printf("\n Категория: %s\n", currentCategory)
			fmt.Println("   ───────────────────────────────────")
		}
		fmt.Printf(" %d. %s  (поставок: %d)\n", pr.Rank, pr.ProductName, pr.ShipmentCount)
	}

	// 6. (Бонус) Выполним EXPLAIN ANALYZE для этого запроса,
	// чтобы убедиться, что используются оконные функции и индексы.
	fmt.Println("\n=== ПЛАН ВЫПОЛНЕНИЯ (EXPLAIN ANALYZE) ===")
	var plan string
	err = pool.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) "+query).Scan(&plan)
	if err != nil {
		log.Printf("Не удалось получить план выполнения: %v", err)
	} else {
		fmt.Println(plan)
	}

	// 7. Дополнительно: покажем, как использовать другие оконные функции
	// Например, LAG для сравнения с предыдущей поставкой (задача 5).
	fmt.Println("\n=== ПРИМЕР С LAG (сравнение с предыдущей поставкой) ===")
	lagQuery := `
		SELECT
			id,
			supplier_id,
			shipment_date,
			quantity,
			LAG(quantity) OVER (PARTITION BY supplier_id ORDER BY shipment_date) AS prev_quantity
		FROM shipments
		WHERE supplier_id = 1  -- ограничим для наглядности
		ORDER BY shipment_date
		LIMIT 10;
	`

	type ShipmentWithLag struct {
		ID           int
		SupplierID   int
		ShipmentDate time.Time
		Quantity     int
		PrevQuantity *int // может быть NULL, поэтому указатель
	}

	rows2, err := pool.Query(ctx, lagQuery)
	if err != nil {
		log.Fatalf("Ошибка выполнения LAG-запроса: %v", err)
	}
	defer rows2.Close()

	fmt.Printf("\n%-5s %-12s %-15s %-8s %-8s\n", "ID", "Supplier", "Date", "Qty", "Prev Qty")
	fmt.Println("─────────────────────────────────────────────────")
	for rows2.Next() {
		var swl ShipmentWithLag
		err := rows2.Scan(
			&swl.ID,
			&swl.SupplierID,
			&swl.ShipmentDate,
			&swl.Quantity,
			&swl.PrevQuantity,
		)
		if err != nil {
			log.Fatalf("Ошибка сканирования: %v", err)
		}
		prev := "NULL"
		if swl.PrevQuantity != nil {
			prev = fmt.Sprintf("%d", *swl.PrevQuantity)
		}
		fmt.Printf("%-5d %-12d %-15s %-8d %-8s\n",
			swl.ID,
			swl.SupplierID,
			swl.ShipmentDate.Format("2006-01-02"),
			swl.Quantity,
			prev,
		)
	}
	fmt.Println("\n ВЫВОДЫ ")
	fmt.Println("1. Оконные функции (ROW_NUMBER, LAG, LEAD, RANK и др.) легко")
	fmt.Println("   выполняются из Go через обычный Query.")
	fmt.Println("2. Результат сканируется в структуры так же, как и обычные запросы.")
	fmt.Println("3. Используйте EXPLAIN (ANALYZE) для проверки производительности.")
	fmt.Println("4. Оконные функции позволяют строить сложные отчёты без множественных подзапросов.")
	fmt.Println("5. В Go можно удобно группировать и форматировать результат для пользователя.")
}
