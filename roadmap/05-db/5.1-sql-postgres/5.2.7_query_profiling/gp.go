package queryprofiling

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

/*
 НАСТРОЙКА ЛОГИРОВАНИЯ МЕДЛЕННЫХ ЗАПРОСОВ В GO

 В Go можно логировать медленные запросы на уровне драйвера БД, чтобы:
   - видеть запросы, которые тормозят в приложении,
   - связывать их с конкретными эндпоинтами или бизнес-операциями,
   - использовать структурированное логирование (slog, zap) для интеграции с мониторингом.

 Основные подходы:
   1. Обёртка над методом Query/Exec с замером времени.
   2. Использование Middleware для HTTP-хендлеров.
   3. Настройка пула соединений с таймаутами.
   4. Логирование query_id из pg_stat_statements (опционально).

 В этом примере мы покажем универсальную обёртку, которую можно встроить в репозиторий.
*/
// 1. КОНФИГУРАЦИЯ ЛОГГЕРА

var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// 2. ОБЁРТКА ДЛЯ ВЫПОЛНЕНИЯ ЗАПРОСОВ С ЗАМЕРОМ ВРЕМЕНИ
// SlowQueryLogger содержит пул соединений и порог медленного запроса.
type SlowQueryLogger struct {
	pool    *pgxpool.Pool
	timeout time.Duration // порог в миллисекундах
}

func NewSlowQueryLogger(pool *pgxpool.Pool, slowThreshold time.Duration) *SlowQueryLogger {
	return &SlowQueryLogger{
		pool:    pool,
		timeout: slowThreshold,
	}
}

// Exec обёртка над Exec с логированием.
func (s *SlowQueryLogger) Exec(ctx context.Context, sql string, args ...any) (pgx.CommandTag, error) {
	start := time.Now()
	result, err := s.pool.Exec(ctx, sql, args...)
	duration := time.Since(start)

	if duration > s.timeout {
		logger.Warn("Slow query detected",
			slog.String("sql", sql),
			slog.Any("args", args),
			slog.String("duration_ms", duration.String()),
			slog.String("method", "Exec"),
		)
	}
	return result, err
}

// Query обёртка над Query с логированием.
func (s *SlowQueryLogger) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	start := time.Now()
	rows, err := s.pool.Query(ctx, sql, args...)
	duration := time.Since(start)

	if duration > s.timeout {
		logger.Warn("Slow query detected",
			slog.String("sql", sql),
			slog.Any("args", args),
			slog.String("duration_ms", duration.String()),
			slog.String("method", "Query"),
		)
	}
	return rows, err
}

// QueryRow обёртка над QueryRow с логированием.
func (s *SlowQueryLogger) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	start := time.Now()
	row := s.pool.QueryRow(ctx, sql, args...)
	duration := time.Since(start)

	if duration > s.timeout {
		logger.Warn("Slow query detected",
			slog.String("sql", sql),
			slog.Any("args", args),
			slog.String("duration_ms", duration.String()),
			slog.String("method", "QueryRow"),
		)
	}
	return row
}

// 3. ПРИМЕР РЕПОЗИТОРИЯ С ОБЁРТКОЙ

type UserRepo struct {
	db *SlowQueryLogger
}

func NewUserRepo(db *SlowQueryLogger) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) GetUserByID(ctx context.Context, id int64) (string, error) {
	var name string
	// Используем обёртку вместо прямого вызова pool
	err := r.db.QueryRow(ctx, "SELECT name FROM users WHERE id = $1", id).Scan(&name)
	if err != nil {
		return "", err
	}
	return name, nil
}

func (r *UserRepo) GetActiveUsers(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.db.Query(ctx, "SELECT name FROM users WHERE is_active = true LIMIT $1", limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		names = append(names, name)
	}
	return names, nil
}

// 4. HTTP MIDDLEWARE ДЛЯ ЛОГИРОВАНИЯ МЕДЛЕННЫХ ЗАПРОСОВ

// SlowMiddleware — HTTP-прослойка для логирования медленных обработчиков.
func SlowMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Запоминаем query_id из заголовка или генерируем свой
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = fmt.Sprintf("%d", time.Now().UnixNano())
		}

		// Передаём request_id в контекст (чтобы логировать в БД-методах)
		ctx := context.WithValue(r.Context(), "request_id", requestID)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)

		duration := time.Since(start)
		if duration > 500*time.Millisecond {
			logger.Warn("Slow HTTP request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("request_id", requestID),
				slog.String("duration_ms", duration.String()),
			)
		}
	})
}

// 5. ИНИЦИАЛИЗАЦИЯ И ИСПОЛЬЗОВАНИЕ

func main() {
	// Подключение к БД
	cfg, err := pgxpool.ParseConfig("postgres://user:pass@localhost:5432/db")
	if err != nil {
		logger.Error("Failed to parse config", "error", err)
		return
	}
	// Настраиваем таймауты соединений
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		logger.Error("Failed to connect", "error", err)
		return
	}
	defer pool.Close()

	// Создаём обёртку с порогом 100 мс
	slowLogger := NewSlowQueryLogger(pool, 100*time.Millisecond)

	// Репозиторий
	repo := NewUserRepo(slowLogger)

	// Пример использования
	ctx := context.Background()
	name, err := repo.GetUserByID(ctx, 1)
	if err != nil {
		logger.Error("GetUserByID failed", "error", err)
	} else {
		logger.Info("User found", "name", name)
	}

	names, err := repo.GetActiveUsers(ctx, 10)
	if err != nil {
		logger.Error("GetActiveUsers failed", "error", err)
	} else {
		logger.Info("Active users", "count", len(names))
	}
}
