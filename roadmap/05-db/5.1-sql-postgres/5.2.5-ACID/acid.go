package acid

import (
	"context"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	conn, err := pgxpool.New(ctx, "postgres://user:pass@localhost:5432/db")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	// 1. Создаём таблицу логов
	_, err = conn.Exec(ctx, `
		DROP TABLE IF EXISTS logs;
		CREATE TABLE logs (
			id SERIAL PRIMARY KEY,
			message TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		);
	`)
	if err != nil {
		log.Fatal(err)
	}

	// 2. Вставляем 100 000 строк
	_, err = conn.Exec(ctx, `
		INSERT INTO logs (message)
		SELECT 'log_' || generate_series(1, 100000);
	`)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Вставлено 100 000 строк")

	// 3. Измеряем размер таблицы
	var sizeBefore int64
	err = conn.QueryRow(ctx, `SELECT pg_total_relation_size('logs');`).Scan(&sizeBefore)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Размер до удаления: %d байт (%.2f MB)", sizeBefore, float64(sizeBefore)/1024/1024)

	// 4. Удаляем все записи (имитация очистки логов)
	_, err = conn.Exec(ctx, `DELETE FROM logs;`)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Удалены все записи")

	// 5. Измеряем размер после удаления
	var sizeAfter int64
	err = conn.QueryRow(ctx, `SELECT pg_total_relation_size('logs');`).Scan(&sizeAfter)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Размер после удаления: %d байт (%.2f MB)", sizeAfter, float64(sizeAfter)/1024/1024)
	log.Printf("Размер не уменьшился! (MVCC держит мёртвые строки)")

	// 6. Смотрим количество мёртвых строк
	var nDead int64
	err = conn.QueryRow(ctx, `SELECT n_dead_tup FROM pg_stat_user_tables WHERE relname = 'logs';`).Scan(&nDead)
	if err != nil {
		log.Println("Не удалось получить статистику, возможно, нужно включить pg_stat_statements")
	}
	log.Printf("Мёртвых строк: %d", nDead)

	// 7. Выполняем VACUUM
	_, err = conn.Exec(ctx, `VACUUM logs;`)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Выполнен VACUUM")

	// 8. Размер после VACUUM
	var sizeAfterVac int64
	err = conn.QueryRow(ctx, `SELECT pg_total_relation_size('logs');`).Scan(&sizeAfterVac)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Размер после VACUUM: %d байт (%.2f MB)", sizeAfterVac, float64(sizeAfterVac)/1024/1024)
	log.Printf("VACUUM пометил мёртвые строки как свободное место, но не вернул ОС")

	// 9. VACUUM FULL (блокирует таблицу!)
	_, err = conn.Exec(ctx, `VACUUM FULL logs;`)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Выполнен VACUUM FULL (блокировал таблицу)")

	// 10. Размер после VACUUM FULL
	var sizeAfterFull int64
	err = conn.QueryRow(ctx, `SELECT pg_total_relation_size('logs');`).Scan(&sizeAfterFull)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Размер после VACUUM FULL: %d байт (%.2f MB)", sizeAfterFull, float64(sizeAfterFull)/1024/1024)
	log.Println("VACUUM FULL вернул место ОС, но заблокировал таблицу на время выполнения")

	// 11. Использование автовакуума — просто включаем и настраиваем
	log.Println("В продакшене рекомендуется настроить AUTOVACUUM: autovacuum_vacuum_scale_factor = 0.05")
	log.Println("Или использовать партиционирование для автоматического удаления старых партиций")
}
