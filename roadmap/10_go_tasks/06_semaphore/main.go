package main

/*
Классический семафор (ограничение количества горутин)
У нас есть 10 задач, но мы хотим выполнять не более 3
одновременно (например, чтобы не перегрузить БД).
*/
/*
func main() {
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(taskID int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			fmt.Printf("Задача %d: начала работу\n", taskID)
			time.Sleep(1 * time.Second)
			fmt.Printf("Задача %d: закончила работу\n", taskID)
		}(i)
	}
	wg.Wait()
	fmt.Println("Все задачи выполнены")
}
*/
/*
Семафор с ограничением по контексту (Graceful Shutdown)
То же самое, но с возможностью отмены. Если сервер получает
SIGTERM, мы не хотим, чтобы горутины навсегда зависли в ожидании слота.
*/
/*
func main() {
	sem := make(chan struct{}, 2)
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				fmt.Printf("Задача %d: выполняется\n", id)
				time.Sleep(2 * time.Second)

			case <-ctx.Done():
				fmt.Printf("Задача %d: отменена (таймаут ожидания слота)\n", id)
				return
			}
		}(i)
	}
	wg.Wait()
	cancel()
	fmt.Println("Все горутины завершены")
}
*/

/*
3: Семафор с обработкой ошибок (errgroup + semaphore)
Часто семафор используют в связке с errgroup, чтобы выполнять HTTP-запросы
к внешнему API (ограничивая параллелизм) и останавливаться при первой ошибке.
*/
/*
func main() {
	// Семафор на 3 одновременных запроса
	sem := make(chan struct{}, runtime.NumCPU())
	g, ctx := errgroup.WithContext(context.Background())

	for i := 1; i <= 105; i++ {
		id := i
		g.Go(func() error {
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				fmt.Printf("Запрос %d: отменен на этапе ожидания слота\n", id)
				return ctx.Err()
			}

			if ctx.Err() != nil {
				return ctx.Err()
			}
			time.Sleep(500 * time.Millisecond)
			fmt.Printf("Запрос %d: успешно\n", id)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		fmt.Printf("Ошибка в пуле: %v\n", err)
	} else {
		fmt.Println("Все запросы выполнены")
	}
}
*/
/*
Weighted Semaphore (Взвешенный семафор)
В реальной жизни задачи бывают разного «веса». Например, один запрос — это 1 слот,
а другой (тяжелый) — 3 слота. Здесь обычный chan struct{} не подойдет.
В Go есть пакет golang.org/x/sync/semaphore, который реализует Weighted Semaphore.
*/
/*
func main() {
	sem := semaphore.NewWeighted(5)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()

			if err := sem.Acquire(ctx, 2); err != nil {
				fmt.Printf("Задача %d: ошибка захвата: %v\n", id, err)
				return
			}
			defer sem.Release(2)
			fmt.Printf("Задача %d (вес 2): выполняется\n", id)
			time.Sleep(1 * time.Second)
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx := context.Background()

			if err := sem.Acquire(ctx, 1); err != nil {
				fmt.Printf("Задача %d: ошибка захвата: %v\n", id, err)
				return
			}
			defer sem.Release(1)
			fmt.Printf("Легкая задача %d (вес 1): выполняется\n", id)
			time.Sleep(500 * time.Millisecond)
		}(i)
	}
	wg.Wait()
	fmt.Println("Все задачи завершены")
}
*/
//"Ограниченный пайплайн обработки логов"
/*
func processLogs(ctx context.Context, logs <-chan string) error {
	sem := make(chan struct{}, 3)
	g, ctx := errgroup.WithContext(ctx)

	for {
		select {
		case <-ctx.Done():
			return g.Wait()
		case log, ok := <-logs:
			if !ok {
				return g.Wait()
			}

			g.Go(func() error {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case sem <- struct{}{}:
					defer func() { <-sem }()
				}

				if ctx.Err() != nil {
					return ctx.Err()
				}
				fmt.Printf("Обрабатываю лог: %s\n", log)
				time.Sleep(500 * time.Millisecond)

				return nil
			})
		}
	}
}
*/
