package __2_2_idx

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Сценарий: у тебя есть таблица suppliers с колонкой email.
// Ты выполняешь SELECT по email, и он тормозит на 100k+ строк.
// Ты создаёшь индекс и проверяешь ускорение.

// 1. ПОДКЛЮЧЕНИЕ К БД
func main() {
	ctx := context.Background()

	connStr := "postgres://user:password@localhost:5432/dbname?sslmode=disable"
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	// 2. МЕДЛЕННЫЙ ЗАПРОС (БЕЗ ИНДЕКСА)
	// Выполняем запрос, замеряем время.
	start := time.Now()

	var name, email string
	err = pool.QueryRow(ctx, "SELECT name, email FROM suppliers WHERE email = 'roga@mail.ru'").Scan(&name, &email)
	if err != nil {
		log.Println("Query error:", err)
	}
	elapsed := time.Since(start)
	fmt.Printf("Запрос без индекса: %s, результат: name=%s, email=%s\n", elapsed, name, email)

	// 3. ПОЛУЧАЕМ ПЛАН ВЫПОЛНЕНИЯ (EXPLAIN)

	// Чтобы понять, почему медленно, выполняем EXPLAIN (без ANALYZE, чтобы не делать запрос заново)
	var plan string
	err = pool.QueryRow(ctx, "EXPLAIN (FORMAT TEXT) SELECT name, email FROM suppliers WHERE email = 'roga@mail.ru'").Scan(&plan)
	if err != nil {
		log.Println("Explain error:", err)
	}
	fmt.Printf("План выполнения:\n%s\n", plan)
	// Ожидаем: Seq Scan on suppliers (cost=... rows=...)
	// Это значит, что индекс не используется.

	// 4. СОЗДАЁМ ИНДЕКС (через тот же пул соединений)

	// В реальном проекте индекс создают через миграции (golang-migrate, goose).
	// Но для демонстрации можем выполнить прямо здесь (с осторожностью).
	fmt.Println("Создаём индекс idx_suppliers_email ...")
	_, err = pool.Exec(ctx, "CREATE INDEX IF NOT EXISTS idx_suppliers_email ON suppliers(email)")
	if err != nil {
		log.Fatal("Не удалось создать индекс:", err)
	}
	fmt.Println("Индекс создан.")

	// 5. ПОВТОРНЫЙ ЗАПРОС (ПОСЛЕ СОЗДАНИЯ ИНДЕКСА)

	start = time.Now()
	err = pool.QueryRow(ctx, "SELECT name, email FROM suppliers WHERE email = 'roga@mail.ru'").Scan(&name, &email)
	if err != nil {
		log.Println("Query error (after index):", err)
	}
	elapsed = time.Since(start)
	fmt.Printf("Запрос С индексом: %s, результат: name=%s, email=%s\n", elapsed, name, email)

	// 6. ПРОВЕРЯЕМ ПЛАН ПОСЛЕ ИНДЕКСА

	err = pool.QueryRow(ctx, "EXPLAIN (FORMAT TEXT) SELECT name, email FROM suppliers WHERE email = 'roga@mail.ru'").Scan(&plan)
	if err != nil {
		log.Println("Explain error after index:", err)
	}
	fmt.Printf("План выполнения ПОСЛЕ создания индекса:\n%s\n", plan)
	// Ожидаем: Index Scan using idx_suppliers_email on suppliers

	// 7. ДОПОЛНИТЕЛЬНО: ПОКАЗАТЬ РАЗНИЦУ В ЧИСЛАХ

	// Если индекс не используется, можно принудительно отключить Seq Scan (не рекомендуется в проде).
	// Но для теста можно использовать SET enable_seqscan = off; временно.
	// Это покажет, что индекс ускоряет запрос.

	_, err = pool.Exec(ctx, "SET LOCAL enable_seqscan = off")
	if err != nil {
		log.Println("Ошибка отключения Seq Scan:", err)
	}
	// Снова выполняем запрос с отключенным Seq Scan.
	start = time.Now()
	err = pool.QueryRow(ctx, "SELECT name, email FROM suppliers WHERE email = 'roga@mail.ru'").Scan(&name, &email)
	if err != nil {
		log.Println("Query error (seqscan off):", err)
	}
	elapsed = time.Since(start)
	fmt.Printf("Запрос с отключенным Seq Scan: %s\n", elapsed)
}
