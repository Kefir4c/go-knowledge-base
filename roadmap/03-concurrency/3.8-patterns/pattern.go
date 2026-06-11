package main

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

/*
ПАТТЕРНЫ КОНКУРЕНТНОСТИ В GO
Паттерн (шаблон, образец) — это типовое проверенное решение часто встречающейся проблемы в определенном контексте.

Ниже собраны основные паттерны, которые нужно знать для собеседования и реальной работы.
Каждый паттерн решает свою типовую задачу.
*/

/*
1. Worker Pool (Пул воркеров) — это паттерн параллельного программирования, где создается фиксированное количество горутин (воркеров),
которые обрабатывают задачи из общего канала.

ПРОБЛЕМА:
  - Для каждой задачи создаётся новая горутина
  - При 10000 задач → 10000 горутин
  - Каждая горутина потребляет память (~2KB стека)
  - Растёт нагрузка на планировщик Go

РЕШЕНИЕ:
  - Создаём фиксированное количество горутин (воркеров)
  - Задачи отправляются в общий канал
  - Воркеры разбирают задачи и отправляют результаты

КОГДА ИСПОЛЬЗОВАТЬ:
  - Много независимых задач (обработка изображений, email-рассылки)
  - Нужно ограничить параллелизм (не перегружать БД или внешнее API)
  - Задачи не зависят друг от друга
*/

type workerPoolResult struct {
	id     int
	value  int
	worker int
}

func workerPool() {
	const (
		numTasks   = 10
		numWorkers = 3
		bufferSize = 5
	)
	var wg sync.WaitGroup

	tasks := make(chan int, bufferSize)
	results := make(chan workerPoolResult, bufferSize)

	// ЗАПУСК ВОРКЕРОВ
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)

		go func(workerID int) {
			defer wg.Done()

			for task := range tasks {
				result := workerPoolResult{
					id:     task,
					value:  task * 2,
					worker: workerID,
				}
				results <- result
			}
		}(w)
	}

	// ОТПРАВКА ЗАДАЧ
	for i := 1; i <= numTasks; i++ {
		tasks <- i
	}
	close(tasks)

	// ЗАКРЫТИЕ КАНАЛА РЕЗУЛЬТАТОВ
	go func() {
		wg.Wait()
		close(results)
	}()

	//Сбор результатов
	for res := range results {
		fmt.Printf("Task: %d, Value: %d, Worker: %d\n", res.id, res.value, res.worker)
	}
}

/*
2. PIPELINE (КОНВЕЙЕР)
  ПРОБЛЕМА:
  - Нужно обработать данные последовательно, но параллельно на каждом этапе
  - Классический пример: прочитать файл → распарсить → валидировать → сохранить

РЕШЕНИЕ:
  - Данные проходят через цепочку обработчиков
  - Каждый этап — отдельная горутина
  - Следующий этап стартует до завершения предыдущего

КЛЮЧЕВЫЕ ОСОБЕННОСТИ:
  - Fan-in: несколько источников → один канал (сбор данных)
  - Fan-out: один источник → несколько обработчиков (распараллеливание)

ОТЛИЧИЯ ОТ WORKER POOL:
   - Pipeline — последовательная обработка (каждый элемент проходит все этапы)
   - Worker Pool — параллельная обработка (много воркеров делают одно и то же)

Pipeline — один из паттернов, который ИДЕАЛЬНО ложится на контекст.
*/

/*
PIPELINE — БАЗОВЫЙ (обязательно выучить)
Суть: данные проходят через цепочку обработчиков, каждый этап — своя горутина.
*/

func pipelineBasic() {
	// Этап 1: генератор чисел (источник данных)
	gen := func(nums ...int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for _, n := range nums {
				out <- n
			}
		}()
		return out
	}

	// Этап 2: возведение в квадрат (трансформация)
	square := func(in <-chan int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for n := range in {
				out <- n * n
			}
		}()
		return out
	}

	for result := range square(gen(1, 2, 3, 4, 5)) {
		fmt.Println(result)
	}
}

/*
PIPELINE — МНОГОЭТАПНЫЙ

Фишки:
- Несколько этапов подряд
- Можно комбинировать любые трансформации
*/

