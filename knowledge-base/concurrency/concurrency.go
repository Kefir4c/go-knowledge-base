package concurrency

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// 1. ГОРУТИНЫ и БАЗОВЫЕ КАНАЛЫ
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. ЧТО ТАКОЕ ГОРУТИНА (GOROUTINE)?
   Это "легковесный поток", управляемый рантаймом Go, а не операционной системой.
   - Память: поток ОС занимает ~1-2 МБ, горутина стартует с 2 КБ.
   - Переключение контекста: у горутин оно происходит быстрее, так как не
     требует обращения к ядру ОС (LWP).
   - Масштабируемость: на одном сервере можно запустить миллионы горутин.

2. КЛЮЧЕВОЕ СЛОВО `go`:
   Любую функцию можно превратить в горутину, просто добавив `go` перед вызовом.
   ВНИМАНИЕ: main() — это тоже горутина. Если она завершится, все остальные
   горутины "умрут" мгновенно.

3. КАНАЛЫ (CHANNELS):
   Это "труба", по которой горутины передают данные.
   - Типизация: канал строго привязан к типу данных (chan int, chan string).
   - Синхронизация: передача данных через небуферизированный канал
     блокирует обе горутины до момента передачи.
*/

// ПРИМЕР 1: Простейший запуск горутины
func sayHello() {
	fmt.Println("Привет из горутины!")
}

func exampleDasicGo() {
	go sayHello() // Запуск в фоне
	// Если здесь не подождать, программа выйдет раньше, чем горутина успеет что-то напечатать
	time.Sleep(10 * time.Millisecond)
}

// ПРИМЕР 2: Анонимные горутины и замыкания (ОПАСНОСТЬ)
func exampleClosureIssue() {
	for i := 0; i < 5; i++ {
		// ОШИБКА: к моменту запуска горутины `i` может уже стать 5.
		// Всегда передавай переменные цикла через аргументы!
		go func(val int) {
			fmt.Printf("%d ", val)
		}(i)
	}
	time.Sleep(10 * time.Millisecond)
}

// ПРИМЕР 3: Создание и работа с небуферизированным каналом
func exampleChannels() {
	// Создаем канал для передачи строк
	ch := make(chan string)

	go func() {
		ch <- "Данные из горутины" // ОТПРАВКА (блокируется, пока кто-то не прочитает)
	}()

	msg := <-ch // ПОЛУЧЕНИЕ (блокируется, пока кто-то не отправит)
	fmt.Println(msg)
}

// ПРИМЕР 4: Передача данных в обе стороны
func pinger(pingChan, pongChan chan string) {
	msg := <-pingChan
	fmt.Println("Получено:", msg)
	pongChan <- "pong"
}

func ExamplePingPong() {
	ping := make(chan string)
	pong := make(chan string)

	go pinger(ping, pong)

	ping <- "ping"

	fmt.Println("Ответ:", <-pong)
}

// ПРИМЕР 5: Закрытие канала (Close)
func exampleCloseChannel() {
	ch := make(chan int)

	go func() {
		for i := 0; i < 3; i++ {
			ch <- i
		}
		close(ch)
	}()

	for v := range ch {
		fmt.Println("Получено:", v)
	}
}

// --- ПРИМЕР 6: Проверка, закрыт ли канал (Comma ok) ---
func exampleCheckClosed() {
	ch := make(chan string)
	close(ch)

	val, ok := <-ch
	if !ok {
		fmt.Println("Канал закрыт, данных нет!")
	} else {
		fmt.Println("Получено:", val)
	}
}

// --- ПРИМЕР 7: Канал как сигнал завершения (Void Channel) ---
func exampleDoneSignal() {
	done := make(chan bool)
	go func() {
		fmt.Println("Выполняю тяжелую работу...")
		time.Sleep(time.Second)
		done <- true // Сигнализируем о завершении
	}()
	<-done // Ждем, пока в канал что-то прилетит
	fmt.Println("Работа закончена!")
}

// 2. МОДЕЛЬ GMP, СИХРОНИЗАЦИЯ И СОСТОЯНИЕ ГОНКИ
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. МОДЕЛЬ GMP (ПОЧЕМУ ГОРУТИНЫ — НЕ ПОТОКИ):
   В традиционных языках (Java, C++) один поток программы = один поток ОС (1:1).
   В Go используется модель M:N (много горутин на малое число потоков ОС).
   - G (Goroutine): Твой код, стек (от 2КБ), состояние.
   - M (Machine): Поток ОС (Thread).
   - P (Processor): Ресурс для выполнения (контекст). Обычно их столько, сколько ядер у CPU.
   Это позволяет Go переключать горутины за ~10-20 наносекунд (потоки ОС — за микросекунды).

