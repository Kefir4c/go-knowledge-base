package main

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// запрос к HTTP с таймаутом через select.

func fetchWithRetry(ctx context.Context, url string, maxRetries int, timeout time.Duration) (string, error) {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Проверяем, не отменён ли контекст
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:

		}

		// Каналы для одного запроса
		result := make(chan string)
		errCh := make(chan error)

		// Запускаем запрос
		go func() {
			resp, err := http.Get(url)
			if err != nil {
				errCh <- err
				return
			}
			defer resp.Body.Close()
			result <- resp.Status
		}()

		// Ожидаем результат, ошибку или таймаут
		select {
		case status := <-result:
			return status, nil
		case err := <-errCh:
			lastErr = err
			if attempt == maxRetries {
				return "", fmt.Errorf("последняя ошибка: %w", err)
			}
		case <-time.After(timeout):
			lastErr = fmt.Errorf("timeout")
			if attempt == maxRetries {
				return "", fmt.Errorf("таймаут после %d попыток", maxRetries)
			}
		case <-ctx.Done():
			return "", ctx.Err()
		}

		// Задержка перед ретраем (с учётом контекста)
		delay := time.Duration(attempt) * 100 * time.Millisecond
		select {
		case <-time.After(delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return "", lastErr
}

func main() {
	// Контекст с общим таймаутом на всё
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "https://httpbin.org/delay/2" // 2 секунды задержки
	status, err := fetchWithRetry(ctx, url, 3, 1*time.Second)

	if err != nil {
		fmt.Println("Ошибка:", err)
	}

	fmt.Println("Успех:", status)
}
