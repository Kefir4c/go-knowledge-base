package channel

import (
	"fmt"
	"sync"
)

// Я решил вынести паттерны в отдельный файл :)
/*
Паттерн (шаблон, образец) — это типовое проверенное решение часто встречающейся проблемы в определенном контексте.

Ниже будет список паттернов, которые мы напишем:
- Worker Pool
- Pipeline
- Таймауты и отмена
*/

// ПОПУЛЯРНЫЕ ПАТТЕРНЫ РАБОТЫ С КАНАЛАМИ

/*
1. Worker Pool (Пул воркеров) — это паттерн параллельного программирования, где создается фиксированное количество горутин (воркеров),
которые обрабатывают задачи из общего канала.
ПРОБЛЕМА БЕЗ WORKER POOL:
- Для каждой задачи создается новая горутина
- При 10000 задачах → 10000 горутин
- Каждая горутина потребляет память (~2KB стека)
- Растет нагрузка на планировщик Go

ПРОБЛЕМА С WORKER POOL:
- Всего N горутин (например, 10)
- Задачи распределяются между ними
- Предсказуемое потребление ресурсов
- Контроль над параллелизмом
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
   — Данные проходят через цепочку последовательных обработчиков.
   — Каждый этап — отдельная горутина.
   — Следующий этап стартует до завершения предыдущего.

КЛЮЧЕВЫЕ ОСОБЕННОСТИ:
   - Fan-in: несколько источников → один канал
   - Fan-out: один источник → несколько обработчиков
   - Паттерн "генератор" (gen) — создает поток данных

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