2. МЬЮТЕКСЫ (MUTEX) VS КАНАЛЫ:
   - Мутексы (sync.Mutex): Защищают ПАМЯТЬ. Используй их для простых счетчиков, мап и кэшей.
   - Каналы: Передают ВЛАДЕНИЕ данными. Используй для передачи задач и координации.
   Правило: "Сложная логика — каналы, простое состояние — мьютексы".

3. RACE CONDITION (СОСТОЯНИЕ ГОНКИ):
   Ситуация, когда две горутины одновременно читают и пишут в одну переменную без синхронизации.
   Результат становится непредсказуемым.
*/

// --- ПРИМЕР 1: Проблема конкурентного доступа (Race Condition) ---
// Этот код содержит баг. Если запустить с флагом -race, Go его найдет.
func exampleRaceCondition() {
	var counter int

	for i := 0; i < 10000; i++ {
		go func() {
			counter++ // НЕСКОЛЬКО ГОРУТИН ПИШУТ СЮДА ОДНОВРЕМЕННО
		}()
	}
	time.Sleep(10 * time.Microsecond)
	fmt.Println("Счетчик без защиты:", counter) // Скорее всего будет меньше 1000
}

// ПРИМЕР 2: Решение через sync.Mutex
func exampleMutex() {
	var (
		counter int
		mu      sync.Mutex
		wg      sync.WaitGroup
	)

	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			counter++
			mu.Unlock()
		}()
	}
	wg.Wait()
	fmt.Println("Mutex - счетчик:", counter)
	fmt.Println("(Всегда 1000, но медленнее из-за блокировок)\n")
}

// ПРИМЕР 3: Решение через канал
func exampleChannel() {
	var (
		counter int
		wg      sync.WaitGroup
	)
	ch := make(chan int, 10000)

	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch <- 1
		}()

		go func() {
			wg.Wait()
			close(ch)
		}()
	}

	for val := range ch {
		counter += val
	}

	fmt.Println("Канал - счетчик:", counter)
	fmt.Println("Хорошо для коммуникации между горутинами, но для счетчика оверхед")
}

// ПРИМЕР 4: Оптимизация через sync.RWMutex
// Если у нас много читателей и мало писателей, RWMutex в разы быстрее.
type Config struct {
	mu   sync.RWMutex
	data map[string]string
}

func (c *Config) get(key string) string {
	c.mu.RLock() // Читатели не блокируют друг друга
	defer c.mu.RUnlock()
	return c.data[key]
}

func (c *Config) set(key, val string) {
	c.mu.Lock() // Писатель блокирует всех (и читателей тоже)
	defer c.mu.Unlock()
	c.data[key] = val
}

// --- ПРИМЕР 5: Координация через sync.WaitGroup ---
// Никогда не используй time.Sleep для ожидания горутин!
func exampleWaitGroup() {
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			fmt.Printf("Worker %d закончил работу\n", id)
		}(i)
	}
	wg.Wait() // Ждем, пока счетчик не станет 0
	fmt.Println("Все воркеры свободны")
}

// --- ПРИМЕР 6: Однократное выполнение (sync.Once) ---
// Идеально для Singleton или ленивой инициализации БД.
func exampleSyncOnce() {
	var once sync.Once
	init := func() {
		fmt.Println("Тяжелая инициализация выполнена!")
	}

	for i := 0; i < 10; i++ {
		go func() {
			once.Do(init) // Выполнится только ОДИН раз за всю жизнь программы
		}()
	}
	time.Sleep(time.Millisecond * 50)
}

// --- ПРИМЕР 7: sync.Map для высоконагруженных кэшей ---
// Обычная мапа + мьютекс медленнее на многоядерных CPU из-за конкуренции за мьютекс.
func ExampleSyncMap() {
	var sm sync.Map

	sm.Store("key", "value")
	val, ok := sm.Load("key")
	if ok {
		fmt.Println("Из sync.Map получено:", val)
	}
}

// --- ПРИМЕР 8: Работа с "Race Detector" ---
/* Как обнаружить гонку? Go имеет встроенный инструмент.
Запускай свои тесты или приложение так:
$ go run -race main.go
$ go test -race ./...

Если в коде есть доступ к переменной из двух горутин без мьютекса/канала,
Go выведет подробный отчет: где читали, где писали и в каких горутинах.
*/

// 3. ШАБЛОНЫ МАШТАБИРОВАНИЯ И ПРОФИЛИРОВАНИЯ
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. WORKER POOL:
   Запуск миллионов горутин — это дешево, но не бесплатно. Если каждая горутина
   идет в БД, вы положите базу. Worker Pool ограничивает количество одновременно
   работающих горутин, создавая фиксированную "очередь" задач.

