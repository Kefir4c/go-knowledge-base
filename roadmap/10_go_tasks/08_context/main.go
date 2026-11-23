package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync/atomic"
	"time"
)

/*
Context Cancellation (Отмена, когда клиент ушел)
Представь, что клиент делает HTTP-запрос, который обращается к БД (долгая операция).Если клиент
закрыл вкладку браузера, сервер должен перестать тратить ресурсы на эту операцию и отменить её.
*/
/*
func loadDBQuery(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("запрос в БД отменен: %w", ctx.Err())
	case <-time.After(time.Second * 5):
		return "Данные из БД", nil
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(2 * time.Second)
		fmt.Println("Клиент закрыл вкладку! Отменяем операцию...")
		cancel()
	}()

	res, err := loadDBQuery(ctx)
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	} else {
		fmt.Printf("Результат: %s\n", res)
	}
}
*/

/*
Timeout Wrapper (Таймаут для конкретной задачи)
У тебя есть функция, которая может зависнуть навсегда.Ты хочешь
обернуть её в таймаут, чтобы она не повесила весь сервис.
*/
/*
func withTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	return fn(timeoutCtx) // fn обязан использовать timeoutCtx
}

func doWork(ctx context.Context) error {
	select {
	case <-time.After(3 * time.Second):
		fmt.Println("Работа успешно завершена")
		return nil
	case <-ctx.Done():
		return fmt.Errorf("работа прервана по таймауту: %w", ctx.Err())
	}
}

func main() {
	err := withTimeout(context.Background(), 2*time.Second, doWork)
	if err != nil {
		fmt.Printf("Ошибка: %v\n", err)
	}
}
*/

/*
Retry с Exponential Backoff (Повтор с задержкой)
Внешние API часто падают с ошибками 500 или 429 Too Many Requests. Вместо того чтобы сразу сдаваться,
мы пробуем еще раз, но с увеличивающейся задержкой (1 сек, 2 сек, 4 сек...), чтобы не долбить сервер.
*/
/*
func retryWitchBackoff(ctx context.Context, retries int, fn func(ctx context.Context) error) error {
	var lastErr error

	for i := 0; i < retries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		err := fn(ctx)
		if err == nil {
			return nil
		}
		lastErr = err

		delay := time.Duration(math.Pow(2, float64(i))) * time.Second
		fmt.Printf("Попытка %d упала: %v. Ждём %v...\n", i+1, err, delay)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
	}
	return fmt.Errorf("все %d попытки провалены. Последняя ошибка: %w", retries, lastErr)
}

func main() {
	attempt := 0
	failingFn := func(ctx context.Context) error {
		attempt++
		if attempt < 8 {
			return fmt.Errorf("ошибка сети (500)")
		}
		fmt.Println("Успешно!")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	err := retryWitchBackoff(ctx, 5, failingFn)
	if err != nil {
		fmt.Printf("Провал: %v\n", err)
	}
}
*/

/*
Комбинированный (Timeout + Retry + Cancellation)
В реальной жизни вам нужно защитить задачу от всех трех проблем одновременно. Например, скачивание файла
по HTTP. Оно может: зависнуть навсегда (таймаут), упасть с ошибкой (retry), быть отменено пользователем (cancel).
*/
/*
func executeWithPolicy(ctx context.Context, taskName string, fn func(ctx context.Context) error) error {
	maxRetries := 5
	taskTimeout := 2 * time.Second
	var lastErr error

	for i := 0; i < maxRetries; i++ {
		if ctx.Err() != nil {
			if lastErr != nil {
				return fmt.Errorf("%w (отменено: %v)", lastErr, ctx.Err())
			}
			return ctx.Err()
		}

		attemptCtx, cancel := context.WithTimeout(ctx, taskTimeout)
		err := fn(attemptCtx)
		cancel()

		if err == nil {
			return nil
		}

		lastErr = err

		if errors.Is(err, context.Canceled) {
			return fmt.Errorf("[%s] отменено контекста: %w", taskName, err)
		}
		fmt.Printf("[%s] Ошибка: %v. Ждем перед следующей попыткой...\n", taskName, err)

		base := time.Duration(i+1) * 500 * time.Millisecond
		jitter := time.Duration(rand.Int63n(int64(base / 2)))
		backoff := base + jitter
		select {
		case <-ctx.Done():
			return fmt.Errorf("[%s] отменено во время backoff (последняя ошибка: %w): %w", taskName, lastErr, ctx.Err())
		case <-time.After(backoff):
		}
	}
	return fmt.Errorf("[%s] все %d попытки провалены. Последняя ошибка: %w", taskName, maxRetries, lastErr)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		time.Sleep(2 * time.Second)
		fmt.Println("\nКлиент отменил запрос!")
		cancel()
	}()

	attempt := 0
	unstableFn := func(ctx context.Context) error {
		attempt++

		time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)

		if attempt == 1 {
			return fmt.Errorf("timeout: база данных не ответила")
		}
		if attempt == 2 {
			return fmt.Errorf("500 Internal Server Error")
		}
		fmt.Println("Успешно обработано!")
		return nil
	}

	err := executeWithPolicy(ctx, "DownloadFile", unstableFn)
	if err != nil {
		fmt.Printf("Итог: %v\n", err)
	} else {
		fmt.Println("Итог: Успех")
	}
}
*/

