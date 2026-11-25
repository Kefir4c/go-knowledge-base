package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

/*
Logging Middleware (Логирование запросов)
Зачем: Чтобы знать, кто, когда и с каким статусом дернул твой API.
Подвох: Стандартный http.ResponseWriter не отдает статус-код (200, 404). Нам нужно обернуть его в свою структуру.
*/
/*
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		lrw := &loggingResponseWriter{w, http.StatusOK}

		next.ServeHTTP(lrw, r)

		duration := time.Since(start)
		log.Printf("[%s] %s %d %v", r.Method, r.URL.Path, lrw.statusCode, duration)
	})
}
*/

/*
Recovery Middleware (Отлов паник)
Если в одном хендлере случится panic: index out of range, сервер упадет.
Мы должны поймать панику, залогировать её и вернуть клиенту 500.
*/
/*
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC: %v\n%s", err, debug.Stack())
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
*/

/*
Auth Middleware (Проверка JWT/Токена)
Зачем: Защитить приватные роуты.
*/
/*
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" || token != "Bearer my-secret-token" {
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}
*/
/*
Request ID Middleware (TraceID)
Зачем: Чтобы связать логи всех слоев для одного запроса..
*/
/*
type ctxKey string
const requestIDKey ctxKey = "HybaByba"

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := fmt.Sprintf("req-%d", time.Now().UnixNano())
		ctx := context.WithValue(r.Context(), requestIDKey, reqID)

		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}
*/
/*
CORS Middleware (Кросс-доменные запросы)
Зачем: Чтобы фронтенд на localhost:3000 мог делать запросы к бэкенду на localhost:8080.
*/
/*
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Браузер сначала шлет OPTIONS-запрос (preflight), на него надо ответить 200 OK
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
*/

type CustomResponseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (w *CustomResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *CustomResponseWriter) Write(b []byte) (int, error) {
	if w.statusCode == 0 {
		w.statusCode = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	return n, err
}

func (w *CustomResponseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

type ctxKey string

const traceIDKey ctxKey = "traceID"

func LoggerMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			traceID := fmt.Sprintf("req-%d", time.Now().UnixNano())
			ctx := context.WithValue(r.Context(), traceIDKey, traceID)
			r = r.WithContext(ctx)

			cw := &CustomResponseWriter{w, http.StatusOK, 0}
			next.ServeHTTP(cw, r)

			duration := time.Since(start)

			reqLogger := logger.With(
				slog.String("trace_id", traceID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", cw.statusCode),
				slog.Duration("duration", duration),
				slog.Int64("bytes_written", cw.written),
				slog.String("user_agent", r.UserAgent()),
			)

			if cw.statusCode >= 500 {
				reqLogger.Error("HTTP Request Failed")
			} else if cw.statusCode >= 400 {
				reqLogger.Warn("HTTP Client Error")
			} else {
				reqLogger.Info("HTTP Request")
			}
		})
	}
}

func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					traceID, _ := r.Context().Value(traceIDKey).(string)

					logger.Error("PANIC RECOVERED",
						slog.String("trace_id", traceID),
						slog.Any("error", err),
						slog.String("stack_trace", string(debug.Stack())),
					)

					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusInternalServerError)

					json.NewEncoder(w).Encode(map[string]string{
						"error":    "Internal Server Error",
						"trace_id": traceID,
					})
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}
