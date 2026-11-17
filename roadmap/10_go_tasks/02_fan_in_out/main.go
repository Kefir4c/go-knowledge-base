package main

import (
	"context"
	"errors"
	"time"

	"golang.org/x/sync/errgroup"
)

/*
type results struct {
	jobID int
	val   int
	err   error
}

func generate(ctx context.Context, jobs chan<- int, count int) error {
	defer close(jobs)
	for i := 1; i <= count; i++ {
		select {
		case jobs <- i:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func worker(ctx context.Context, id int, jobs <-chan int, out chan<- results) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case job, ok := <-jobs:
			if !ok {
				return nil
			}

			taskCtx, taskCancel := context.WithTimeout(ctx, 600*time.Millisecond)
			val, err := doWork(taskCtx, job)
			taskCancel()

			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- results{job, val, err}:
			}
		}
	}
}

func doWork(ctx context.Context, job int) (int, error) {
	if job%7 == 0 {
		time.Sleep(900 * time.Millisecond)
	}
	select {
	case <-ctx.Done():
		return 0, ctx.Err()
	case <-time.After(50 * time.Millisecond):
		return job * job, nil
	}
}

func merge(ctx context.Context, chans ...<-chan results) <-chan results {
	out := make(chan results)
	var wg sync.WaitGroup

	for _, ch := range chans {
		wg.Add(1)
		go func(c <-chan results) {
			defer wg.Done()
			for r := range c {
				select {
				case <-ctx.Done():
					return
				case out <- r:
				}
			}
		}(ch)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	g, ctx := errgroup.WithContext(ctx)

	const numWorks = 4
	jobs := make(chan int, 10)

	workOuts := make([]<-chan results, numWorks)
	for i := 1; i < numWorks; i++ {
		id := i
		out := make(chan results, 10)
		workOuts = append(workOuts, out)

		g.Go(func() error {
			defer close(out)
			return worker(ctx, id, jobs, out)
		})
	}
	g.Go(func() error {
		return generate(ctx, jobs, 20)
	})

	results := merge(ctx, workOuts...)

	for r := range results {
		if r.err != nil {
			fmt.Printf("задача %d: ошибка: %v\n", r.jobID, r.err)
		} else {
			fmt.Printf("задача %d: результат %d\n", r.jobID, r.val)
		}
	}

	err := g.Wait()
	switch {
	case err == nil:
		fmt.Println("все задачи обработаны")
	case errors.Is(err, context.Canceled):
		fmt.Println("остановлено сигналом ОС")
	default:
		fmt.Printf("остановлено с ошибкой: %v\n", err)
	}
}
*/
/*
type checkResults struct {
	url      string
	status   int
	latency  time.Duration
	err      error
	workerID int
}

func urlProducer(ctx context.Context, url chan<- string, list []string) error {
	defer close(url)
	for _, u := range list {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case url <- u:
		}
	}
	return nil
}

func checker(ctx context.Context, id int, urls <-chan string, results chan<- checkResults) error {
	client := &http.Client{
		Timeout: 10 * time.Second, // защита от зависших соединений
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case url, ok := <-urls:
			if !ok {
				return nil // канал закрыт — работы больше нет
			}

			start := time.Now()
			// производный контекст с таймаутом на один запрос
			reqCtx, cancel := context.WithTimeout(ctx, 5500*time.Millisecond)

			req, err := http.NewRequestWithContext(reqCtx, "GET", url, nil)
			if err != nil {
				cancel()
				select {
				case results <- checkResults{url: url, err: err, workerID: id}:
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}

			resp, err := client.Do(req)
			latency := time.Since(start)
			cancel() // сразу освобождаем таймер

			if err != nil {
				select {
				case results <- checkResults{url: url, err: err, latency: latency, workerID: id}:
				case <-ctx.Done():
					return ctx.Err()
				}
				continue
			}
			resp.Body.Close() // важно: иначе утечка соединений

			select {
			case results <- checkResults{
				url:      url,
				status:   resp.StatusCode,
				latency:  latency,
				workerID: id,
			}:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func dynamicMerge(ctx context.Context, chans ...<-chan checkResults) <-chan checkResults {
	out := make(chan checkResults)
	var wg sync.WaitGroup

	for _, ch := range chans {
		wg.Add(1)
		go func(c <-chan checkResults) {
			defer wg.Done()
			for r := range c {
				select {
				case <-ctx.Done():
					return
				case out <- r:
				}
			}
		}(ch)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	urls := []string{
		"https://google.com",
		"https://github.com",
		"https://stackoverflow.com",
		"https://golang.org",
		"https://invalid-domain-12345.com", // ошибка DNS
		"https://httpbin.org/delay/5",      // превысит таймаут 1.5с
	}

	const numWorkers = 3
	urlChan := make(chan string, len(urls))

	// FAN-OUT: каждый воркер со своим каналом результатов
	workerChans := make([]<-chan checkResults, 0, numWorkers)
	var wg sync.WaitGroup

	for i := 1; i <= numWorkers; i++ {
		id := i
		ch := make(chan checkResults, 10)
		workerChans = append(workerChans, ch)

		wg.Add(1)
		go func() {
			defer wg.Done()
			defer close(ch) // закрываем ПОСЛЕ завершения checker
			if err := checker(ctx, id, urlChan, ch); err != nil {
				fmt.Printf("воркер %d остановлен: %v\n", id, err)
			}
		}()
	}

	// Producer
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := urlProducer(ctx, urlChan, urls); err != nil {
			fmt.Printf("producer остановлен: %v\n", err)
		}
	}()

	// FAN-IN
	results := dynamicMerge(ctx, workerChans...)

	// Consumer
	for r := range results {
		if r.err != nil {
			fmt.Printf("[воркер %d] %s: ОШИБКА: %v (%v)\n", r.workerID, r.url, r.err, r.latency)
		} else {
			fmt.Printf("[воркер %d] %s: статус %d (%v)\n", r.workerID, r.url, r.status, r.latency)
		}
	}

	wg.Wait()
	fmt.Println("Все проверки завершены")
}
*/