func pipelineMultiStage() {
	// Генератор
	gen := func(nums ...int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for _, n := range nums {
				out <- n
			}
		}()
		return out
	}

	// Умножение на число
	multiply := func(in <-chan int, factor int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for n := range in {
				out <- n * factor
			}
		}()
		return out
	}

	// Фильтр только для четных чисел
	evenOnly := func(in <-chan int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for n := range in {
				if n%2 == 0 {
					out <- n
				}
			}
		}()
		return out
	}

	// Суммирование (накопление)
	sum := func(in <-chan int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			total := 0
			for n := range in {
				total += n
			}
			out <- total
		}()
		return out
	}

	// Цепочка: gen → multiply(2) → evenOnly → sum
	numbers := gen(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	multiplied := multiply(numbers, 2)
	filtered := evenOnly(multiplied)
	total := sum(filtered)

	for result := range total {
		fmt.Println("Сумма четных чисел ×2:", result)
		// 2+4+6+8+10+12+14+16+18+20 = 110
	}
}

/*
PIPELINE — FAN-OUT / FAN-IN

Когда использовать:
- Один этап обработки очень медленный
- Нужно распараллелить процесс на несколько горутин

FAN-OUT (РАЗВЕТВЛЕНИЕ):
  - Один входной канал, несколько горутин читают из него
  - Каждая задача обрабатывается только одним воркером
  - Позволяет распараллелить обработку

FAN-IN (СЛИЯНИЕ):
  - Несколько входных каналов, один выходной канал
  - Собирает результаты от всех воркеров в один поток
  - Использует sync.WaitGroup для ожидания всех источников

КОГДА ИСПОЛЬЗОВАТЬ:
  - Fan-out: когда обработка одной задачи — узкое место
  - Fan-in: когда нужно собрать результаты от параллельных обработчиков

КЛЮЧЕВОЙ МОМЕНТ:
Данные из numbers распределяются автоматически между worker1, worker2, worker3.
Одно число попадает только в одного worker'а.
За счет этого достигается параллелизм.

Сравнение:
- Без Fan-out: 10 операций по 1 сек = 10 сек
- С Fan-out (3 worker): 10 операций распределяются на 3 worker'a ≈ 3-4 сек
*/

func pipelineFanOutFanIn() {
	// Генератор (источник)
	gen := func(nums ...int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for _, n := range nums {
				out <- n
			}
		}()
		return out
	}
	// Тяжелый обработчик (имитируем медленную операцию)
	worker := func(id int, in <-chan int) <-chan int {
		out := make(chan int)
		go func() {
			defer close(out)
			for n := range in {
				result := n * n
				fmt.Printf("Worker %d обработал %d\n", id, n)
				out <- result
			}
		}()
		return out
	}

	// Fan-in: слияние нескольких каналов в один
	merge := func(channels ...<-chan int) <-chan int {
		out := make(chan int)
		var wg sync.WaitGroup

		for _, ch := range channels {
			wg.Add(1)
			go func(c <-chan int) {
				defer wg.Done()
				for val := range c {
					out <- val
				}
			}(ch)
		}

		go func() {
			wg.Wait()
			close(out)
		}()
		return out
	}

	// Fan-out: запускаем 3 параллельных worker'а
	numbers := gen(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)

	worker1 := worker(1, numbers)
	worker2 := worker(2, numbers)
	worker3 := worker(3, numbers)

	results := merge(worker1, worker2, worker3)

	for result := range results {
		fmt.Println("Результат:", result)
	}
}

/*
3. CONTEXT С ТАЙМАУТОМ

ЗАЧЕМ ЭТО НУЖНО:
- Любая операция может зависнуть (HTTP запрос, БД, чтение файла)
- Без таймаута горутина будет висеть вечно → утечка памяти
- Таймаут — это защита от "вечно висящих" операций

ЧТО ТАКОЕ CONTEXT:
- Стандартный способ управления временем жизни в Go
- Несет с собой: дедлайны, таймауты, сигналы отмены
- Передается в функции как первый аргумент (по конвенции)

ПОЧЕМУ НУЖНО ВЫЗЫВАТЬ cancel() В defer:
- Каждый WithTimeout/WithCancel создает внутреннюю структуру
- Если не вызвать cancel — она будет висеть в памяти
- Даже после завершения работы горутины ресурсы не освободятся
- defer cancel() — ЗОЛОТОЕ ПРАВИЛО, запомни наизусть

ЧТО ВОЗВРАЩАЕТ ctx.Err():
- context.DeadlineExceeded — истек таймаут
- context.Canceled — вызвали cancel() вручную
- nil — контекст еще активен

ФИШКИ И ПОДВОДНЫЕ КАМНИ:
1. Background() — корневой контекст (пустой, никогда не отменяется)
2. TODO() — когда не знаешь, какой контекст передать (временная заглушка)
3. WithTimeout = WithDeadline + время от текущего момента
4. Таймаут начинает тикать сразу с момента создания
5. После cancel() канал ctx.Done() закрывается (можно читать бесконечно)

КОГДА ИСПОЛЬЗУЕТСЯ НА ПРАКТИКЕ (реальные примеры):
1. HTTP сервер: каждый запрос имеет свой контекст
2. HTTP клиент: req, _ := http.NewRequestWithContext(ctx, ...)
3. Запросы в БД: rows, _ := db.QueryContext(ctx, ...)
4. Свои горутины: select { case <-ctx.Done(): return }
*/

func contextWithTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	ch := make(chan string)

	go func() {
		time.Sleep(200 * time.Millisecond)
		ch <- "Done"
	}()

	select {
	case res := <-ch:
		fmt.Println("Результат:", res)
	case <-ctx.Done():
		fmt.Println("Таймаут! Причина:", ctx.Err())
	}
}

/*
HEARTBEAT — КОГДА МОЖЕТ ПРИГОДИТЬСЯ:

Реальный кейс:
- Мониторинг долго работающих горутин
- Проверка, не завис ли воркер
- Graceful shutdown с ожиданием завершения

ДЛЯ СОБЕСА:
Не спрашивают, но если спросят "как проверить что горутина жива" — покажешь этот код.
*/

func heartbeatForMonitoring() {
	heartbeat := make(chan struct{})

	// Рабочая горутина с heartbeat
	go func() {
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				heartbeat <- struct{}{} // Живчик
			default:
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	for {
		select {
		case <-heartbeat:
			fmt.Println("alive")
		case <-ctx.Done():
			fmt.Println("stopped")
			return
		}
	}
}

/*
4. SEMAPHORE (СЕМАФОР) ЧЕРЕЗ BUFFERED CHANNEL

ПРОБЛЕМА:
  - Нужно ограничить количество ОДНОВРЕМЕННО выполняющихся операций.
  - Пример: не более 5 параллельных запросов к внешнему API.

РЕШЕНИЕ:
  - Буферизированный канал с ёмкостью = максимальное число операций.
  - Перед операцией: канал <- struct{}{} (занимаем слот).
  - После операции: <-канал (освобождаем слот).

ОТЛИЧИЕ ОТ WORKER POOL:
  - Worker Pool: фиксированное число ГОРУТИН, задачи в очереди.
  - Semaphore: фиксированное число ОПЕРАЦИЙ, горутин может быть много.
*/

func semaphoreExample() {
	const (
		maxConcurrent = 3  // максимум 3 параллельные операции
		totalTasks    = 10 // всего 10 задач
	)

	sem := make(chan struct{}, maxConcurrent) // семафор
	var wg sync.WaitGroup

	for i := 0; i < totalTasks; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()

			sem <- struct{}{}        // захватываем слот (блокируется, если все заняты)
			defer func() { <-sem }() // освобождаем слот

			// имитация работы
			fmt.Printf("Semaphore: Task %d is running\n", taskID)
			time.Sleep(500 * time.Millisecond)
		}(i)
	}

	wg.Wait()
	fmt.Println("Semaphore: All tasks completed")
}

/*
5. ERRGROUP.WITHCONTEXT

ПРОБЛЕМА:
  - Запустили несколько горутин, нужно:
    1. Дождаться завершения всех.
    2. Если одна горутина упала с ошибкой — отменить остальные.
    3. Получить первую ошибку.

РЕШЕНИЕ:
  - errgroup.Group — надстройка над sync.WaitGroup.
  - WithContext создаёт группу с общим контекстом.
  - При ошибке в любой горутине контекст отменяется.

КОГДА ИСПОЛЬЗОВАТЬ:
  - Параллельные запросы к нескольким сервисам.
  - Задачи, где ошибка в одной = бессмысленность остальных.
*/

