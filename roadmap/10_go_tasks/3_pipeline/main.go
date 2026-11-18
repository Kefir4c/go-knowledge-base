package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

/*
Задача 1: Простой Pipeline (лайтовая)
Генерируешь числа от 1 до 100. Стадия 1: умножает на 2.
Стадия 2: фильтрует только чётные. Стадия 3: суммирует. Выводишь сумму.
*/
/*
func generateNumbers(ctx context.Context, count int) <-chan int {
	out := make(chan int, count)
	go func() {
		defer close(out)
		for i := 0; i < count; i++ {
			select {
			case <-ctx.Done():
				return
			case out <- i:
			}
		}
	}()
	return out
}

func multiplyByTwo(ctx context.Context, in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for num := range in {
			select {
			case <-ctx.Done():
				return
			case out <- num * num:
			}
		}
	}()
	return out
}

func filterDivisibleByFour(ctx context.Context, in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for num := range in {
			if num%4 != 0 {
				continue // Пропускаем неподходящие
			}
			select {
			case <-ctx.Done():
				return
			case out <- num:
			}
		}
	}()
	return out
}

func sumNumbers(ctx context.Context, in <-chan int) int {
	sum := 0
	for num := range in {
		sum += num
	}
	return sum
}

func main() {
	ctx := context.Background()

	numbers := generateNumbers(ctx, 1000000)
	doubled := multiplyByTwo(ctx, numbers)
	filtered := filterDivisibleByFour(ctx, doubled)
	result := sumNumbers(ctx, filtered)

	fmt.Printf("Сумма: %d\n", result)
}
*/

/*
Задача 2: Pipeline с Fan-out (параллельные воркеры на стадии)
Обрабатываешь 1000 "задач" (числа). Стадия обработки тяжёлая (спит 10мс). Нужно обработать
параллельно в 5 воркеров, чтобы уложиться в разумное время. Результаты собирать в один канал.
*/
/*
type task struct {
	id int
}

type results struct {
	taskID   int
	workerID int
	duration time.Duration
}

func generateTask(ctx context.Context, count int) <-chan task {
	out := make(chan task, count)
	go func() {
		defer close(out)
		for i := 0; i < count; i++ {
			select {
			case <-ctx.Done():
				return
			case out <- task{id: i}:
			}
		}
	}()
	return out
}

func processTask(ctx context.Context, in <-chan task, workers int) <-chan results {
	workerChans := make([]<-chan results, 0, workers)

	for i := 0; i < workers; i++ {
		workerID := i
		ch := make(chan results)
		workerChans = append(workerChans, ch)

		go func() {
			defer close(ch)

			for task := range in {
				start := time.Now()
				time.Sleep(3 * time.Millisecond)

				select {
				case <-ctx.Done():
					return
				case ch <- results{taskID: task.id, workerID: workerID, duration: time.Since(start)}:
				}
			}
		}()
	}
	return merge(ctx, workerChans...)
}

func merge(ctx context.Context, chans ...<-chan results) <-chan results {
	out := make(chan results)
	var wg sync.WaitGroup

	for _, ch := range chans {
		ch := ch //опционально.Для старых версий
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

func collectResults(ctx context.Context, in <-chan results) (int, time.Duration) {
	count := 0
	var totalDuration time.Duration

	for r := range in {
		count++
		totalDuration += r.duration
	}
	return count, totalDuration
}

func main() {
	ctx := context.Background()

	tasks := generateTask(ctx, 10000)
	results := processTask(ctx, tasks, 5) // 5 воркеров
	count, total := collectResults(ctx, results)

	fmt.Printf("Обработано: %d задач\n", count)
	fmt.Printf("Суммарное время работы воркеров: %v\n", total)
	fmt.Printf("Реальное время (примерно): %v (если бы последовательно - %v)\n", total/5, total)
}
*/