2. FAN-OUT / FAN-IN:
   - Fan-out: Одна горутина читает из канала и раздает задачи множеству воркеров.
   - Fan-in: Множество горутин пишут в свои каналы, а одна "собирающая" горутина
     объединяет их результаты в один поток.

3. ПРОФИЛИРОВАНИЕ (PPROF):
   В Go есть встроенный инструмент `pprof`. Он позволяет увидеть:
   - Стек всех запущенных горутин (найдем утечки).
   - Блокировки (кто кого ждет).
   - Использование CPU и памяти.
*/

// ПРИМЕР 1: Паттерн Worker Pool (Классика)
// Ограничиваем количество воркеров, чтобы не перегрузить систему.
func worker(id int, jobs <-chan int, results chan<- int) {
	for j := range jobs {
		fmt.Printf("Воркер %d начал задачу %d\n", id, j)
		time.Sleep(100 * time.Millisecond) // Имитация работы
		results <- j * 2
	}
}

func ExampleWorkerPool() {
	const numJobs = 10
	const numWorkers = 3

	jobs := make(chan int, numJobs)
	results := make(chan int, numJobs)

	// Запускаем пул воркеров
	for w := 1; w <= numWorkers; w++ {
		go worker(w, jobs, results)
	}

	// Отправляем задачи
	for j := 1; j <= numJobs; j++ {
		jobs <- j
	}
	close(jobs) // Важно: закрываем, чтобы воркеры вышли из цикла range

	// Собираем результаты
	for a := 1; a <= numJobs; a++ {
		<-results
	}
}

// ПРИМЕР 2: Fan-Out / Fan-In (Конвейер)
func producer(nums ...int) <-chan int {
	out := make(chan int)
	go func() {
		for _, n := range nums {
			out <- n
		}
		close(out)
	}()
	return out
}

func square(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		for n := range in {
			out <- n * n
		}
		close(out)
	}()
	return out
}

func merge(cs ...<-chan int) <-chan int {
	var wg sync.WaitGroup
	out := make(chan int)

	output := func(c <-chan int) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}

	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func exampleFanInFanOut() {
	in := producer(1, 2, 3, 4)

	// Fan-out: распределяем работу на два воркера
	c1 := square(in)
	c2 := square(in)

	// Fan-in: собираем результаты в один канал
	for n := range merge(c1, c2) {
		fmt.Println("Результат:", n)
	}
}

// ПРИМЕР 3: Паттерн Semaphore (Ограничение ресурсов)
// Если не нужен полноценный Worker Pool, можно использовать буферизированный канал как семафор.
func exampleSemaphore() {
	maxConcurrency := 5
	semaphore := make(chan struct{}, maxConcurrency)

	for i := 0; i < 20; i++ {
		go func(id int) {
			semaphore <- struct{}{}        // Занимаем слот
			defer func() { <-semaphore }() // Освобождаем слот

			fmt.Printf("Процесс %d выполняется...\n", id)
			time.Sleep(time.Second)
		}(i)
	}
}

// ПРИМЕР 4: Паттерн Pipeline (Трубопровод)
// Разделение сложной задачи на этапы, каждый в своей горутине.
func examplePipeline() {
	// stage 1: генерация
	nums := producer(2, 3, 4, 5)
	// stage 2: возведение в квадрат
	squares := square(nums)
	// stage 3: вывод
	for res := range squares {
		fmt.Println(res)
	}
}

// ПРИМЕР 5: Проверка утечек через runtime.NumGoroutine()
func exampleCheckLeaks() {
	fmt.Println("Горутин до:", runtime.NumGoroutine())

	go func() {
		ch := make(chan int)
		<-ch // Горутина зависнет здесь навсегда (утечка!)
	}()
	time.Sleep(time.Millisecond * 10)
	fmt.Println("Горутин после:", runtime.NumGoroutine())
}

// ПРИМЕР 6: Использование pprof
/*
Для отладки в реальном времени:
1. Импортируем _ "net/http/pprof"
2. Запускаем HTTP сервер: go func() { http.ListenAndServe("localhost:6060", nil) }()
3. В терминале:
   $ go tool pprof http://localhost:6060/debug/pprof/goroutine
   $ (pprof) top
   $ (pprof) list myFunc
*/

// 4. ПОД КАПОТОМ, ATOMIC & CONTEXT (
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. ВНУТРЕННЕЕ УСТРОЙСТВО ГОРУТИН:
   - Stack: Горутина начинает с 2 КБ. Если стек растет, Go выделяет новый
     участок памяти и КОПИРУЕТ туда стек. Это эффективнее, чем фиксированные
     8 МБ у потоков ОС.
   - States: Горутина может быть в состояниях: _Gidle, _Grunnable, _Grunning,
     _Gsyscall, _Gwaiting.