/*"Трассировка запроса (TraceID)"
Условие:
Нам нужно написать middleware для HTTP-сервера, которое:
Генерирует уникальный TraceID (например, uuid).
Кладёт его в context.
Передает контекст вглубь по вызовам функций (имитация слоев: Handler -> Service -> Repository).
На каждом слое логирует сообщение с этим TraceID, чтобы мы могли отследить весь путь запроса.
*/
/*
type ctxKey string

const traceIDKey ctxKey = "traceID"

func generateTraceID() string {
	return uuid.New().String()
}

func withTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

func getTraceID(ctx context.Context) string {
	if id, ok := ctx.Value(traceIDKey).(string); ok {
		return id
	}
	return "unknown"
}

func repository(ctx context.Context) {
	traceID := getTraceID(ctx)
	fmt.Printf("[REPO] TraceID: %s Сохраняю данные в БД\n", traceID)
	time.Sleep(50 * time.Millisecond)
}

func service(ctx context.Context) {
	traceID := getTraceID(ctx)
	fmt.Printf("[SERVICE] TraceID: %s -> Обрабатываю бизнес-логику\n", traceID)
	time.Sleep(50 * time.Millisecond)

	repository(ctx)
}

func handler(w http.ResponseWriter, r *http.Request) {
	traceID := r.Header.Get("X-Request-ID")
	if traceID == "" {
		traceID = generateTraceID()
	}

	ctx := withTraceID(r.Context(), traceID)

	fmt.Printf("[HANDLER] TraceID: %s -> Получен запрос от клиента\n", traceID)

	service(ctx)

	w.Header().Set("X-Trace-ID", traceID)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK\n"))
}

func main() {
	http.HandleFunc("/", handler)
	fmt.Println("Сервер запущен на :8080")

	go func() {
		time.Sleep(100 * time.Millisecond)
		for i := 1; i <= 3; i++ {
			fmt.Printf("\n КЛИЕНТ: Отправляю запрос %d \n", i)
			http.Get("http://localhost:8080/")
			time.Sleep(200 * time.Millisecond)
		}
	}()

	http.ListenAndServe(":8080", nil)
}
*/
/*
HTTP Client с Retry, Timeout и Circuit Breaker (40 минут)
Напиши HTTP-клиент для внешнего API, который:
Делает GET-запрос с таймаутом 2 секунды на попытку
Retry до 3 раз с экспоненциальным backoff (1с, 2с, 4с) + jitter
Если API возвращает 5xx или 429 — retry. Если 4xx — не retry (фатально)
Если 3 попытки подряд упали — "circuit breaker" (не делаем запросы 10 секунд, сразу возвращаем ошибку)
Всё это должно корректно отменяться по ctx
После успешного запроса circuit breaker сбрасывается
*/

var ErrCircuitOpen = errors.New("circuit breaker open")

type Client struct {
	httpClient *http.Client

	consecutiveFailures atomic.Int32
	lastFailureTime     atomic.Int64
	circuitOpen         atomic.Bool

	maxRetries     int
	baseBackoff    time.Duration
	breakerTimeout time.Duration
}

func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		maxRetries:     3,
		baseBackoff:    1 * time.Second,
		breakerTimeout: 10 * time.Second,
	}
}

func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	if err := c.checkCircuitBreaker(); err != nil {
		return nil, err
	}

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("ctx отменён на попытке %d: %w", attempt+1, err)
		}

		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		body, err := c.doRequest(attemptCtx, url)
		cancel()

		if err == nil {
			c.resetCircuitBreaker()
			return body, nil
		}

		lastErr = err
		c.recordFailure()

		if !isRetryable(err) {
			return nil, fmt.Errorf("фатальная ошибка: %w", err)
		}

		if attempt < c.maxRetries-1 {
			backoff := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}
	}
	return nil, fmt.Errorf("все попытки исчерпаны: %w", lastErr)
}

func (c *Client) doRequest(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("таймаут: %w", err)
		}
		return nil, fmt.Errorf("сеть: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return body, nil
	case resp.StatusCode == 429:
		return nil, errors.New("rate limit 429")
	case resp.StatusCode >= 500:
		return nil, fmt.Errorf("server error %d", resp.StatusCode)
	default: // 4xx
		return nil, fmt.Errorf("client error %d", resp.StatusCode)
	}
}

func isRetryable(err error) bool {
	errStr := err.Error()
	return !contains(errStr, "client error 4")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && contains(s[1:], substr) || s[:len(substr)] == substr)
}

func (c *Client) calculateBackoff(attempt int) time.Duration {
	base := c.baseBackoff * (1 << attempt) // 2^attempt
	jitter := time.Duration(rand.Int63n(int64(base / 2)))
	return base + jitter
}

func (c *Client) checkCircuitBreaker() error {
	if !c.circuitOpen.Load() {
		return nil
	}
	lastFail := time.Unix(0, c.lastFailureTime.Load())
	if time.Since(lastFail) > c.breakerTimeout {
		c.circuitOpen.Store(false)
		c.consecutiveFailures.Store(0)
		return nil
	}
	return fmt.Errorf("%w, подождите %v", ErrCircuitOpen, c.breakerTimeout-time.Since(lastFail))
}

func (c *Client) recordFailure() {
	c.consecutiveFailures.Add(1)
	c.lastFailureTime.Store(time.Now().UnixNano())
	if c.consecutiveFailures.Load() >= 3 {
		c.circuitOpen.Store(true)
	}
}

func (c *Client) resetCircuitBreaker() {
	c.consecutiveFailures.Store(0)
	c.circuitOpen.Store(false)
}
