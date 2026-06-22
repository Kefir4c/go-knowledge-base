package __6_slog

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/google/uuid"
)

/*
LOG/SLOG — СТРУКТУРИРОВАННОЕ ЛОГИРОВАНИЕ (Go 1.21+)

slog — официальный пакет для структурированного логирования.
Он пришёл на смену старому log и стандартизировал подход.

ПОЧЕМУ SLOG?
  1. СТРУКТУРИРОВАННЫЕ ЛОГИ — не строки, а пары ключ-значение.
  2. УРОВНИ — Debug, Info, Warn, Error.
  3. КОНТЕКСТ — можно добавлять поля из контекста.
  4. JSON ПО УМОЛЧАНИЮ — легко интегрируется с ELK/Loki/Grafana.

ЧЕМ ЛУЧШЕ SLOG:
  • Легче парсить логи (JSON, а не строки).
  • Легче фильтровать по уровням.
  • Легче добавлять поля без изменения сообщения.
  • Легче искать по ключам (не надо писать regexp).
*/

/*
1. ОСНОВНЫЕ ТИПЫ

type Logger struct { ... }      // основной логгер
type Handler interface { ... }  // определяет формат вывода
type Level int                  // уровень логирования

LEVELS:
  LevelDebug = -4
  LevelInfo  = 0
  LevelWarn  = 4
  LevelError = 8

ДЕФОЛТНЫЙ ЛОГГЕР:
  slog.Default() // возвращает глобальный логгер

ОСНОВНЫЕ МЕТОДЫ:
  logger.Debug(msg, args...)
  logger.Info(msg, args...)
  logger.Warn(msg, args...)
  logger.Error(msg, args...)
  logger.Log(ctx context.Context, level Level, msg string, args ...any)

  logger.With(args...) *Logger   // возвращает логгер с привязанными полями
  logger.WithGroup(name string) *Logger // группирует поля

2. ХЕНДЛЕРЫ (HANDLERS)

TEXT HANDLER (человекочитаемый):
  handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
      Level:     slog.LevelDebug,
      AddSource: true, // добавить файл:строку
  })
  logger := slog.New(handler)

JSON HANDLER (для продакшена):
  handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
      Level:     slog.LevelInfo,
      AddSource: false,
  })
  logger := slog.New(handler)

ЗАМЕНА ГЛОБАЛЬНОГО ЛОГГЕРА:
  slog.SetDefault(logger)

3. АТРИБУТЫ (ATTRIBUTES)

Способы передачи атрибутов:

1. КЛЮЧ-ЗНАЧЕНИЕ (самый популярный):
  logger.Info("user created", "id", 123, "name", "Alice")

2. ЧЕРЕЗ slog.Any:
  logger.Info("user created", slog.Any("user", user))

3. ЧЕРЕЗ slog.Int, slog.String:
  logger.Info("user created", slog.Int("id", 123), slog.String("name", "Alice"))

4. СВЯЗЫВАНИЕ С ЛОГГЕРОМ:
  logger = logger.With("component", "http", "service", "users")
  logger.Info("request started") // автоматически добавит компонент и сервис

5. ГРУППЫ (вложенные поля):
  logger.WithGroup("user").Info("created", "id", 123, "name", "Alice")
  // {"time":"...","msg":"created","user":{"id":123,"name":"Alice"}}

4. КОНТЕКСТНЫЕ ПОЛЯ (CONTEXT)

ЧТО ЭТО: поля, которые достаются из контекста и добавляются в лог.

КАК РАБОТАЕТ:
  1. Извлекаем логгер из контекста (или создаём с полями)
  2. Передаём логгер в контексте
  3. Внутри функции берём логгер из контекста

ПРИМЕР:
  func middleware(next http.Handler) http.Handler {
      return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
          requestID := uuid.New().String()
          logger := slog.With("request_id", requestID)
          ctx := context.WithValue(r.Context(), "logger", logger)
          r = r.WithContext(ctx)
          next.ServeHTTP(w, r)
      })
  }

  // внутри хендлера
  logger := ctx.Value("logger").(*slog.Logger)
  logger.Info("processing request")

5. НАСТРОЙКА ДЛЯ РАЗНЫХ СРЕД

ПРОДАКШЕН:
  • JSON Handler (для ELK/Loki)
  • Уровень: INFO или WARN
  • AddSource: false (для производительности)

РАЗРАБОТКА:
  • Text Handler (читаемый)
  • Уровень: DEBUG
  • AddSource: true (показывает файл и строку)

ПЕРЕКЛЮЧЕНИЕ ЧЕРЕЗ ENV:
  env := os.Getenv("ENV")
  var handler slog.Handler
  if env == "production" {
      handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
          Level: slog.LevelInfo,
      })
  } else {
      handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
          Level:     slog.LevelDebug,
          AddSource: true,
      })
  }
  logger := slog.New(handler)
  slog.SetDefault(logger)

6. ПРОИЗВОДИТЕЛЬНОСТЬ (ВАЖНО!)

ЧТО МЕДЛЕННО:
  logger.Info("msg", "key", expensiveFunc()) // expensiveFunc вызывается ВСЕГДА

ЧТО БЫСТРО:
  logger.Info("msg", "key", func() any { ... }) // вызывается только если уровень позволяет

  // Или с использованием аттрибутов:
  logger.LogAttrs(context.Background(), slog.LevelInfo, "msg", slog.Any("key", expensiveFunc()))

ЛУЧШАЯ ПРАКТИКА:
  if logger.Enabled(ctx, slog.LevelDebug) {
      logger.Debug("expensive", "data", expensiveFunc())
  }

7. ТИПИЧНЫЕ ОШИБКИ

1. ПЕРЕДАЧА НЕПРАВИЛЬНЫХ ТИПОВ:
   ❌ slog.Info("user", "id", "123") // id должен быть int
   ✅ slog.Info("user", "id", 123)

2. ИГНОРИРОВАНИЕ ОШИБОК:
   ❌ logger.Error("failed", "err", err) // err не передан как поле
   ✅ logger.Error("failed", "err", err)

3. ЗАБЫТЬ СЕТОВАТЬ ДЕФОЛТ:
   slog.SetDefault(logger) // иначе используется неконфигурированный

4. ПЕРЕДАЧА НЕПРАВИЛЬНОГО КОНТЕКСТА:
   ❌ logger.Info("msg", "ctx", ctx) // ctx невалидное поле
   ✅ logger.InfoContext(ctx, "msg")

5. СЛИШКОМ МНОГО ПОЛЕЙ:
   ❌ logger.Info("msg", "field1", v1, "field2", v2, ... "field30", v30)
   ✅ logger.With("component", "x").Info("msg", "key", value)

8. ШПАРГАЛКА ДЛЯ СОБЕСЕДОВАНИЯ

КАК СОЗДАТЬ ЛОГГЕР?
  logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

КАК ЗАМЕНИТЬ ДЕФОЛТ?
  slog.SetDefault(logger)

ЧЕМ JSON ОТЛИЧАЕТСЯ ОТ TEXT?
  JSON — машинный (для ELK), Text — человеческий (для разработки).

ЧТО ТАКОЕ АТРИБУТЫ?
  Пары ключ-значение, которые добавляются в лог.

ЗАЧЕМ СВЯЗЫВАТЬ ЛОГГЕР С КОНТЕКСТОМ?
  Чтобы передавать request-scoped поля (trace_id, user_id) без явной передачи.

КАК УМЕНЬШИТЬ НАГРУЗКУ?
  Использовать LogAttrs, проверять уровень перед вызовом.

ЗАЧЕМ ИСПОЛЬЗОВАТЬ JSON?
  Для интеграции с системами сбора логов (ELK, Loki, Splunk).

ЧТО ТАКОЕ WITHGROUP?
  Группирует поля в JSON (вложенные объекты).

*/

