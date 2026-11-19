package main

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/time/rate"
)

/*
Самый простой Rate Limiter (через time.Ticker)
Это классический пример. Суть: у нас есть канал, в который
каждые N миллисекунд падает «токен». Если токена нет — мы ждем.
*/
/*
func main() {
	ticker := time.NewTicker(1 * time.Nanosecond)
	defer ticker.Stop()

	tasks := []int{1, 2, 3, 4, 5, 6, 7, 8}

	for _, task := range tasks {
		<-ticker.C
		fmt.Printf("Выполняю задачу %d в %v\n", task, time.Now().Format("15:04:05.000"))
	}
}
*/

/*
Пример 2: Потокобезопасный Token Bucket (через Mutex)
Чтобы ограничить скорость всего сервиса (а не отдельной горутины), нам нужен общий стейт.
Token Bucket (Ведро с токенами) работает так:
У нас есть «ведро» (переменная tokens).
Оно пополняется с определенной скоростью (например, 1 токен в секунду).
Есть максимальный размер (capacity), чтобы ведро не переполнилось.
Когда горутина хочет выполнить задачу, она берет токен. Если токенов нет — ждет.
*/
/*
type tokenBucket struct {
	mu         sync.Mutex
	tokens     int
	maxTokens  int
	refillRate time.Duration
}

func newTokenBucket(maxTokens int, refillRate time.Duration) *tokenBucket {
	tb := &tokenBucket{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
	}

	go func() {
		ticker := time.NewTicker(tb.refillRate)
		defer ticker.Stop()
		for {
			<-ticker.C
			tb.mu.Lock()
			if tb.tokens < tb.maxTokens {
				tb.tokens++
			}
			tb.mu.Unlock()
		}
	}()
	return tb
}

func (tb *tokenBucket) allow() bool {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	if tb.tokens > 0 {
		tb.tokens--
		return true
	}
	return false
}

func main() {
	limiter := newTokenBucket(10, 250*time.Millisecond)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()
			if limiter.allow() {
				fmt.Printf("Задача %d: ВЫПОЛНЕНА\n", taskID)
			} else {
				fmt.Printf("Задача %d: ОТКЛОНЕНА (лимит)\n", taskID)
			}
		}(i)
	}
	wg.Wait()
}
*/

/*
Пример 3: Блокирующий Rate Limiter (через context)
В реальной жизни мы не хотим отклонять запрос. Мы хотим подождать, пока токен
не появится. Для этого используется context (чтобы клиент мог отменить ожидание).
*/
/*
type blockingRateLimiter struct {
	ticker *time.Ticker
}

func newBlockingRateLimiter(interval time.Duration) *blockingRateLimiter {
	return &blockingRateLimiter{
		ticker: time.NewTicker(interval),
	}
}

func (rl *blockingRateLimiter) wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-rl.ticker.C:
		return nil
	}
}

func (rl *blockingRateLimiter) stop() {
	rl.ticker.Stop()
}

func main() {
	limiter := newBlockingRateLimiter(time.Second)
	defer limiter.stop()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for i := 0; i < 10; i++ {
		err := limiter.wait(ctx)
		if err != nil {
			fmt.Printf("Задача %d: отменена, причина: %v\n", i, err)
			break
		}
		fmt.Printf("Задача %d: ВЫПОЛНЕНА\n", i)
	}
}
*/

/*
Пример 4: Production-Ready с golang.org/x/time/rate (Самый главный)
*/
func main() {
	limiter := rate.NewLimiter(rate.Limit(3), 8)

	for i := 0; i < 20; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)

		err := limiter.Wait(ctx)
		cancel()

		if err != nil {
			fmt.Printf("Задача %d: отменена (timeout)\n", i)
			continue
		}
		fmt.Printf("Задача %d: ВЫПОЛНЕНА в %v\n", i, time.Now().Format("15:04:05.000"))
	}
	fmt.Println("Проверка без ожидания")

	for i := 0; i < 3; i++ {
		if limiter.Allow() {
			fmt.Printf("Запрос %d: ПРОПУЩЕН\n", i)
		} else {
			fmt.Printf("Запрос %d: ОТКЛОНЕН (429 Too Many Requests)\n", i)
		}
	}
}
