package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Result – результат обработки одного URL.
type Result struct {
	URL        string
	StatusCode int
	BodySize   int64
	Error      error
	Duration   time.Duration
}

func Crawl(ctx context.Context, urls []string, maxConcurrent int) <-chan Result {
	out := make(chan Result, len(urls)) // буферизированный, чтобы не блокировать отправителей

	// Семафор: буферизированный канал, который ограничивает количество активных горутин.
	sem := make(chan struct{}, maxConcurrent)

	// Запускаем горутину, которая будет отправлять задачи в воркеры.
	go func() {
		defer close(out) // закрываем канал после завершения всех задач
		var wg sync.WaitGroup

		for _, url := range urls {
			// Проверяем, не отменён ли контекст перед запуском новой горутины.
			select {
			case <-ctx.Done():
				// Если отменён – прекращаем отправку новых задач.
				break
			default:
			}

			wg.Add(1)
			// Захватываем URL для замыкания.
			url := url

			go func() {
				defer wg.Done()

				// Занимаем слот семафора (блокируется, если уже maxConcurrent воркеров активны).
				select {
				case sem <- struct{}{}:
					// Получили разрешение – идём дальше.
				case <-ctx.Done():
					// Если контекст отменён – не начинаем новую работу.
					return
				}
				// Освобождаем слот после завершения работы.
				defer func() { <-sem }()

				// Выполняем HTTP-запрос с учётом контекста.
				result := fetch(ctx, url)

				// Отправляем результат в выходной канал.
				select {
				case out <- result:
				case <-ctx.Done():
					// Если контекст отменён – прекращаем отправку (но результат уже получен).
				}
			}()
		}
		// Ждём завершения всех запущенных горутин.
		wg.Wait()
	}()
	return out
}

// fetch выполняет HTTP GET-запрос к URL с учётом контекста.
// Возвращает заполненную структуру Result.
func fetch(ctx context.Context, url string) Result {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Result{
			URL:      url,
			Error:    fmt.Errorf("create request: %w", err),
			Duration: time.Since(start),
		}
	}

	client := &http.Client{Timeout: 30 * time.Second} // дополнительная защита
	resp, err := client.Do(req)
	if err != nil {
		return Result{
			URL:      url,
			Error:    err,
			Duration: time.Since(start),
		}
	}
	defer resp.Body.Close()

	// Читаем тело ответа (можно ограничить размер, но для простоты читаем всё).
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Result{
			URL:        url,
			StatusCode: resp.StatusCode,
			Error:      fmt.Errorf("read body: %w", err),
			Duration:   time.Since(start),
		}
	}

	return Result{
		URL:        url,
		StatusCode: resp.StatusCode,
		BodySize:   int64(len(body)),
		Error:      nil,
		Duration:   time.Since(start),
	}
}

func main() {
	urls := []string{
		"https://google.com",
		"https://github.com",
		"https://stackoverflow.com",
		"https://httpbin.org/delay/3", // задерживается на 3 секунды
		"https://nonexistent.example.com",
	}

	// Контекст, который отменяется по Ctrl+C.
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	// Запускаем краулер с ограничением 2 параллельных запроса.
	results := Crawl(ctx, urls, 2)

	// Обрабатываем результаты по мере поступления.
	for res := range results {
		if res.Error != nil {
			fmt.Printf("[ERR] %s: %v (duration %v)\n", res.URL, res.Error, res.Duration)
		}
		fmt.Printf("[OK] %s -> %d, body size %d bytes (%v)\n",
			res.URL, res.StatusCode, res.BodySize, res.Duration)
	}
	fmt.Println("\nCrawling finished (cancelled or all done).")
}
