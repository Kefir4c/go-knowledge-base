package logs

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

//1. ОСНОВЫ И СТАНДАРТНЫЙ ПАКЕТ LOG
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:
1. Зачем нужны логи?
Это "черный ящик" самолета. Когда программа падает у пользователя или на сервере,
логи — единственный способ узнать, что пошло не так.
- Отладка: Поиск багов без интерактивного дебаггера.
- История: Хронология событий (кто зашел, что нажал).

2. Стандартный пакет log:
- Входит в состав Go. Простой как палка.
- Потокобезопасность: Можно писать логи из разных горутин одновременно,
пакет сам выстроит их в очередь и не перемешает символы.
- Формат: По умолчанию выводит дату и время.

3. Уровни логирования (База):
Хотя в пакете log нет кнопок "INFO" или "ERROR", программист сам разделяет их:
- Информационные: "Сервер запущен", "Подключение успешно".
- Предупреждения: "Мало места на диске", "Повторная попытка запроса".
- Ошибки: "База данных недоступна", "Файл не найден".
*/
func LogBasics() {
	// --- 1. Знание базовых функций ---
	log.Print("Обычный лог (как Print)")
	log.Println("Лог с новой строкой")
	// log.Fatal("Лог + os.Exit(1)") // Программа завершится тут
	// log.Panic("Лог + panic()")   // Вызовет панику

	// --- 2. Умение выводить сообщения различных уровней ---
	// В стандартном пакете нет уровней, поэтому используем префиксы:
	log.SetPrefix("[INFO] ")
	log.Println("Сервер запущен на порту 8080")

	log.SetPrefix("[WARN] ")
	log.Println("Превышен лимит подключений, возможны задержки")

	log.SetPrefix("[ERROR] ")
	log.Println("Ошибка подключения к БД: connection refused")

	// --- 3. Понимание структуры и форматирования ---
	// log.SetFlags меняет структуру строки лога (добавляет дату, время, файл)
	// По умолчанию стоит: log.LstdFlags (дата и время)
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)

	log.SetPrefix("[DEBUG] ")
	log.Println("Проверка структуры лога: теперь есть микросекунды и файл:строка")
}

// 2. КАСТОМИЗАЦИЯ, ФАЙЛЫ И КОНТЕКСТ
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:
1. Вывод в разные источники:
В Go любой объект, реализующий интерфейс io.Writer, может быть целью для логов.
С помощью io.MultiWriter можно дублировать логи в файл, консоль и сетевой сервис разом.

2. Ротация логов:
Стандартный log.SetOutput просто пишет в файл. Если его не "ротировать" (архивировать старые,
создавать новые), файл сожрет всё место на диске.
В Go для этого стандарт — библиотека "lumberjack".

3. Контекст и TraceID:
В микросервисах важно видеть цепочку логов одного запроса. Для этого в каждый лог
прокидывается уникальный ID (TraceID/RequestID) из context.Context.

4. Идентификация горутин:
В Go специально нет простого способа получить ID горутины (философия языка).
Если это нужно для отладки, ID либо вытаскивают через хаки (runtime.Stack),
либо (что правильно) прокидывают свой CorrelationID через контекст.
*/

func logging() {
	//  Выбор формата и вывод в разные источники
	file, _ := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)

	// MultiWriter: пишем в консоль (stdout) и в созданный файл
	multi := io.MultiWriter(os.Stdout, file)

	// Создаем кастомный логгер (Writer, префикс, флаги)
	customLogger := log.New(multi, "CUSTOM", log.Ldate|log.Ltime|log.Lshortfile)
	customLogger.Println("Этот лог попадет и в консоль, и в файл app.log")

	// Ротация(концептуальный пример)
	/*
		В реальном проекте вместо os.OPenFile использовали бы lumberjack:
	*/
	log.SetOutput(&lumberjack.Logger{
		Filename:   "/var/log/myapp/foo.log",
		MaxSize:    500, // мегабайты
		MaxBackups: 3,
		MaxAge:     28,   // дни
		Compress:   true, //по умолчанию отключен
	})

	// 3. Использование контекста и TraceID
	ctx := context.WithValue(context.Background(), "traceID", "a1-b2-c3")

	// Функция-обертка, имитирующая логирование с контекстом
	logWithCtx(ctx, "Пользователь совершил покупку")
}

func logWithCtx(ctx context.Context, msg string) {
	traceID := ctx.Value("traceID")
	log.Printf("[TraceID: %v] %s", traceID, msg)
}

// 3. ПРОДВИНУТЫЕ БИБЛИОТЕКИ И ЦЕНТРАЛИЗАЦИЯ
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:
1. Зачем нужны Zap / Zerolog?
- Структурированность: Они пишут в JSON. Машине (Elasticsearch) проще читать
  {"level":"error", "msg":"db failure"}, чем обычный текст.
- Скорость: Uber Zap и Zerolog минимизируют аллокации в памяти (zero-allocation).
  Logrus сейчас считается устаревшим, так как он медленный.

2. Фильтрация и уровни (Performance):
В проде уровень логов обычно INFO. Если приложение под нагрузкой, мы не пишем DEBUG,
чтобы не тратить ресурсы на запись в файл/сеть. Это экономит CPU и диск.

3. Централизация (Сбор логов):
Логи с 10 серверов нельзя читать руками.
Схема: [App] -> [Fluentd/Logstash] -> [Elasticsearch] -> [Kibana] (стек ELK).
Или более современный: [App] -> [Promtail] -> [Grafana Loki].
*/

// GlobalLogger — в реальных проектах логгер часто инициализируется один раз при старте
var logger *zap.Logger

