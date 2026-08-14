package main

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-playground/validator/v10"
	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/time/rate"
)

/*
  УРОК 6.3. MIDDLEWARE

  Middleware — это один из краеугольных камней веб-разработки на Go.
  Понимание middleware на уровне архитектуры, производительности и нюансов
  реализации отличает джуниора от сеньора. Этот материал покрывает всё,
  что может быть задано на собеседовании.

  1. ЧТО ТАКОЕ MIDDLEWARE В КОНТЕКСТЕ HTTP?
  Middleware (промежуточное ПО) — это функция, которая получает HTTP-запрос,
  выполняет какую-то логику, а затем передаёт управление следующему обработчику
  в цепочке. Цепочка middleware образует "конвейер" (pipeline), через который
  проходит каждый запрос.

  В Go стандартный паттерн для middleware — это функция, которая принимает
  http.Handler и возвращает http.Handler:
      type Middleware func(http.Handler) http.Handler
  Это позволяет легко компоновать middleware, создавая вложенные обёртки.

  В популярных фреймворках (Echo, Gin) middleware имеют свой интерфейс,
  но суть остаётся той же: каждый middleware может выполнить код до и после
  вызова следующего обработчика.

  2. ЗАЧЕМ НУЖЕН MIDDLEWARE? (СКВОЗНЫЕ ЗАДАЧИ)
  Основное преимущество middleware — централизация сквозной функциональности,
  которая не относится к бизнес-логике, но требуется для каждого запроса.

  Примеры таких задач:
    - Логирование (кто, когда, что запросил, сколько времени заняло)
    - Аутентификация и авторизация (проверка токенов, прав доступа)
    - Обработка ошибок и паник (recovery, кастомные ответы на ошибки)
    - CORS (кросс-доменные запросы)
    - Сжатие ответов (gzip)
    - Лимитирование тела запроса (body limit)
    - Трейсинг и распределённое логирование (RequestID)
    - Извлечение реального IP (RealIP)
    - Ограничение частоты запросов (rate limiting)
    - Таймауты (graceful deadline для длительных операций)
    - Добавление заголовков безопасности (HSTS, CSP, etc.)
    - Кэширование ответов
    - Метрики и мониторинг (Prometheus)

  Используя middleware, мы:
    - избегаем дублирования кода в каждом обработчике;
    - упрощаем тестирование (middleware тестируются отдельно);
    - делаем код более модульным и читаемым.

  3. ПОРЯДОК ПРИМЕНЕНИЯ (ORDER MATTERS)
  Middleware применяются в том порядке, в котором они зарегистрированы.
  Это критически важно, потому что некоторые middleware зависят от других.

  Например, правильный порядок для стандартного набора:
    1. Recover       (самый внешний) — ловит паники из всех внутренних слоёв.
    2. RequestID     — генерирует ID для логирования.
    3. Logging       — логирует запросы, используя RequestID.
    4. RealIP        — извлекает IP из заголовков.
    5. CORS          — добавляет заголовки CORS.
    6. Auth          — проверяет аутентификацию.
    7. RateLimit     — ограничивает частоту запросов.
    8. BodyLimit     — ограничивает размер тела.
    9. Timeout       — устанавливает дедлайн на выполнение.
    10. Основной роутер и бизнес-логика.

  Если поменять местами, например, Logging и Recover, то логирование не
  перехватит панику, потому что паника будет обработана раньше.

  4. КОНТЕКСТ И ПЕРЕДАЧА ДАННЫХ МЕЖДУ MIDDLEWARE
  Middleware часто должны передавать данные дальше (например, ID пользователя,
  RequestID, результат аутентификации). Для этого используется context.Context.

  В стандартном http.Handler мы добавляем данные в контекст запроса:

      ctx := context.WithValue(r.Context(), userIDKey, userID)
      r = r.WithContext(ctx)
      next.ServeHTTP(w, r)

  В Echo контекст (echo.Context) хранит значения через c.Set(key, value),
  а в Gin — через c.Set(key, value).

  Важно: контекст живёт только в рамках текущего запроса. Нельзя сохранять
  контекст в глобальные переменные или использовать его после завершения
  запроса (например, в горутинах).

  5. ВИДЫ MIDDLEWARE ПО ОТНОШЕНИЮ К ОБРАБОТЧИКУ
  Различают middleware, которые:
    - Выполняются до обработчика (pre-middleware) — подготовка данных,
      аутентификация, валидация.
    - Выполняются после обработчика (post-middleware) — логирование,
      сжатие ответа, добавление заголовков.
    - Могут прервать цепочку (преждевременно вернуть ответ) — например,
      если аутентификация не пройдена, middleware возвращает 401 и не вызывает
      следующий обработчик.
    - Могут изменить запрос или ответ — например, сжать тело ответа.

  6. ПРОИЗВОДИТЕЛЬНОСТЬ MIDDLEWARE
  Middleware добавляют накладные расходы, но обычно они незначительны.
  Однако при высоких нагрузках стоит учитывать:
    - Каждый middleware выполняет код и может выделять память.
    - Слишком длинные цепочки могут увеличить задержку.
    - Важно избегать тяжёлых операций в middleware (например, сложных
      вычислений или блокирующих вызовов), если они не обязательны для
      каждого запроса.
    - Используйте готовые оптимизированные middleware (например, из
      пакетов github.com/gorilla/handlers, github.com/justinas/alice
      для композиции, golang.org/x/time/rate для лимитов).
    - В Echo и Gin middleware стараются быть zero-allocation (не выделять
      память в горячих путях).

  7. СТАНДАРТНЫЕ ПАТТЕРНЫ РЕАЛИЗАЦИИ
  а) Функция, возвращающая middleware (замыкание) — позволяет передавать
     параметры (например, таймаут, список разрешённых доменов для CORS).

  б) Middleware с доступом к контексту фреймворка (Echo, Gin) —
     используют специфичный для фреймворка контекст (c *gin.Context,
     c echo.Context) для удобства работы с запросом/ответом.

  в) Композиция — использование вспомогательных библиотек для объединения
     нескольких middleware в цепочку (например, alice).

  г) Middleware как структура (реже) — когда нужно хранить состояние
     (например, пул соединений).

  8. ОБРАБОТКА ОШИБОК В MIDDLEWARE
  Ошибки в middleware обрабатываются по-разному:

  - В стандартном http.Handler: если middleware обнаруживает ошибку,
    он может записать ответ (через http.Error) и вернуться, не вызывая next.
    Но важно не возвращать ошибку, потому что http.HandlerFunc не возвращает
    ошибку. Вместо этого мы просто пишем ответ и return.

  - В Echo: middleware может вернуть ошибку (error). Если он возвращает
    ошибку, дальнейшая цепочка не выполняется, и Echo вызывает глобальный
    обработчик ошибок (HTTPErrorHandler), чтобы отдать клиенту ответ.

  - В Gin: middleware может прервать цепочку вызовом c.Abort() и
    затем вернуть ответ.

  9. РЕАЛИЗАЦИЯ REQUEST-ID В РАСПРЕДЕЛЁННЫХ СИСТЕМАХ
  В микросервисной архитектуре RequestID используется для сквозной трассировки.
  Каждый сервис должен:
    1. Принимать X-Request-ID из входящего запроса.
    2. Если его нет — генерировать новый.
    3. Передавать его во все исходящие вызовы (HTTP, gRPC, DB) и логи.
  Это позволяет связать логи разных сервисов по одному ID.

  10. REALIP — ЗАЧЕМ И КАК
  Когда сервер стоит за прокси (nginx, haproxy, AWS ELB), реальный IP клиента
  передаётся в заголовках:
    - X-Forwarded-For: список IP-адресов (клиент, прокси1, прокси2...)
    - X-Real-IP: прямой IP клиента (часто устанавливается nginx)

  RealIP middleware извлекает первый публичный IP из X-Forwarded-For или
  значение X-Real-IP и подставляет его в r.RemoteAddr (или в контекст).

  Важно: если прокси доверенный, иначе злоумышленник может подделать
  заголовки. Поэтому обычно middleware должен проверять, что IP отправителя
  находится в доверенном списке (или использовать значение из конфигурации).

  11. RATE LIMITING — СТРАТЕГИИ
  Rate limit (ограничение частоты запросов) защищает от DDoS и перегрузки.
  Популярные алгоритмы:
    - Token bucket (golang.org/x/time/rate) — гибкий и простой.
    - Sliding window — скользящее окно.
    - Leaky bucket — выравнивание нагрузки.
  Хранилище для счётчиков:
    - In-memory (для одного экземпляра) — быстро, но не разделяется между
      подами.
    - Redis (для распределённой системы) — общее хранилище.
    - Memcached.

  В стандартной библиотеке есть готовый middleware для gin и echo.

  12. TIMEOUT MIDDLEWARE — КАК НЕ ПОТЕРЯТЬ СОЕДИНЕНИЕ
  Таймауты важны для защиты от "медленных" клиентов или зависших операций.
  Реализация:
    1. Создаём контекст с таймаутом через context.WithTimeout.
    2. Запускаем основной обработчик в горутине.
    3. Ждём либо завершения, либо срабатывания таймаута.
    4. При таймауте возвращаем 504 Gateway Timeout.
  Важно: обрабатывать таймауты на уровне сервера (ReadTimeout, WriteTimeout)
  и на уровне middleware — они решают разные задачи.

  13. БЕЗОПАСНОСТЬ В MIDDLEWARE (CORS, HSTS, CSP)
  - CORS: добавляет заголовки, разрешающие кросс-доменные запросы.
    В продакшене никогда не используйте "*" (разрешить все) — настраивайте
    список разрешённых origin.

  - HSTS: Strict-Transport-Security — требует от браузера всегда использовать
    HTTPS. Добавляется через middleware.

  - CSP: Content-Security-Policy — ограничивает, откуда можно загружать ресурсы.

  Все эти заголовки можно добавлять централизованно через middleware.

  14. КОМПОЗИЦИЯ MIDDLEWARE (ЦЕПОЧКИ)
  Вместо того чтобы вручную оборачивать один middleware в другой, можно
  использовать утилиты для композиции:

  - В стандартном net/http: часто используется паттерн с функцией, которая
    принимает slice middleware и базовый handler, и возвращает конечный handler.

  - Популярная библиотека "alice" (github.com/justinas/alice) позволяет
    строить цепочки middleware более декларативно:

        chain := alice.New(
            RecoverMiddleware,
            LoggingMiddleware,
            RequestIDMiddleware,
        )
        handler := chain.Then(finalHandler)

  - В Echo и Gin композиция встроена: e.Use(mw1, mw2, ...) применяет их
    последовательно.

  15. ТЕСТИРОВАНИЕ MIDDLEWARE
  Middleware тестируются изолированно:
    1. Создаём тестовый обработчик (можно простой http.HandlerFunc).
    2. Оборачиваем его тестируемым middleware.
    3. Используем httptest.NewRequest и httptest.NewRecorder.
    4. Проверяем, что:
       - middleware добавил нужные заголовки,
       - изменил контекст,
       - вернул корректный статус при ошибке,
       - не вызвал панику и т.д.

  Пример:
      func TestRequestIDMiddleware(t *testing.T) {
          next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
              id := GetRequestID(r.Context())
              w.Write([]byte(id))
          })
          handler := RequestIDMiddleware(next)
          req := httptest.NewRequest("GET", "/", nil)
          w := httptest.NewRecorder()
          handler.ServeHTTP(w, req)
          // проверяем, что в ответе есть ID и заголовок X-Request-ID
      }

  16. ГОТОВЫЕ БИБЛИОТЕКИ MIDDLEWARE
  Сторонние пакеты предоставляют множество готовых middleware, экономя время:
  - gorilla/handlers: CORS, Compress, Logging, Recovery, etc.
  - justinas/alice: композиция цепочек.
  - rs/cors: гибкий CORS.
  - go-chi/chi/middleware: набор для chi.
  - labstack/echo/middleware: встроенные в Echo.
  - gin-contrib: расширения для Gin.

  Использование готовых решений обычно предпочтительнее, чтобы не изобретать
  велосипед, но важно понимать, как они работают под капотом.

  17. ПОДВОДНЫЕ КАМНИ И BEST PRACTICES
  1. Не изменяйте тело запроса (r.Body) без необходимости — это может
     сломать последующие middleware или обработчики.

  2. Если вы читаете тело запроса (например, для логирования), не забудьте
     восстановить его (через io.TeeReader) или прочитать один раз, чтобы
     не потерять данные.

  3. Всегда вызывайте next.ServeHTTP (или c.Next()) внутри middleware,
     иначе цепочка оборвётся.

  4. Используйте defer для освобождения ресурсов после завершения запроса.

  5. Не храните большие данные в контексте — это может привести к утечкам
     памяти.

  6. Для паник используйте recover только в самом верхнем middleware,
     чтобы централизованно логировать и возвращать 500.

  7. Не затягивайте выполнение middleware длительными операциями (например,
     запросы к БД) — это увеличит задержку для всех запросов.

  8. Для высоконагруженных систем предпочитайте zero-allocation middleware
     (например, в Echo/gin). Изучите, как устроены их контексты, чтобы
     минимизировать выделения.

  9. Логируйте ошибки с достаточной информацией (RequestID, стек трейс,
     время, метод, путь).

  10. Middleware должны быть идемпотентными (не иметь побочных эффектов,
      зависящих от предыдущих вызовов) — они могут вызываться много раз
      для разных запросов.

  18. СРАВНЕНИЕ РЕАЛИЗАЦИЙ В РАЗНЫХ ФРЕЙМВОРКАХ
  Стандартный http.Handler:
      func middleware(next http.Handler) http.Handler {
          return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
              // до
              next.ServeHTTP(w, r)
              // после
          })
      }

  Echo:
      func middleware(next echo.HandlerFunc) echo.HandlerFunc {
          return func(c echo.Context) error {
              // до
              err := next(c)
              // после (возвращаем ошибку)
              return err
          }
      }

  Gin:
      func middleware(c *gin.Context) {
          // до
          c.Next()
          // после
      }

  Основные отличия:
    - В stdlib и chi — используется http.Handler, обработчик не возвращает
      ошибку, ошибки обрабатываются через запись ответа.
    - В Echo — обработчик возвращает error, что позволяет использовать
      глобальный обработчик ошибок.
    - В Gin — c.Next() вызывает следующий обработчик, а пауза в цепочке
      делается через c.Abort().

  19. ЗАКЛЮЧЕНИЕ
  Middleware — это мощный паттерн, который позволяет выносить сквозную
  функциональность из бизнес-логики, делая код чище и тестируемее.
  Понимание порядка, контекста, производительности и готовых решений —
  ключевой навык для Go-разработчика.

  На собеседовании вас могут спросить:
    - "Напишите middleware для логирования" — это база.
    - "Как передать данные между middleware?" — через контекст.
    - "Что произойдёт, если middleware не вызовет next?" — цепочка оборвётся.
    - "Как защититься от паник?" — recover middleware.
    - "Как ограничить частоту запросов?" — rate limit middleware.
    - "В каком порядке применять middleware?" — зависит от зависимостей.
*/