// 1. БАЗОВЫЙ ЛОГГЕР
func primer1() {
	// Создаём логгер
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// Простые логи
	slog.Info("server started", "port", 8080)
	slog.Warn("memory usage high", "percent", 85)
	slog.Error("connection failed", "err", "timeout")

	// Debug (не будет выведен, если уровень не Debug)
	slog.Debug("debug message", "key", "value")
}

// 2. РАЗНЫЕ ХЕНДЛЕРЫ
func primer2() {
	// Text handler (человек)
	textHandler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
	})
	textLogger := slog.New(textHandler)

	// JSON handler (машина)
	jsonHandler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelInfo,
	})

	jsonLogger := slog.New(jsonHandler)

	textLogger.Info("text handler")
	jsonLogger.Info("json handler")
}

// 3. АТРИБУТЫ
func primer3() {
	logger := slog.Default()

	// 1. Ключ-значение (популярный)
	logger.Info("user created", "id", 123, "name", "Kolya")

	// 2. slog-атрибуты (типобезопасно)
	logger.Info("user created", slog.Int("id", 123), slog.String("name", "Kolya"))

	// 3. Связывание с логгером
	userLogger := logger.With("component", "user", "service", "auth")

	userLogger.Info("user logged in")
	userLogger.Info("user logged out")

	// 4. Группы
	logger.WithGroup("user").Info("created",
		slog.Int("id", 123),
		slog.String("name", "Alice"),
	)
}