func InitLogger() {
	config := zap.NewProductionConfig()

	// Настраиваем формат времени (стандарт ISO8601 удобен для Kibana)
	config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Можно добавить глобальные поля, которые будут в каждом логе (версия сервиса, окружение)
	logger, _ = config.Build(zap.Fields(
		zap.String("service", "payment-gateway"),
		zap.String("env", "production"),
	))
}

// 1. ПРИМЕР: Middleware для автоматического логирования запросов с TraceID
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Генерируем или берем из заголовков TraceID
		traceID := r.Header.Get("X-Trace-ID")
		if traceID == "" {
			traceID = "gen-123" // В реальности тут UUID
		}

		// Кладем TraceID в контекст, чтобы он пробрасывался глубже в бизнес-логику
		ctx := context.WithValue(r.Context(), "trace_id", traceID)

		// Создаем логгер с уже привязанным TraceID для этого конкретного запроса
		requestLogger := logger.With(zap.String("trace_id", traceID))

		// Выполняем сам запрос
		next.ServeHTTP(w, r.WithContext(ctx))

		// ПРИМЕР: Логирование результата (Уровни и Фильтрация)
		duration := time.Since(start)
		requestLogger.Info("HTTP Request Finished",
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Duration("latency", duration),
			zap.Int("status", 200), // В реальности берется из ResponseWriter wrapper
		)
	})

}

// 2. Использование в бизнес-логике
func processPayment(ctx context.Context, amount int) {
	// Достаем trace_id из контекста (правильный путь для Middle+)
	traceID, _ := ctx.Value("trace_id").(string)

	l := logger.With(zap.String("trace_id", traceID))

	l.Debug("Starting payment validation") // Не выведется в Production (уровень Info)

	if amount < 0 {
		l.Error("Invalid payment amount",
			zap.Int("amount", amount),
			zap.String("user_id", "user_99"),
		)
		return
	}

	l.Info("Payment processed successfully")
}

// 4. ЭКСПЕРТНЫЙ УРОВЕНЬ, МОНИТОРИНГ И STACK TRACES
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:
1. Sampling (Сэмплирование): Если сервис ловит 10 000 ошибок в секунду, запись их всех
   убьет диск и забьет сеть. Сэмплирование позволяет записывать, например, только
   первые 10 логов одной категории, а остальные — каждый сотый.

2. Логирование как часть Observability: На этом уровне логи не живут сами по себе.
   Они обязаны содержать TraceID и SpanID для корреляции с распределенным трейсингом (Jaeger).

3. Безопасность (Data Masking): Сеньор должен гарантировать, что персональные данные (PII)
   не попадут в ELK. Это делается через кастомные энкодеры или обертки.
*/

//1. АРХИТЕКТУРНЫЙ ПРИМЕР: ГИБКИЙ ЛОГГЕР С РАЗДЕЛЕНИЕМ ПОТОКОВ

func newLogger() *zap.Logger {
	// Конфиг для JSON (для Kibana/Grafana)
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoder := zapcore.NewJSONEncoder(encoderConfig)

	// Разделение вывода:
	// Ошибки (Error+) льем в Stderr, всё остальное в Stdout.
	// Это стандарт для K8s, чтобы проще было фильтровать критические сбои.
	highPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= zapcore.ErrorLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl < zapcore.ErrorLevel && lvl >= zapcore.InfoLevel
	})

	consoleDebugging := zapcore.Lock(os.Stdout)
	consoleErrors := zapcore.Lock(os.Stderr)

	core := zapcore.NewTee(
		zapcore.NewCore(encoder, consoleErrors, highPriority),
		zapcore.NewCore(encoder, consoleDebugging, lowPriority),
	)

	//2. SAMPLING (Сэмплирование)
	// Записываем первые 5 одинаковых логов в секунду, далее — каждый 10-й.
	// Это спасет хранилище при циклической ошибке.
	samplerCore := zapcore.NewSamplerWithOptions(
		core,
		time.Second,
		5,  // initial
		10, // thereafter
	)

	return zap.New(samplerCore, zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
}

//3. AUDIT LOGGING (Безопасность и Бизнес-аналитика)

type auditEvent struct {
	Action string `json:"action"`
	Actor  string `json:"actor"`
	Target string `json:"target"`
}

func (a auditEvent) мarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("action", a.Action)
	enc.AddString("actor", a.Actor)
	enc.AddString("target", a.Target)
	return nil
}

func logSecurityEvent(logger *zap.Logger, event auditEvent) {
	// Используем ObjectMarshaler для максимальной производительности без рефлексии
	logger.Info("Audit Security Event", zap.Object("audit", event))
}

//4. DATA MASKING (Маскировка PII)

func maskedField(key string, value string) zap.Field {
	// Простая логика маскировки (на сеньор-собесе оценивают сам подход)
	if len(value) > 4 {
		value = value[:2] + "****" + value[len(value)-2:]
	}
	return zap.String(key, value)
}

func exampleUsage() {
	log := newLogger()
	defer log.Sync()

	// Пример 1: Использование TraceID из контекста (связка с Tracing)
	ctx := context.WithValue(context.Background(), "trace_id", "550e8400-e29b-41d4-a716-446655440000")
	tID := ctx.Value("trace_id").(string)

	l := log.With(zap.String("trace_id", tID))

	// Пример 2: Безопасное логирование (PII Masking)
	l.Info("User profile updated",
		maskedField("email", "sem-dev@google.com"), // Выведет se****om
	)

	// Пример 3: Аудит
	logSecurityEvent(log, auditEvent{
		Action: "delete_database",
		Actor:  "admin_id_77",
		Target: "prod_db_main",
	})
}
