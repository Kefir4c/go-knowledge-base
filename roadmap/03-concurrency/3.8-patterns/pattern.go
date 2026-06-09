package main

import (
	"context"
	"fmt"
	"sync"
	"time"
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