// 4. КОНТЕКСТ
type ctxKey string

func primer4() {
	logger := slog.Default()

	// Добавляем requestID в контекст
	ctx := context.WithValue(context.Background(), ctxKey("logger"), logger.With("request_id, req-123"))

	// Извлекаем и логируем
	log := ctx.Value(ctxKey("logger")).(*slog.Logger)
	log.Info("processing request")
}

// 5. НАСТРОЙКА ДЛЯ РАЗНЫХ СРЕД
func primer5() {
	env := os.Getenv("ENV")
	if env == "" {
		env = "dev"
	}

	var handler slog.Handler
	if env == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: false,
			Level:     slog.LevelInfo,
		})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			AddSource: true,
			Level:     slog.LevelDebug,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	slog.Info("environment", "env", env)
	slog.Debug("debug message in dev")
}

// 6. ПРОИЗВОДИТЕЛЬНОСТЬ
func primer6() {
	logger := slog.Default()

	// 1. Дорогая операция вызывается ТОЛЬКО если уровень позволяет
	expensive := func() string {
		time.Sleep(100 * time.Millisecond)
		return "expensive_value"
	}

	// ❌ ПЛОХО: expensive() вызывается всегда
	logger.Debug("debug", "key", expensive())

	// ✅ ХОРОШО: проверяем уровень
	if logger.Enabled(context.Background(), slog.LevelDebug) {
		logger.Debug("debug", "key", expensive())
	}

	// ✅ ХОРОШО: через LogAttrs
	logger.LogAttrs(context.Background(), slog.LevelDebug, "debug", slog.Any("key", expensive()))
}

// 7. ПРОДАКШЕН-ЛОГГЕР С HTTP
func primer7() {
	// Настройка: JSON + Info
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	logger := slog.New(handler)

	// Добавляем постоянные поля
	logger = logger.With("service", "myapp", "version", "1.0.0")

	// Логирование с ошибкой
	err := fmt.Errorf("connection refused")
	logger.Error("failed to connect",
		"host", "localhost",
		"port", 5432,
		"error", err,
	)

	// Логирование с контекстом
	ctx := context.WithValue(context.Background(), ctxKey("request_id"), "req-123")
	logger.Info("request started", "request_id", ctx.Value(ctxKey("request_id")))
}

// 8. MIDDLEWARE ДЛЯ HTTP
func primer8(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Добавляем request ID
		requserID := uuid.New().String()
		logger := slog.Default().With(
			"request_id", requserID,
			"method", r.Method,
			"path", r.URL.Path)

		// Сохраняем в контекст
		ctx := context.WithValue(context.Background(), "logger", logger)
		r = r.WithContext(ctx)

		// Логируем входящий запрос
		logger.Info("request started")

		// Обрабатываем запрос
		next.ServeHTTP(w, r)

		// Логируем завершение
		logger.Info("request finished", "duration", time.Since(start))
	})
}

func main() {
	fmt.Println("primer1")
	primer1()
	fmt.Println("primer2")
	primer2()
	fmt.Println("primer3")
	primer3()
	fmt.Println("primer4")
	primer4()
	fmt.Println("primer5")
	primer5()
	fmt.Println("primer6")
	primer6()
	fmt.Println("primer7")
	primer7()
	fmt.Println("primer8")
	//primer8()
}
