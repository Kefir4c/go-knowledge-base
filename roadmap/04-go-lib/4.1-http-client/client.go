package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"syscall"
	"time"
)

/*
HTTP КЛИЕНТ В GO: ОСНОВНЫЕ ПОНЯТИЯ

http.Client — основная структура для выполнения HTTP-запросов.
Она управляет: таймаутами, куками, редиректами, keep-alive, прокси.

ПОЧЕМУ http.DefaultClient НЕЛЬЗЯ ИСПОЛЬЗОВАТЬ В PRODUCTION?

  • DefaultClient не имеет таймаутов — запрос может висеть вечно.
  • DefaultClient использует Transport, который не лимитирует соединения.
  • DefaultClient может не закрыть соединение при ошибке.
  • DefaultClient разделяется всей программой, настройки нельзя изменить.

Вот что по умолчанию у DefaultClient:
  client := &http.Client{
      Transport: &http.Transport{
          MaxIdleConns:    100,
          IdleConnTimeout: 90 * time.Second,
          TLSClientConfig: nil,
      },
      Timeout: 0, // <--- НЕТ ТАЙМАУТА! Это опасно.
  }

ТАЙМАУТЫ В HTTP КЛИЕНТЕ

Таймауты бывают на разных уровнях:

1. http.Client.Timeout — общий таймаут на весь запрос (включая чтение тела).
2. context.WithTimeout — передаётся в запрос, позволяет отменить его в любой момент.
3. http.Transport.TLSHandshakeTimeout — таймаут на TLS рукопожатие.
4. http.Transport.ResponseHeaderTimeout — таймаут на получение заголовков ответа.
5. http.Transport.DialContext — таймаут на установку TCP соединения.

РЕКОМЕНДАЦИЯ: ВСЕГДА устанавливай таймауты на всех уровнях.

HTTP.TRANSPORT И CONNECTION POOLING

Транспорт управляет пулом соединений (keep-alive). По умолчанию он:
  • Переиспользует соединения (MaxIdleConnsPerHost = 2)
  • Держит idle-соединения 90 секунд (IdleConnTimeout)

Для высоких нагрузок нужно настраивать:
  MaxIdleConns        — максимальное количество idle-соединений (всего)
  MaxIdleConnsPerHost — для одного хоста (по умолч. 2 — мало!)
  MaxConnsPerHost     — лимит соединений к одному хосту (включая активные)
  IdleConnTimeout     — как долго держать idle-соединение
  DisableKeepAlives   — отключить keep-alive (почти никогда не нужно)

ПРИМЕР НАСТРОЙКИ ДЛЯ PRODUCTION:
  transport := &http.Transport{
      MaxIdleConns:        100,
      MaxIdleConnsPerHost: 20,
      MaxConnsPerHost:     50,
      IdleConnTimeout:     90 * time.Second,
      TLSHandshakeTimeout: 10 * time.Second,
      ResponseHeaderTimeout: 30 * time.Second,
      ExpectContinueTimeout: 1 * time.Second,
      DialContext: (&net.Dialer{
          Timeout:   30 * time.Second,
          KeepAlive: 30 * time.Second,
      }).DialContext,
  }
  client := &http.Client{
      Timeout:   60 * time.Second,
      Transport: transport,
  }

ПОВТОРЫ (RETRY)

http.Client сам НЕ ДЕЛАЕТ повторных запросов. Нужно реализовать вручную.

Какие ошибки стоит ретраить:
  • Таймауты (net.Error.Timeout())
  • Временные сбои (net.Error.Temporary())
  • EOF, connection reset, broken pipe
  • 5xx ошибки (кроме 501, 505)

Какие ошибки НЕ стоит ретраить:
  • 4xx (клиентские ошибки)
  • context.Canceled
  • Ошибки парсинга URL

Пример с ретраем и exponential backoff см. в примерах.
*/

// 1. ПЛОХОЙ КЛИЕНТ (БЕЗ ТАЙМАУТОВ, НЕ ДЛЯ PRODUCTION)
func badClientEx() {
	// ⚠️ Никогда так не делайте в продакшене!
	resp, err := http.Get("https://httpbin.org/delay/10") // зависнет на 10 секунд
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Println("Response:", string(body)[:50])
}

func primer1() {
	fmt.Println("-----Primer1-----")
	fmt.Println("Этот запрос зависнет на 10 секунд, но программа не прервётся.")
	fmt.Println("Запускать не рекомендуется, показываем только теорию.")
	// badClientExample() // раскомментировать с осторожностью
}

// 2. ХОРОШИЙ КЛИЕНТ С ТАЙМАУТАМИ
func goodClientEx() {
	// Настраиваем Transport с таймаутами
	transport := http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second, //таймаут установки TCP соединения
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   5 * time.Second,  // таймаут TLS рукопожатия
		ResponseHeaderTimeout: 10 * time.Second, //таймаут получения заголовков
		ExpectContinueTimeout: 1 * time.Second,
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
	}

	client := &http.Client{
		Timeout:   15 * time.Second, // общий таймаут на запрос
		Transport: &transport,
	}

	// Создаём запрос с контекстом (можно дополнительно ограничить)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://httpbin.org/delay/3", nil)
	if err != nil {
		fmt.Println("Create request error:", err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Do request error:", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %s, Body length: %d\n", resp.Status, len(body))

}