// ОБЩИЕ ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ
func runServer(srv *http.Server, name string) {
	go func() {
		log.Printf("[%s] запущен на %s", name, srv.Addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[%s] ошибка: %v", name, err)
		}
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("[%s] ошибка при остановке: %v", name, err)
	}
}

func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

//ПРИМЕР 0: СТРУКТУРИРОВАННОЕ ЛОГИРОВАНИЕ (JSON)
/*
ЗАЧЕМ: В продакшене нужны структурированные логи в JSON, чтобы их можно было
       собирать в ELK/Loki/Datadog и анализировать. Логи включают все поля,
       которые помогают в отладке: request_id, метод, путь, статус, время,
       размер, IP, User-Agent, а также метаданные (например, версию сервиса).

ФИШКА: Используем кастомный логгер на основе log/slog (или своего формата).
       Все логи централизованы. Добавляем уровень логирования (info, error, warn).
       Включаем поля, которые можно фильтровать (статус >= 400 — ошибки).
*/

// LogEntry — структура лога (можно дополнить под свои нужды).
type LogEntry struct {
	Timestamp   string `json:"timestamp"`
	Level       string `json:"level"`
	RequestID   string `json:"request_id,omitempty"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Status      int    `json:"status"`
	DurationMs  int64  `json:"duration_ms"`
	SizeBytes   int    `json:"size_bytes"`
	IP          string `json:"ip"`
	UserAgent   string `json:"user_agent"`
	Referer     string `json:"referer"`
	QueryParams string `json:"query_params,omitempty"`
	Error       string `json:"error,omitempty"`
	Service     string `json:"service"`
	Env         string `json:"env"`
	Version     string `json:"version"`
}

// Глобальный логгер (можно инициализировать из конфига).
var (
	logger      *log.Logger
	serviceName = "my-service"
	env         = "prod"
	version     = "1.0.0"
)

func init() {
	// Инициализация стандартного логгера (можно заменить на slog).
	logger = log.New(os.Stdout, "", 0) // без префикса, чтобы не ломать JSON.
}

func logJSON(entry *LogEntry) {
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	entry.Service = serviceName
	entry.Env = env
	data, _ := json.Marshal(entry)
	log.Println(string(data))
}

// LoggingMiddleware — для net/http.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		reqID := GetRequestID(r.Context()) // предполагаем, что есть функция получения
		if reqID == "" {
			reqID = "no-id"
		}
		lw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lw, r)

		entry := &LogEntry{
			Level:       infoLevel(lw.statusCode),
			RequestID:   reqID,
			Method:      r.Method,
			Path:        r.URL.Path,
			Status:      lw.statusCode,
			DurationMs:  time.Since(start).Milliseconds(),
			SizeBytes:   lw.bodySize,
			IP:          r.RemoteAddr,
			UserAgent:   r.UserAgent(),
			Referer:     r.Referer(),
			QueryParams: r.URL.RawQuery,
		}
		if lw.statusCode >= 400 {
			entry.Error = http.StatusText(lw.statusCode)
		}
		logJSON(entry)
	})
}

// infoLevel определяет уровень лога по коду статуса.
func infoLevel(status int) string {
	if status >= 500 {
		return "error"
	} else if status >= 400 {
		return "warn"
	}
	return "info"
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
	bodySize   int
}

func (lw *loggingResponseWriter) Write(b []byte) (int, error) {
	n, err := lw.ResponseWriter.Write(b)
	lw.bodySize += n
	return n, err
}

func (lw *loggingResponseWriter) WriteHeader(statusCode int) {
	lw.statusCode = statusCode
	lw.ResponseWriter.WriteHeader(statusCode)
}

//ПРИМЕР 1: КАСТОМНЫЙ КОНТЕКСТ (CHI)

/*
ЗАЧЕМ: В сложных системах middleware должны передавать данные дальше по цепочке
       (результат аутентификации, ID пользователя, скоуп доступа). В chi это
       делается через context.WithValue.

ФИШКА: Создаём кастомные ключи для контекста, чтобы избежать коллизий.
       В middleware извлекаем токен, валидируем его и сохраняем пользователя
       в контекст. В обработчике получаем пользователя через функцию-хелпер.
*/

type contextKey string

const UserKey contextKey = "user"

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token == "" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if token != "valid-token" {
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		user := map[string]string{"id": "123", "name": "Kaka"}
		ctx := context.WithValue(r.Context(), UserKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func GetUser(ctx context.Context) map[string]string {
	user, ok := ctx.Value(UserKey).(map[string]string)
	if !ok {
		return nil
	}
	return user
}

func primer1() {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(AuthMiddleware)

	r.Get("/api/profile", func(w http.ResponseWriter, r *http.Request) {
		user := GetUser(r.Context())
		if user == nil {
			http.Error(w, "User not found", http.StatusInternalServerError)
			return
		}
		respondJSON(w, http.StatusOK, user)
	})

	srv := &http.Server{Addr: ":8080", Handler: r}
	runServer(srv, "primer1")
}

// ─── ПРИМЕР 2: ГРУППИРОВКА С ПРЕФИКСНЫМИ MIDDLEWARE (CHI) ──────────────

/*
ЗАЧЕМ: Часто разные части API требуют разных middleware.
       Например, публичные эндпоинты (без аутентификации) и защищённые (с JWT).
       Chi позволяет навешивать middleware на группы маршрутов.

ФИШКА: Используем r.Group() и r.With() для создания подроутеров с общими
       middleware. Админка имеет свой префикс /admin и свою аутентификацию,
       отдельный роутер для API v1 и v2.
*/

func primer2() {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("ok"))
	})

	apiV1 := r.Group(nil)
	apiV1.Use(AuthMiddleware)
	apiV1.Route("/api/v1", func(r chi.Router) {
		r.Get("/users", func(w http.ResponseWriter, r *http.Request) {
			respondJSON(w, http.StatusOK, []string{"user1", "user2"})
		})
	})
	apiV2 := r.Group(nil)
	apiV2.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-API-KEY")
			if key != "super-key" {
				http.Error(w, "Invalid API key", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	apiV2.Route("api/v2", func(r chi.Router) {
		r.Get("/data", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("v2 data"))
		})
	})
	admin := r.With(chimw.BasicAuth("admin", map[string]string{"admin": "password"}))
	admin.Route("/admin", func(r chi.Router) {
		r.Get("/stats", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("stats: 100 user"))
		})
	})
	srv := &http.Server{Addr: ":8081", Handler: r}
	runServer(srv, "primer2")
}

//ПРИМЕР 3: ОТЛОЖЕННАЯ ИНИЦИАЛИЗАЦИЯ (CHI)

/*
ЗАЧЕМ: Некоторые ресурсы (БД, клиенты) инициализируются один раз при старте,

	но иногда нужно отложить инициализацию до первого запроса (например,
	для перезагрузки конфига или ленивой загрузки).

ФИШКА: Используем sync.Once и middleware, который проверяет, инициализирован ли

	ресурс, и если нет — делает это. Подходит для случаев, где инициализация
	дорогая и нужна не всегда.
*/
var (
	dbClient   *fakeDB
	initDBOnce sync.Once
)

type fakeDB struct {
	conn string
}

func initDB() {
	log.Println("Инициализация БД...")
	time.Sleep(2 * time.Second)
	dbClient = &fakeDB{conn: "connected"}
}

func DBConnection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		initDBOnce.Do(initDB)
		ctx := context.WithValue(r.Context(), "db", dbClient)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func primer3() {
	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(DBConnection)

	r.Get("/api/status", func(w http.ResponseWriter, r *http.Request) {
		db := r.Context().Value("db").(*fakeDB)
		w.Write([]byte(db.conn))
	})

	srv := &http.Server{Addr: ":8082", Handler: r}
	runServer(srv, "CHI-Example3")
}

//ПРИМЕР 4: ГЛОБАЛЬНЫЙ ОБРАБОТЧИК ОШИБОК (ECHO)

/*
ЗАЧЕМ: В Echo обработчики возвращают error, что позволяет централизованно
       обрабатывать ошибки. Удобно для логирования и кастомизации ответов.

ФИШКА: Переопределяем HTTPErrorHandler. Все ошибки (404, 405, бизнес-ошибки)
       проходят через него. Добавляем RequestID для трейсинга.
*/

func RequestIDMiddlewareEcho(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		reqID := c.Request().Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
		c.Set("request_id", reqID)
		c.Response().Header().Set("X-Request-ID", reqID)
		return next(c)
	}
}

func LoggerMiddlewareEcho(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		err := next(c)
		reqID, _ := c.Get("request_id").(string)
		log.Printf("[%s] %s %s → %d (%v)", reqID, c.Request().Method, c.Path(), c.Response().Status, time.Since(start))
		return err
	}
}

func primer4() {
	e := echo.New()
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		reqID, _ := c.Get("request_id").(string)
		if reqID == "" {
			reqID = "no-id"
		}

		if he, ok := err.(*echo.HTTPError); ok {
			code := he.Code
			message := he.Message.(string)
			log.Printf("[ERROR] request_id=%s code=%d message=%s", reqID, code, message)
			c.JSON(code, map[string]string{
				"error":      message,
				"request_id": reqID,
			})
			return
		}
		log.Printf("[ERROR] request_id=%s unknown: %v", reqID, err)
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error":      "internal server error",
			"request_id": reqID,
		})
	}

	e.Use(RequestIDMiddlewareEcho)
	e.Use(LoggerMiddlewareEcho)

	e.GET("/api/users/:id", func(c echo.Context) error {
		id := c.Param("id")
		if id == "0" {
			return echo.NewHTTPError(http.StatusNotFound, "user not found")
		}
		return c.JSON(http.StatusOK, map[string]string{"id": id})
	})
	e.GET("/api/panic", func(c echo.Context) error {
		panic("test panic")
	})
	srv := &http.Server{Addr: ":8083", Handler: e}
	runServer(srv, "primer4")
}

//ПРИМЕР 5: ВАЛИДАЦИЯ ЗАПРОСОВ (ECHO)

/*
ЗАЧЕМ: Валидация входящих данных — обязательная часть любого API.
       Echo поддерживает валидацию через теги структур и middleware.

ФИШКА: Используем пакет go-playground/validator. Создаём кастомную функцию
       валидации, которая автоматически проверяет все структуры в хендлерах.
       В middleware перехватываем ошибки и возвращаем структурированный ответ.
*/

type CustomValidator struct {
	validator *validator.Validate
}

func (cv *CustomValidator) Validate(i interface{}) error {
	return cv.validator.Struct(i)
}

func primer5() {
	e := echo.New()
	e.Validator = &CustomValidator{validator: validator.New()}

	type userRequest struct {
		Name  string `json:"name" validate:"required"`
		Email string `json:"email" validate:"required,email"`
		Age   int    `json:"age" validate:"min=18,max=120"`
	}
	e.POST("/api/users", func(c echo.Context) error {
		var req userRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid json")
		}
		if err := c.Validate(req); err != nil {
			errs := err.(validator.ValidationErrors)
			errorsMap := make(map[string]string)
			for _, e := range errs {
				errorsMap[e.Field()] = e.Tag()
			}
			return c.JSON(http.StatusBadRequest, map[string]interface{}{
				"error":  "validation failed",
				"fields": errorsMap,
			})
		}
		return c.JSON(http.StatusCreated, map[string]string{"id": "123"})
	})
	srv := &http.Server{Addr: ":8084", Handler: e}
	runServer(srv, "primer5")
}

//РИМЕР 6: ПОТОКОВАЯ ПЕРЕДАЧА ДАННЫХ (ECHO)

/*
ЗАЧЕМ: Для больших отчётов, логов или событий (SSE) нужна потоковая передача.

	Echo поддерживает стриминг через c.Response().Flush().

ФИШКА: Отправляем данные по частям без закрытия соединения. Используем

	c.Response().Flush() для немедленной отправки каждого куска.
*/
func primer6() {
	e := echo.New()
	e.GET("/api/stream", func(c echo.Context) error {
		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Connection", "keep-live")

		flusher, ok := c.Response().Writer.(http.Flusher)
		if !ok {
			return echo.NewHTTPError(http.StatusInternalServerError, "streaming not supported")
		}

		for i := 1; i <= 10; i++ {
			msg := fmt.Sprintf("data: { \"count\": %d }\n\n", i)
			_, err := c.Response().Write([]byte(msg))
			if err != nil {
				return err
			}
			flusher.Flush()
			time.Sleep(1 * time.Second)
		}
		_, err := c.Response().Write([]byte("data: { \"status\": \"done\" }\n\n"))
		if err != nil {
			return err
		}
		flusher.Flush()
		return nil
	})

	srv := &http.Server{Addr: ":8085", Handler: e}
	runServer(srv, "primer6")
}

//ПРИМЕР 7: АУТЕНТИФИКАЦИЯ JWT (GIN)

/*
ЗАЧЕМ: JWT — стандартный способ аутентификации в REST API.
       Gin имеет удобные middleware для работы с JWT.

ФИШКА: Используем библиотеку github.com/golang-jwt/jwt/v5.
       В middleware проверяем токен, извлекаем claims и сохраняем в контекст.
       В хендлерах получаем пользователя через функцию.
*/

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

var jwtSecret = []byte("my-secret-key")

func JWTAuth(c *gin.Context) {
	tokenString := c.Request.Header.Get("Authorization")
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{})
		c.Abort()
		return
	}
	tokenString = strings.TrimPrefix(tokenString, "Bearer")

	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
		c.Abort()
		return
	}
	claims, ok := token.Claims.(*Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid claims"})
		c.Abort()
		return
	}
	c.Set("user_id", claims.UserID)
	c.Next()
}

func GetUserIDGin(c *gin.Context) string {
	id, _ := c.Get("user_id")
	return id.(string)
}

func primer7() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	r.POST("/api/login", func(c *gin.Context) {
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, &Claims{
			UserID: "123",
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			},
		})
		tokenString, _ := token.SignedString(jwtSecret)
		c.JSON(http.StatusOK, gin.H{"token": tokenString})
	})

	auth := r.Group("/api")
	auth.Use(JWTAuth)
	{
		auth.GET("/profile", func(c *gin.Context) {
			userID := GetUserIDGin(c)
			c.JSON(http.StatusOK, gin.H{"user_id": userID, "name": "John"})
		})
	}

	srv := &http.Server{Addr: ":8086", Handler: r}
	runServer(srv, "primer7")
}

//ПРИМЕР 8: КЭШИРОВАНИЕ ОТВЕТОВ (GIN)

/*
ЗАЧЕМ: Кэширование часто запрашиваемых данных (GET-запросы) снижает нагрузку.
       Gin позволяет легко реализовать in-memory кэш в middleware.

ФИШКА: Используем sync.Map для хранения кэша с TTL. В middleware проверяем
       наличие кэша, если есть — отдаём, иначе выполняем обработчик и сохраняем.
*/

type CacheEntry struct {
	Data      []byte
	Status    int
	Headers   map[string]string
	CreatedAt time.Time
	TTL       time.Duration
}

type Cache struct {
	store sync.Map
}

func (c *Cache) Get(key string) (*CacheEntry, bool) {
	val, ok := c.store.Load(key)
	if !ok {
		return nil, false
	}
	entry := val.(*CacheEntry)
	if time.Since(entry.CreatedAt) > entry.TTL {
		c.store.Delete(key)
		return nil, false
	}
	return entry, true
}

func (c *Cache) Set(key string, entry *CacheEntry) {
	c.store.Store(key, entry)
}

var cache = &Cache{}

func CacheMiddleware(ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != "GET" {
			c.Next()
			return
		}
		key := c.Request.URL.String()
		if entry, ok := cache.Get(key); ok {
			for k, v := range entry.Headers {
				c.Writer.Header().Set(k, v)
			}
			c.Writer.Header().Set("X-Cache", "HIT")
			c.Writer.WriteHeader(entry.Status)
			c.Writer.Write(entry.Data)
			c.Abort()
			return
		}
		w := &cacheResponseWriter{ResponseWriter: c.Writer}
		c.Writer = w
		c.Next()
		if w.statusCode >= 200 && w.statusCode < 300 {
			cache.Set(key, &CacheEntry{
				Data:      w.body,
				Status:    w.statusCode,
				Headers:   w.headers,
				CreatedAt: time.Now(),
				TTL:       ttl,
			})
		}
	}
}

type cacheResponseWriter struct {
	gin.ResponseWriter
	body       []byte
	statusCode int
	headers    map[string]string
}

func (w *cacheResponseWriter) Write(b []byte) (int, error) {
	w.body = append(w.body, b...)
	return w.ResponseWriter.Write(b)
}

func (w *cacheResponseWriter) WriteHeader(statusCode int) {
	w.statusCode = statusCode
	w.ResponseWriter.WriteHeader(statusCode)
}

func (w *cacheResponseWriter) Header() http.Header {
	headers := w.ResponseWriter.Header()
	w.headers = make(map[string]string)
	for k, v := range headers {
		if len(v) > 0 {
			w.headers[k] = v[0]
		}
	}
	return headers
}

func primer8() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(CacheMiddleware(60 * time.Second))

	r.GET("/api/expensive", func(c *gin.Context) {
		time.Sleep(2 * time.Second)
		c.JSON(http.StatusOK, gin.H{"data": "expensive result", "timestamp": time.Now().Unix()})
	})

	srv := &http.Server{Addr: ":8087", Handler: r}
	runServer(srv, "primer8")
}

// ─── ПРИМЕР 9: ТРАССИРОВКА И МЕТРИКИ (GIN) ──────────────────────────────

/*
ЗАЧЕМ: Для мониторинга и отладки нужны метрики (количество запросов, latency,
       статусы) и трассировка (распределённый трейсинг). Gin легко интегрируется
       с Prometheus и Jaeger.

ФИШКА: В middleware собираем метрики: счётчики по эндпоинтам, гистограммы
       времени ответа. Используем кастомные метрики Prometheus.
*/

var (
	requestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "path", "status"},
	)
	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 2.0, 5.0},
		},
		[]string{"method", "path"},
	)
)

func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		status := c.Writer.Status()
		path := c.FullPath()
		requestsTotal.WithLabelValues(c.Request.Method, path, strconv.Itoa(status)).Inc()
		requestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}

func primer9() {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(MetricsMiddleware())

	r.GET("/api/data", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"data": "ok"})
	})
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	srv := &http.Server{Addr: ":8088", Handler: r}
	runServer(srv, "primer9")
}

func main() {
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
	primer7()
	primer8()
	primer9()
}