/*
Задача: Агрегатор профиля пользователя
Формулировка (как на собесе):
Есть три внешних сервиса: ProfileService, OrdersService, RecommendationsService.
Нужно по userID собрать данные из всех трёх параллельно.
Если сервис отвечает дольше 800мс — считаем, что он недоступен, возвращаем nil + ошибку в поле результата.
Общий дедлайн на всю операцию — 2 секунды.
Если один сервис упал — остальные всё равно должны отработать.
Результат: структура с тремя полями + слайс ошибок.
*/
type Profile struct {
	Name string
	Age  int
}
type Order struct {
	ID    string
	Total float64
}
type Recommendation struct {
	Item  string
	Score float64
}

type UserData struct {
	Profile         *Profile
	Orders          []Order
	Recommendations []Recommendation
	Errors          []error
}

// Эти функции "блокирующие", ходят в сеть. Их нельзя менять.
func fetchProfile(ctx context.Context, userID string) (*Profile, error) {
	select {
	case <-time.After(500 * time.Millisecond):
		return &Profile{Name: "Ivan", Age: 30}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func fetchOrders(ctx context.Context, userID string) ([]Order, error) {
	select {
	case <-time.After(1200 * time.Millisecond):

		return []Order{{ID: "123", Total: 99.99}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func fetchRecommendations(ctx context.Context, userID string) ([]Recommendation, error) {
	select {
	case <-time.After(200 * time.Millisecond):
		return []Recommendation{{Item: "Book", Score: 4.5}}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func GetUserData(ctx context.Context, userID string) (*UserData, error) {
	g, ctx := errgroup.WithContext(ctx)
	results := make(chan struct {
		profile         *Profile
		orders          []Order
		recommendations []Recommendation
		err             error
	}, 3)

	g.Go(func() error {
		invalidCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		defer cancel()
		p, err := fetchProfile(invalidCtx, userID)
		results <- struct {
			profile         *Profile
			orders          []Order
			recommendations []Recommendation
			err             error
		}{profile: p, err: err}
		return nil
	})

	g.Go(func() error {
		invalidCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		defer cancel()
		o, err := fetchOrders(invalidCtx, userID)
		results <- struct {
			profile         *Profile
			orders          []Order
			recommendations []Recommendation
			err             error
		}{orders: o, err: err}
		return nil
	})

	g.Go(func() error {
		invalidCtx, cancel := context.WithTimeout(ctx, 800*time.Millisecond)
		defer cancel()
		r, err := fetchRecommendations(invalidCtx, userID)
		results <- struct {
			profile         *Profile
			orders          []Order
			recommendations []Recommendation
			err             error
		}{recommendations: r, err: err}
		return nil
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return nil, err
	}
	close(results)

	data := &UserData{}
	for i := 0; i < 3; i++ {
		res := <-results
		if res.err != nil {
			data.Errors = append(data.Errors, res.err)
			continue
		}
		if res.profile != nil {
			data.Profile = res.profile
		}
		if len(res.orders) > 0 {
			data.Orders = res.orders
		}
		if len(res.recommendations) > 0 {
			data.Recommendations = res.recommendations
		}
	}
	return data, nil
}
