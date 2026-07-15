package conf

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// 1. КОНФИГ ПУЛА (согласован с БД)
type PoolConfig struct {
	MaxConns          int32         // ≤ max_connections в БД (например, 50)
	MinConns          int32         // 5-10 для прогрева
	MaxConnLifetime   time.Duration // 30m
	MaxConnIdleTime   time.Duration // 10m
	HealthCheckPeriod time.Duration // 1m
	QueryTimeout      time.Duration // 5s (защита от зависания)
}

func DefaultPool() PoolConfig {
	return PoolConfig{
		MaxConns:          50,
		MinConns:          10,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   10 * time.Minute,
		HealthCheckPeriod: 1 * time.Minute,
		QueryTimeout:      5 * time.Second,
	}
}

// 2. СОЗДАНИЕ ПУЛА
func NewPool(cfg PoolConfig, dsn string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	config.MaxConns = cfg.MaxConns
	config.MinConns = cfg.MinConns
	config.MaxConnLifetime = cfg.MaxConnLifetime
	config.MaxConnIdleTime = cfg.MaxConnIdleTime
	config.HealthCheckPeriod = cfg.HealthCheckPeriod
	// config.ConnConfig.ConnectTimeout = 5 * time.Second (можно задать в DSN)

	return pgxpool.NewWithConfig(context.Background(), config)
}

// 3. МОНИТОРИНГ (проверяем настройки и производительность)
// CacheHitRatio должна быть > 95% (иначе увеличивай shared_buffers)
func CacheHitRatio(ctx context.Context, pool *pgxpool.Pool) (float64, error) {
	var ratio float64
	err := pool.QueryRow(ctx, `
		SELECT round(
			sum(heap_blks_hit) / (sum(heap_blks_hit) + sum(heap_blks_read)) * 100.0,
			2
		) FROM pg_statio_user_tables
	`).Scan(&ratio)
	return ratio, err
}

// Проверить текущие настройки PostgreSQL (полезно при старте)
func CheckSettings(ctx context.Context, pool *pgxpool.Pool) {
	params := []string{"shared_buffers", "work_mem", "maintenance_work_mem", "effective_cache_size", "max_connections"}
	for _, p := range params {
		var v string
		if err := pool.QueryRow(ctx, "SHOW "+p).Scan(&v); err == nil {
			log.Printf("%s = %s", p, v)
		}
	}
}

// Фоновый мониторинг (запустить в горутине)
func MonitorDB(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ratio, err := CacheHitRatio(ctx, pool); err == nil && ratio < 95.0 {
				log.Printf("⚠️ Cache hit ratio = %.2f%% (<95%%) — увеличьте shared_buffers", ratio)
			}
			// Статистика пула
			s := pool.Stat()
			log.Printf("Pool: acquired=%d idle=%d max=%d", s.AcquiredConns(), s.IdleConns(), s.MaxConns())
		}
	}
}

// 4. ВЫПОЛНЕНИЕ ЗАПРОСОВ С ТАЙМАУТОМ

func ExecWithTimeout(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration, q string, args ...interface{}) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := pool.Exec(ctx, q, args...)
	return err
}

func QueryWithTimeout(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration, q string, args ...interface{}) (pgx.Rows, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return pool.Query(ctx, q, args...)
}

// 5. GRACEFUL SHUTDOWN

func WaitForShutdown(pool *pgxpool.Pool) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("Shutting down, closing DB pool...")
	pool.Close()
}

// 6. ПРИМЕР ИСПОЛЬЗОВАНИЯ

func main() {
	pool, _ := NewPool(DefaultPool(), "postgres://user:pass@localhost:5432/db")
	defer pool.Close()

	ctx := context.Background()
	CheckSettings(ctx, pool)

	go MonitorDB(ctx, pool, 10*time.Second)

	// Запрос с таймаутом
	rows, _ := QueryWithTimeout(ctx, pool, 2*time.Second,
		"SELECT id FROM orders WHERE user_id = $1", 1)
	defer rows.Close()
	// ... scan rows

	WaitForShutdown(pool)
}