2. ПЛАНИРОВЩИК (SCHEDULER):
   - Syscall handling: Если горутина делает блокирующий системный вызов,
     планировщик отсоединяет поток M от процессора P и создает новый поток M,
     чтобы остальные горутины не простаивали.
   - Netpoller: Для сетевых вызовов Go использует epoll/kqueue, чтобы не
     плодить системные потоки.

3. ПАКЕТ ATOMIC:
   Использует инструкции процессора (CAS - Compare-And-Swap) для изменения
   памяти без мьютексов. Это на порядок быстрее мьютекса, так как нет
   переключения контекста и блокировок.

4. CONTEXT:
   Инструмент для иерархического управления горутинами. Если "голова" (запрос)
   отменяется, все "хвосты" (запросы в БД, API) должны мгновенно завершиться.
*/

// ПРИМЕР 1: Atomic операции (Высокая производительность)
// Мьютекс слишком дорог для простого счетчика. Используем атомики.
func exampleAtomic() {
	var (
		counter int64
		wg      sync.WaitGroup
	)
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			atomic.AddInt64(&counter, 1) // Атомарная инкрементация на уровне CPU
			wg.Done()
		}()
	}
	wg.Wait()
	fmt.Println("Atomic counter:", atomic.LoadInt64(&counter))
}

// ПРИМЕР 2: Context with Timeout (Защита ресурсов)
// Если база данных тормозит, мы не хотим ждать вечно.
func exampleContextTimeout() {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel() // Всегда вызывай cancel для очистки ресурсов!

	resCh := make(chan string)

	go func() {
		// Имитируем долгий запрос в БД
		time.Sleep(1 * time.Second)
		resCh <- "data from db"
	}()

	select {
	case res := <-resCh:
		fmt.Println("Успех:", res)
	case <-ctx.Done():
		fmt.Println("Ошибка:", ctx.Err()) // Напечатает context deadline exceeded
	}
}

// ПРИМЕР 3: Context Propagation (Проброс по слоям)
// Правильный паттерн: передавай ctx первым аргументом во все функции.
func serviceCall(ctx context.Context) {
	// Мы можем проверить, не отменен ли запрос выше по дереву
	select {
	case <-time.After(100 * time.Millisecond):
		fmt.Println("Запрос выполнен")
	case <-ctx.Done():
		fmt.Println("Сервис: запрос отменен, прекращаю работу")
	}
}

// ПРИМЕР 4: Паттерн "Or-Done Channel"
// Позволяет объединить несколько каналов отмены в один.
func or(channels ...chan interface{}) <-chan interface{} {
	switch len(channels) {
	case 0:
		return nil
	case 1:
		return channels[0]
	}

	orDone := make(chan interface{})
	go func() {
		defer close(orDone)
		switch len(channels) {
		case 2:
			select {
			case <-channels[0]:
			case <-channels[1]:
			}
		default:
			select {
			case <-channels[0]:
			case <-channels[1]:
			case <-or(append(channels[3:], orDone)...):
			}
		}
	}()
	return orDone
}

// ПРИМЕР 5: Spinlock на базе Atomic (Экспертно)
// В редких случаях, когда ожидание ОЧЕНЬ короткое, мьютекс медленнее цикла.
type spinlock struct {
	state int32
}

func (s *spinlock) Lock() {
	for !atomic.CompareAndSwapInt32(&s.state, 0, 1) {
		// "Крутимся" в цикле, пока не захватим (активное ожидание)
		// В реальности в Go лучше использовать Mutex, так как он умеет в spin-lock
		// на первых итерациях, а потом усыпляет горутину.
	}
}

func (s *spinlock) Unlock() {
	atomic.StoreInt32(&s.state, 0)
}

// ПРИМЕР 6: Продвинутая синхронизация через sync.Cond
// Используется, когда горутинам нужно ждать какого-то события (не данных).
func ExampleSyncCond() {
	mu := &sync.Mutex{}
	cond := sync.NewCond(mu)
	ready := false

	for i := 0; i < 3; i++ {
		go func(id int) {
			cond.L.Lock()
			for !ready {
				cond.Wait() // Ждем сигнала, отпуская мьютекс
			}
			fmt.Printf("Воркер %d погнал!\n", id)
			cond.L.Unlock()
		}(i)
	}

	time.Sleep(time.Second)
	mu.Lock()
	ready = true
	mu.Unlock()
	cond.Broadcast() // Будим ВСЕХ сразу
}
