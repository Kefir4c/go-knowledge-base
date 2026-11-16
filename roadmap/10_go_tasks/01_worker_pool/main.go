package main

import (
	"context"
	"errors"
	"fmt"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

/*
// Обычный воркер пул
	func worker(ctx context.Context, id int, jobs <-chan int, results chan<- int, wg *sync.WaitGroup) {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				fmt.Printf("Воркер %d: отмена\n", id)
				return
			case job, ok := <-jobs:
				if !ok {
					fmt.Printf("Воркер %d: канал закрыт, выхожу\n", id)
					return
				}
				results <- job * job
			}
		}
	}

	func main() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		jobs := make(chan int, 500)
		results := make(chan int, 500)
		var wg sync.WaitGroup

		for i := 1; i <= 12; i++ {
			wg.Add(1)
			go worker(ctx, i, jobs, results, &wg)
		}

		go func() {
			for i := 1; i <= 500; i++ {
				jobs <- i
			}
			close(jobs)
		}()

		go func() {
			wg.Wait()
			close(results)
		}()

		for res := range results {
			fmt.Printf("Результат: %d\n", res)
		}
		fmt.Println("Главная функция завершена")
	}
*/

/*
//Воркер с кастомным результатом
type result struct {
	val int
	err error
}

func worker(ctx context.Context, id int, jobs <-chan int, results chan<- result, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Воркер %d: отмена\n", id)
			return
		case job, ok := <-jobs:
			if !ok {
				fmt.Printf("Воркер %d: канал закрыт, выхожу\n", id)
				return
			}
			if job%2 == 0 {
				results <- result{
					val: job,
					err: nil,
				}
			} else {
				results <- result{
					val: job,
					err: fmt.Errorf("нечетное число: %d", job),
				}
			}
		}
	}
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	jobs := make(chan int, 500)
	results := make(chan result, 500)
	var wg sync.WaitGroup

	for i := 1; i <= 12; i++ {
		wg.Add(1)
		go worker(ctx, i, jobs, results, &wg)
	}

	go func() {
		for i := 1; i <= 500; i++ {
			jobs <- i
		}
		close(jobs)
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	for res := range results {
		if res.err != nil {
			fmt.Printf("Ошибка: %v\n", res.err)
		} else {
			fmt.Printf("Результат: %d\n", res.val)
		}
	}
}
*/

/*
// Усложнение 1: "Fail Fast" (Остановка при первой ошибке)
// в errgroup уже вшит wg
func worker(ctx context.Context, id int, jobs <-chan int) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case job, ok := <-jobs:
			if !ok {
				return nil
			}
			if job%2 != 0 {
				return fmt.Errorf("воркер %d: нечетное число %d", id, job)
			}
			fmt.Printf("Воркер %d: обработал %d\n", id, job)
		}
	}
}

func generateJobs(ctx context.Context, jobs chan<- int, count int) error {
	defer close(jobs)
	for i := 1; i <= count; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case jobs <- i:
		}
	}
	return nil
}

func main() {
	g, ctx := errgroup.WithContext(context.Background())
	jobs := make(chan int, 100)

	for i := 1; i <= 3; i++ {
		id := i
		g.Go(func() error {
			return worker(ctx, id, jobs)
		})
	}

	g.Go(func() error {
		return generateJobs(ctx, jobs, 10)
	})

	if err := g.Wait(); err != nil {
		fmt.Printf("ОШИБКА: %v\n", err)
	} else {
		fmt.Println("Все задачи выполнены")
	}
}
*/
/*
// Усложнение 2: Динамический пул (Context Timeout)
func worker(ctx context.Context, id int, jobs <-chan int) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case job, ok := <-jobs:
			if !ok {
				return nil
			}
			taskCtx, taskCancel := context.WithTimeout(ctx, 1*time.Second)
			err := doHardWork(taskCtx, job)

			taskCancel()
			if err != nil {
				return fmt.Errorf("воркер %d: ошибка на задаче %d: %w", id, job, err)
			}
			fmt.Printf("Воркер %d: успешно обработал %d\n", id, job)
		}
	}
}

func doHardWork(ctx context.Context, job int) error {
	select {
	case <-time.After(999 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func main() {
	g, ctx := errgroup.WithContext(context.Background())
	jobs := make(chan int, 10)

	for i := 1; i <= 3; i++ {
		id := i
		g.Go(func() error {
			return worker(ctx, id, jobs)
		})
	}

	g.Go(func() error {
		defer close(jobs)
		for i := 1; i <= 5; i++ {
			jobs <- i
		}
		return nil
	})

	if err := g.Wait(); err != nil {
		// Ожидаемый вывод: ошибка context deadline exceeded
		fmt.Printf("Пул остановлен: %v\n", err)
	} else {
		fmt.Println("Все задачи успешно обработаны")
	}
}
*/

// полный production-ready код
type job struct {
	id  int
	val int
}

type metrics struct {
	Processed atomic.Int64
	Failed    atomic.Int64
	TotalTime atomic.Int64
}

func worker(ctx context.Context, jobs <-chan job, id int, metrics *metrics) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case job, ok := <-jobs:
			if !ok {
				return nil
			}
			start := time.Now()

			taskCtx, taskCancel := context.WithTimeout(ctx, 500*time.Millisecond)
			err := processTask(taskCtx, job)

			taskCancel()

			metrics.TotalTime.Add(time.Since(start).Milliseconds())

			if err != nil {
				metrics.Failed.Add(1)
				return fmt.Errorf("воркер %d: задача %d провалена: %w", id, job.id, err)
			}
			metrics.Processed.Add(1)
		}
	}
}

func processTask(ctx context.Context, job job) error {
	if job.id == 4 {
		time.Sleep(1 * time.Second)
	}
	select {
	case <-time.After(100 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	jobs := make(chan job, 10)
	metrics := &metrics{}

	for i := 1; i <= 3; i++ {
		id := i
		g.Go(func() error {
			return worker(ctx, jobs, id, metrics)
		})
	}

	g.Go(func() error {
		defer close(jobs)
		for i := 1; i <= 10; i++ {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case jobs <- job{
				id:  i,
				val: i * 10,
			}:
			}
		}
		return nil
	})

	fmt.Println("Пул запущен. Нажмите Ctrl+C для остановки.")
	if err := g.Wait(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			fmt.Printf("Пул остановлен по таймауту: %v\n", err)
		} else if errors.Is(err, context.Canceled) {
			fmt.Println("Пул остановлен по сигналу ОС (Graceful Shutdown).")
		} else {
			fmt.Printf("Пул остановлен с ошибкой: %v\n", err)
		}
	}

	fmt.Println("ИТОГИ ")
	fmt.Printf("Обработано задач: %d\n", metrics.Processed.Load())
	fmt.Printf("Провалено задач: %d\n", metrics.Failed.Load())
	fmt.Printf("Общее время: %d мс\n", metrics.TotalTime.Load())
}