/*
Задача 3: Pipeline с Graceful Shutdown и ошибками (мидл+)
Условие: Обрабатываешь файлы. Стадия 1: читает имена файлов из директории.
Стадия 2: "читает" файл (имитация - sleep). Стадия 3: сохраняет результат.
Если на любой стадии ошибка — останавливаем всё, но с возможностью обработать
уже накопленное (partial results). По Ctrl+C — graceful shutdown с сохранением текущего прогресса.
*/
/*
type fileJob struct {
	name string
}
type fileResult struct {
	name    string
	content string
	err     error
}

func listFile(ctx context.Context, dir string) (<-chan fileJob, error) {
	files := []string{"file1.txt", "file2.txt", "file3.txt", "error.txt", "file5.txt"}
	out := make(chan fileJob)

	go func() {
		defer close(out)
		for _, file := range files {
			select {
			case <-ctx.Done():
				return
			case out <- fileJob{name: file}:
			}
		}
	}()
	return out, nil
}

func processFile(ctx context.Context, in <-chan fileJob, workers int) <-chan fileResult {
	workersChans := make([]<-chan fileResult, 0, workers)

	for i := 0; i < workers; i++ {
		ch := make(chan fileResult)
		workersChans = append(workersChans, ch)

		go func() {
			defer close(ch)
			for job := range in {
				if job.name == "error.txt" {
					select {
					case <-ctx.Done():
						return
					case ch <- fileResult{name: job.name, err: errors.New("файл повреждён")}:
					}
					continue
				}

				time.Sleep(50 * time.Millisecond)

				select {
				case <-ctx.Done():
					return
				case ch <- fileResult{name: job.name, content: fmt.Sprintf("content of %s", job.name), err: nil}:
				}
			}
		}()
	}
	return mergeResults(ctx, workersChans...)
}

func mergeResults(ctx context.Context, chans ...<-chan fileResult) <-chan fileResult {
	out := make(chan fileResult)
	var wg sync.WaitGroup

	for _, ch := range chans {
		wg.Add(1)
		go func(c <-chan fileResult) {
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

func saveResults(ctx context.Context, in <-chan fileResult, bathSize int) ([]fileResult, []error) {
	var saved []fileResult
	var errs []error
	batch := make([]fileResult, 0, bathSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		fmt.Printf("Сохраняю батч из %d файлов\n", len(batch))
		saved = append(saved, batch...)
		batch = batch[:0]
	}
	for r := range in {
		if r.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", r.name, r.err))
			continue
		}
		batch = append(batch, r)
		if len(batch) == bathSize {
			flush()
		}
	}
	flush()
	return saved, errs
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("Старт обработки. Нажми Ctrl+C для остановки.")

	files, err := listFile(ctx, "./data")
	if err != nil {
		fmt.Printf("Ошибка списка файлов: %v\n", err)
		return
	}

	processed := processFile(ctx, files, 3)
	saved, errs := saveResults(ctx, processed, 2)

	fmt.Printf("")
	fmt.Printf("Успешно обработано: %d\n", len(saved))
	for _, s := range saved {
		fmt.Printf("  - %s\n", s.name)
	}

	if len(errs) > 0 {
		fmt.Printf("Ошибки (%d):\n", len(errs))
		for _, e := range errs {
			fmt.Printf("  ! %v\n", e)
		}
	}
}
*/

/*
Задача: Log Processor Pipeline
У нас есть поток логов в JSON-формате. Нужно построить пайплайн обработки:
Ingest — читает строки из канала (имитация tail -f)
Parse — парсит JSON в структуру (3 воркера, так как CPU-heavy)
Filter — пропускает только ERROR и WARN (2 воркера)
Batch — собирает по 20 записей и "сохраняет" в БД (sleep 100ms)
*/

type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"` // "INFO", "ERROR", "WARN", "DEBUG"
	Message   string    `json:"message"`
	Service   string    `json:"service"`
}

func logGenerator(ctx context.Context) <-chan string {
	out := make(chan string)

	go func() {
		defer close(out)
		level := []string{"INFO", "ERROR", "WARN", "DEBUG"}
		services := []string{"auth", "billing", "api"}

		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()

		count := 0
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				count++
				if count%10 == 0 {
					out <- `{"invalid json}`
					continue
				}
				entry := LogEntry{
					Timestamp: time.Now(),
					Level:     level[rand.Intn(len(level))],
					Message:   fmt.Sprint("Hello World ", count),
					Service:   services[rand.Intn(len(services))],
				}
				data, _ := json.Marshal(entry)
				out <- string(data)
			}
		}
	}()
	return out
}

func parseLogs(ctx context.Context, in <-chan string, workers int) <-chan LogEntry {
	out := make(chan LogEntry, 100)
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for line := range in {
				var entry LogEntry
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					continue
				}
				select {
				case <-ctx.Done():
					return
				case out <- entry:
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out

}

func filterLogs(ctx context.Context, in <-chan LogEntry, workers int) <-chan LogEntry {
	out := make(chan LogEntry, 100)
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for entry := range in {
				if entry.Level == "ERROR" || entry.Level == "WARN" {
					select {
					case <-ctx.Done():
						return
					case out <- entry:
					}
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func batchSave(ctx context.Context, in <-chan LogEntry, batchSize int) {
	batch := make([]LogEntry, 0, batchSize)

	save := func(data []LogEntry) {
		time.Sleep(100 * time.Millisecond)
	}

	for {
		select {
		case <-ctx.Done():
			if len(batch) > 0 {
				save(batch)
			}
			return
		case entry, ok := <-in:
			if !ok {
				if len(batch) > 0 {
					save(batch)
				}
				return
			}
			batch = append(batch, entry)
			if len(batch) >= batchSize {
				toSave := make([]LogEntry, len(batch))
				copy(toSave, batch)
				save(toSave)
				batch = batch[:0]
			}
		}
	}
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	rand.Seed(time.Now().UnixNano())

	logs := logGenerator(ctx)
	parsed := parseLogs(ctx, logs, 3)
	filtered := filterLogs(ctx, parsed, 2)
	go batchSave(ctx, filtered, 20)
	<-ctx.Done()
	time.Sleep(200 * time.Millisecond)
}