func errgroupEx() {
	urls := []string{
		"https://httpbin.org/get",
		"https://httpbin.org/status/500",
		"https://httpbin.org/delay/2",
	}
	var result sync.Map
	g, ctx := errgroup.WithContext(context.Background())

	for _, url := range urls {
		url := url // важно: захватываем переменную цикла

		g.Go(func() error {
			// Создаём запрос с контекстом из группы
			req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
			if err != nil {
				return err
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			result.Store(url, resp.StatusCode)

			if resp.StatusCode >= 400 {
				return fmt.Errorf("HTTP error for %s: %d", url, resp.StatusCode)
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Printf("Errgroup: Ошибка в одном из запросов: %v\n", err)
	}
	fmt.Println("Errgroup: Все запросы выполнены успешно")

	result.Range(func(key, value any) bool {
		fmt.Printf("Errgroup: %s → %d\n", key, value)
		return true
	})
}

/*
6. RATE LIMITER (ОГРАНИЧИТЕЛЬ СКОРОСТИ)

ПРОБЛЕМА:
  - Нужно ограничить количество операций в единицу времени.
  - Например: не более 10 запросов в секунду к API.

РЕШЕНИЯ:
  1. time.Ticker: для равномерного распределения операций.
  2. Token Bucket (x/time/rate): позволяет накапливать burst (всплеск).

КОГДА ИСПОЛЬЗОВАТЬ:
  - Интеграция с API, у которых есть rate limit.
  - Защита собственного API от перегрузки.
*/

func rateLimiterTicker() {
	// 5 операций в секунду -> каждые 200 мс
	limiter := time.NewTicker(200 * time.Millisecond)
	defer limiter.Stop()

	for i := 0; i < 10; i++ {
		<-limiter.C // ждём разрешения
		fmt.Printf("Request %d at %s\n", i, time.Now().Format("15:04:05.000"))
	}
}

func rateLimiterTokenBucket() {
	// 5 токенов в секунду, burst = 3 (можно сделать 3 быстрых запроса подряд)
	limiter := rate.NewLimiter(rate.Limit(5), 3)
	start := time.Now()

	for i := 0; i < 10; i++ {
		limiter.Wait(context.Background())
		fmt.Printf("Request %d at %s\n", i, time.Since(start).Truncate(10*time.Millisecond))
	}
}

/*
=============================================================================
7. COMPARE: ВСЕ ПАТТЕРНЫ (ШПАРГАЛКА ДЛЯ СОБЕСА)
=============================================================================

 ПАТТЕРН          КОГДА ИСПОЛЬЗОВАТЬ
 Worker Pool      Много независимых задач. Нужно ограничить число горутин.
                  Задачи приходят в виде очереди.

 Pipeline         Данные нужно последовательно обработать.
                  Каждый этап можно распараллелить (fan-out).

 Semaphore        Нужно ограничить число ОДНОВРЕМЕННЫХ операций.
                  Горутин может быть много, но активных — не больше N.

 errgroup         Нужно дождаться группы горутин И обработать первую ошибку.
                  При ошибке — отменить остальные.

 Rate Limiter     Нужно ограничить число операций в единицу времени.
                  Защита API, интеграция с внешними сервисами.
*/

/*
8. РЕАЛЬНЫЙ КЕЙС: HTTP CLIENT С RETRY, TIMEOUT, RATE LIMIT

Это комбинация паттернов, которую часто спрашивают на собеседованиях:
  - Rate Limiter: не более N запросов в секунду.
  - Timeout: не более M секунд на один запрос.
  - Retry: при ошибке повторяем до K раз с backoff.
  - Context: возможность отменить всю операцию.
*/

type httpClient struct {
	limiter  *rate.Limiter
	timeout  time.Duration
	maxRetry int
}

func newHTTPClient(requestsPerSec int, timeout time.Duration, maxRetry int) *httpClient {
	return &httpClient{
		limiter:  rate.NewLimiter(rate.Limit(requestsPerSec), 1),
		timeout:  timeout,
		maxRetry: maxRetry,
	}
}

func (c *httpClient) doWithRetry(ctx context.Context, url string) (*http.Response, error) {
	var lastErr error

	for attemp := 0; attemp < c.maxRetry; attemp++ {
		// 1. Rate limit: ждём токен
		if err := c.limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limiter: %w", err)
		}
		// 2. Таймаут на конкретный запрос
		reqCtx, cancel := context.WithTimeout(ctx, c.timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		// 3. Делаем запрос
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		// 4. Backoff перед ретраем (кроме последней попытки)
		if attemp < c.maxRetry-1 {
			backoff := time.Duration(attemp+1) * 200 * time.Millisecond
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}
	return nil, fmt.Errorf("failed after %d attempts: %w", c.maxRetry, lastErr)
}

func HTTPClientEx() {
	client := newHTTPClient(3, 2*time.Second, 3)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp, err := client.doWithRetry(ctx, "https://httpbin.org/delay/1")
	if err != nil {
		fmt.Printf("HTTP Client Error: %v\n", err)
		return
	}
	defer resp.Body.Close()
	fmt.Printf("HTTP Client Success: %s\n", resp.Status)
}
