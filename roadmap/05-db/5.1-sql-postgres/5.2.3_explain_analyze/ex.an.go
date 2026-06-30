package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	// 1. Подключение к БД
	connStr := "postgres://user:password@localhost:5432/your_db?sslmode=disable"

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatal(err)
	}
	pool.Close()

	fmt.Println("ЦИКЛ ОПТИМИЗАЦИИ ЗАПРОСА\n")

	// 2. МЕДЛЕННЫЙ ЗАПРОС (БЕЗ ИНДЕКСА)

	// Убедимся, что индекса нет (для чистоты эксперимента)
	_, _ = pool.Exec(ctx, "DROP INDEX IF EXISTS idx_shipments_status_date")

	fmt.Println("1. Выполняем запрос БЕЗ индекса...")

	// Запрос, который мы будем оптимизировать
	query := `
		SELECT id, product_id, quantity, shipment_date
		FROM shipments
		WHERE status = 'delivered' AND shipment_date > '2023-01-01'
		ORDER BY shipment_date DESC
		LIMIT 10
	`

	// Замеряем время выполнения
	start := time.Now()
	rows, err := pool.Query(ctx, query)
	if err != nil {
		log.Fatal("Ошибка выполнения запроса:", err)
	}
	defer rows.Close()

	// Считаем строки (чтобы не разрывать соединение)
	var count int
	for rows.Next() {
		count++
	}

	elapsed := time.Since(start)
	fmt.Printf("   Время выполнения: %s, возвращено строк: %d\n", elapsed, count)

	// 3. ПОЛУЧАЕМ ПЛАН ВЫПОЛНЕНИЯ (EXPLAIN ANALYZE)

	fmt.Println("\n2. Получаем план выполнения (EXPLAIN ANALYZE)...")
	var plan string

	err = pool.QueryRow(ctx, "EXPLAIN(ANALYZE, BUFFERS, FORMAT TEXT)"+query).Scan(&plan)

	if err != nil {
		log.Fatal("Ошибка EXPLAIN:", err)
	}
	fmt.Println("   План выполнения:")
	fmt.Println(plan)

	// Анализируем план (вручную, по тексту)
	if contains(plan, "Seq Scan") {
		fmt.Println("Обнаружен Seq Scan — это медленно! Нужен индекс.")
	} else if contains(plan, "Index Scan") {
		fmt.Println("Index Scan — индекс используется, но читает таблицу.")
	} else if contains(plan, "Index Only Scan") {
		fmt.Println("Index Only Scan — идеально! Все данные из индекса.")
	}

	// 4. СОЗДАЁМ ИНДЕКС
	fmt.Println("\n3. Создаём составной индекс на (status, shipment_date)...")
	_, err = pool.Exec(ctx, "CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shipments_status_date ON shipments(status, shipment_date)")
	if err != nil {
		log.Fatal("Ошибка создания индекса:", err)
	}
	fmt.Println("   Индекс создан.")

	// 5. ПОВТОРНЫЙ ЗАПРОС (С ИНДЕКСОМ)

	fmt.Println("\n4. Выполняем запрос С индексом...")
	start = time.Now()
	rows, err = pool.Query(ctx, query)
	if err != nil {
		log.Fatal("Ошибка выполнения запроса:", err)
	}
	defer rows.Close()
	count = 0
	for rows.Next() {
		count++
	}
	elapsed = time.Since(start)
	fmt.Printf("   Время выполнения: %s, возвращено строк: %d\n", elapsed, count)

	// 6. ПОВТОРНЫЙ EXPLAIN (ПОСЛЕ ИНДЕКСА)
	fmt.Println("\n5. Получаем план выполнения ПОСЛЕ создания индекса:")
	err = pool.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) "+query).Scan(&plan)
	if err != nil {
		log.Fatal("Ошибка EXPLAIN после индекса:", err)
	}
	fmt.Println("   План выполнения после индекса:")
	fmt.Println(plan)

	if contains(plan, "Index Scan") || contains(plan, "Index Only Scan") {
		fmt.Println("Индекс работает! Запрос ускорился.")
	} else {
		fmt.Println("Индекс не используется. Проверьте условие.")
	}

	// 7. ДОПОЛНИТЕЛЬНО: ПОКРЫВАЮЩИЙ ИНДЕКС (Index Only Scan)

	fmt.Println("\n6. (Бонус) Создаём покрывающий индекс для запроса, который возвращает только несколько колонок...")
	_, err = pool.Exec(ctx, `
		CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_shipments_cover
		ON shipments(status, shipment_date)
		INCLUDE (product_id, quantity)
	`)
	if err != nil {
		log.Fatal("Ошибка создания покрывающего индекса:", err)
	}

	// Запрос, который может использовать покрывающий индекс
	coverQuery := `
		SELECT product_id, quantity, shipment_date
		FROM shipments
		WHERE status = 'delivered' AND shipment_date > '2023-01-01'
		ORDER BY shipment_date DESC
		LIMIT 10
	`

	fmt.Println("   Выполняем запрос с покрывающим индексом...")
	start = time.Now()
	rows, err = pool.Query(ctx, coverQuery)
	if err != nil {
		log.Fatal("Ошибка выполнения запроса:", err)
	}
	defer rows.Close()
	count = 0
	for rows.Next() {
		count++
	}
	elapsed = time.Since(start)
	fmt.Printf("   Время выполнения: %s, возвращено строк: %d\n", elapsed, count)

	// EXPLAIN для покрывающего индекса
	err = pool.QueryRow(ctx, "EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT) "+coverQuery).Scan(&plan)
	if err != nil {
		log.Fatal("Ошибка EXPLAIN покрывающего индекса:", err)
	}
	fmt.Println("   План выполнения с покрывающим индексом:")
	fmt.Println(plan)

	if contains(plan, "Index Only Scan") {
		fmt.Println("Index Only Scan — таблица не читалась! Максимальная производительность.")
	}
}

func contains(text, substr string) bool {
	return len(text) >= len(substr) && (text == substr || len(substr) == 0 ||
		(len(text) > 0 && len(substr) > 0 && findSubstring(text, substr)))
}

// Простая реализация поиска подстроки (чтобы не тащить strings)
func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