func primer2() {
	fmt.Println("-----Primer2-----")
	goodClientEx()
}

// 3. КЛИЕНТ С ПОВТОРАМИ (RETRY)
// retryableError проверяет, стоит ли повторять запрос при ошибке
func retryableError(err error) bool {
	if err == nil {
		return false
	}

	// Проверяем на временные сетевые ошибки (включая таймауты)
	var netErr net.Error
	if errors.As(err, &netErr) && (netErr.Timeout() || netErr.Temporary()) {
		return true
	}

	// Другие временные ошибки
	switch err {
	case context.DeadlineExceeded, io.EOF, syscall.ECONNRESET, syscall.ECONNABORTED:
		return true
	}
	return false
}

// DoWithRetry выполняет HTTP-запрос с повторными попытками
func DoWithRetry(ctx context.Context, client *http.Client, req *http.Request, maxRetries int, backoff time.Duration) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt < maxRetries; attempt++ {
		// Важно: создавайте копию запроса, если тело нужно читать несколько раз.
		// Для простоты примера предполагаем, что тело не используется или не изменяется.
		resp, err := client.Do(req.WithContext(ctx))
		if err != nil {
			lastErr = err
			if !retryableError(err) {
				return nil, err // Не пытаемся повторить
			}
			if attempt == maxRetries-1 {
				break // Последняя попытка, выходим
			}
			// Ждём с экспоненциальной задержкой
			select {
			case <-time.After(backoff * time.Duration(1<<attempt)):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			continue
		}
		// Успешный ответ
		return resp, nil
	}
	return nil, fmt.Errorf("failed after %d attempts: %w", maxRetries, lastErr)
}

// primer3 — демонстрация использования DoWithRetry
func primer3() {
	fmt.Println("-----Primer3-----")
	transport := &http.Transport{
		DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext,
	}
	client := &http.Client{
		Timeout:   10 * time.Second,
		Transport: transport,
	}

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://httpbin.org/status/500", nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	resp, err := DoWithRetry(ctx, client, req, 3, 100*time.Millisecond)
	if err != nil {
		fmt.Println("Request failed after retries:", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("Response status:", resp.Status)
}

// 4. НАСТРОЙКА КЛИЕНТА ДЛЯ ВЫСОКОЙ НАГРУЗКИ (CONNECTION POOLING)
func primer4() {
	fmt.Println("-----Primer4-----")
	transport := &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		MaxConnsPerHost:     50,
		IdleConnTimeout:     90 * time.Second,
		DisableKeepAlives:   false,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	client := &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}

	var wg sync.WaitGroup
	urls := []string{
		"https://httpbin.org/get",
		"https://httpbin.org/ip",
		"https://httpbin.org/user-agent",
	}

	for _, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			resp, err := client.Get(url)
			if err != nil {
				fmt.Printf("Error %s: %v\n", url, err)
				return
			}
			defer resp.Body.Close()
			fmt.Printf("%s -> %s\n", url, resp.Status)
		}(url)
	}
	wg.Wait()
}

// 5. ИСПОЛЬЗОВАНИЕ КАСТОМНОГО ТРАНСПОРТА ДЛЯ ТРЕЙСИНГА

// tracingTransport оборачивает http.RoundTripper для логирования запросов
type tracingTransport struct {
	transport http.RoundTripper
}

func (t *tracingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	fmt.Printf("Request: %s %s", req.Method, req.URL)
	resp, err := t.transport.RoundTrip(req)
	duration := time.Since(start)
	if err != nil {
		fmt.Printf("Request failed: %v (duration %v)", err, duration)
	} else {
		fmt.Printf("Response: %s (duration %v)", resp.Status, duration)
	}
	return resp, err
}

func primer5() {
	fmt.Println("-----Primer5-----")
	transport := &tracingTransport{
		transport: http.DefaultTransport,
	}
	client := http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}
	resp, err := client.Get("https://httpbin.org/get")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer resp.Body.Close()
	fmt.Println("Request completed")
}

func main() {
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
}

/*
 ШПАРГАЛКА ПО NET/HTTP КЛИЕНТ

 1. Всегда создавай свой http.Client с таймаутами.
    NEVER use http.DefaultClient in production.

 2. Таймауты:
    - client.Timeout — общий
    - transport.DialContext — TCP соединение
    - transport.TLSHandshakeTimeout — TLS
    - transport.ResponseHeaderTimeout — заголовки ответа
    - context.WithTimeout — гибкое управление

 3. Connection Pooling:
    - transport.MaxIdleConnsPerHost (по умолч. 2) — увеличь до 10-50
    - transport.IdleConnTimeout — 90 сек
    - transport.MaxConnsPerHost — лимит к хосту

 4. Повторы (retry):
    - Делать только на идемпотентные методы (GET, PUT, DELETE)
    - Использовать exponential backoff
    - Проверять на временные ошибки (net.Error.Timeout)

 5. Закрывай resp.Body всегда!
    defer resp.Body.Close()

 6. Для больших нагрузок используй один общий Transport,
    т.к. он переиспользует соединения (keep-alive).
*/
