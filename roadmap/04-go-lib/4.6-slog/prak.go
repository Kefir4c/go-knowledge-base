package __6_slog

import (
	"context"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"golang.org/x/exp/rand"
)

// 1. ГЕНЕРАТОР REQUEST_ID
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b)
}

// 2. КЛЮЧИ ДЛЯ КОНТЕКСТА
type ctxKey string

const (
	requestIDKey ctxKey = "request_id"
	userIDKey    ctxKey = "user_id"
	loggerKey    ctxKey = "logger"
)

// 3. ПОЛУЧЕНИЕ USER_ID ИЗ ЗАГОЛОВКА (имитация)
func getUserIDFromHeader(r *http.Request) string {
	// В реальном проекте — из JWT или сессии
	return r.Header.Get("X-User-ID")
}

// 4. MIDDLEWARE ЛОГИРОВАНИЯ
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Генерируем request_id
		reqID := generateRequestID()

		// Получаем user_id (если есть)
		userID := getUserIDFromHeader(r)

		logger := slog.With(
			"request_id", reqID,
			"user_id", userID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr)

		ctx := context.WithValue(r.Context(), requestIDKey, reqID)
		ctx = context.WithValue(ctx, userIDKey, userID)
		ctx = context.WithValue(ctx, loggerKey, logger)

		r = r.WithContext(ctx)

		// Логируем начало запроса
		logger.Info("request started")

		// Обрабатываем запрос
		next.ServeHTTP(w, r)

		// Логируем завершение с длительностью
		latency := time.Since(start).Milliseconds()
		logger.Info("request finished",
			"latency_ms", latency,
			"status", w.(*responseWriter).status, // кастомный writer для захвата статуса
		)
	})
}

// 5. КАСТОМНЫЙ RESPONSE WRITER ДЛЯ ЗАХВАТА СТАТУСА
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// 6. ХЕНДЛЕРЫ
func userHandler(w http.ResponseWriter, r *http.Request) {
	// Получаем логгер из контекста
	logger := r.Context().Value(loggerKey).(*slog.Logger)

	// Имитация работы
	time.Sleep(50 * time.Millisecond)

	// Логируем бизнес-событие
	logger.Info("user fetched",
		"user_id", r.Context().Value(userIDKey),
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"ok"}`))
}

// 7. ОБРАБОТЧИК ОШИБОК (ПРИМЕР)
func errorHandler(w http.ResponseWriter, r *http.Request) {
	logger := r.Context().Value(loggerKey).(*slog.Logger)

	// Имитация ошибки
	err := fmt.Errorf("database connection lost")
	logger.Error("failed to process request",
		"error", err,
		"user_id", r.Context().Value(userIDKey),
	)

	http.Error(w, "internal error", http.StatusInternalServerError)
}

// 8. НАСТРОЙКА ЛОГГЕРА
func setupLogger(env string) *slog.Logger {
	var handler slog.Handler
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}

	if env == "prod" {
		// Продакшен: JSON, без source (для производительности)
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		// Разработка: Text, с source и Debug
		opts.Level = slog.LevelDebug
		opts.AddSource = true
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}

func main() {
	// Настройка логгера
	env := os.Getenv("ENV")
	if env == "" {
		env = "dev"
	}
	logger := setupLogger(env)
	slog.SetDefault(logger)

	// Создаём роутер
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/users", userHandler)
	mux.HandleFunc("GET /api/error", errorHandler)

	// Оборачиваем в middleware
	handler := loggingMiddleware(mux)

	// Запускаем сервер
	slog.Info("server starting", "port", 8080, "env", env)
	if err := http.ListenAndServe(":8080", handler); err != nil {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}
