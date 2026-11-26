package __goroutines

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/debug"
	"runtime/trace"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// 1. GOROUTINES: ВВЕДЕНИЕ, ЗАПУСК И ЖИЗНЕННЫЙ ЦИКЛ
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. ЧТО ТАКОЕ ГОРУТИНА?
   Это "легковесный поток", управляемый рантаймом Go, а не операционной системой.
   - Память: поток ОС занимает ~1-2 МБ, горутина стартует с 2 КБ.
   - Переключение контекста: у горутин оно происходит быстрее, так как не
     требует обращения к ядру ОС (LWP).
   - Масштабируемость: на одном сервере можно запустить миллионы горутин.
   Горутины живут в пространстве пользователя (User Space) и управляются Runtime Go.

   СРАВНЕНИЕ:
   - Memory: Поток ОС (~1-2 МБ) vs Горутина (от 2 КБ).
   - Creation: Создать поток ОС дорого (вызов ядра), горутину — очень дешево.
   - Context Switch: Переключение между потоками ОС требует сохранения регистров,
     смены режима ядра. У горутин это делает планировщик Go внутри процесса.

2. КЛЮЧЕВОЕ СЛОВО `go`:
   Оно не "запускает функцию параллельно", оно говорит планировщику:
   "Создай новую горутину G и положи её в очередь на исполнение".
   Когда именно она начнет работать — решает планировщик.

3. ПОВЕДЕНИЕ MAIN:
   Запомни: main() — это тоже горутина (Main Goroutine).
   Если она завершается, рантайм вызывает exit, не дожидаясь остальных.
*/
// ПРИМЕР 1: Базовый запуск (Классика)
func printText(s string) {
	fmt.Println(s)
}

func exampleBasic() {
	go printText("Я работаю в фоне") // Создали горутину
	fmt.Println("Я в основной горутине")

	// Без этой паузы мы не увидим текст из горутины
	time.Sleep(10 * time.Millisecond)
}

// ПРИМЕР 2: Анонимные функции
func exampleAnonymous() {
	go func() {
		fmt.Println("Анонимная горутина")
	}()
}

// ПРИМЕР 3: Проблема замыкания***
/* На собесе часто дают код: "Что выведет этот цикл?".
Правильный ответ: Неопределенное поведение, но скорее всего пять раз цифру 5.
*/
func exampleClosureBad() {
	for i := 0; i < 5; i++ {
		go func() {
			fmt.Println(i) // Все горутины ссылаются на ОДНУ И ТУ ЖЕ переменную i
		}()
	}
	time.Sleep(50 * time.Millisecond)
}

// ПРИМЕР 4: Исправление проблемы замыкания (Вариант А — аргументы)
func exampleClosureGood() {
	for i := 0; i < 5; i++ {
		go func(val int) {
			fmt.Println(val)
		}(i)
	}
	time.Sleep(50 * time.Millisecond)
}

// ПРИМЕР 5: Исправление проблемы замыкания (Вариант Б — локальная копия)
func exampleClosureLocal() {
	for i := 0; i < 5; i++ {
		i = i
		go func() {
			fmt.Println(i)
		}()
	}
	time.Sleep(50 * time.Millisecond)
}

// ПРИМЕР 6: Горутины внутри методов структур
type Job struct {
	ID int
}

func (j *Job) process() {
	fmt.Printf("Обработка задачи %d\n", j.ID)
}
func exampleMethod() {
	job := &Job{ID: 10}
	go job.process() // Можно запускать методы
	time.Sleep(10 * time.Millisecond)
}

// ПРИМЕР 7: Запуск горутин в большом количестве
/* Здесь мы проверяем "дешевизну". Попробуй запустить 100к потоков в Java — всё упадет.
В Go 100к горутин — это всего лишь ~200-400 МБ памяти.
*/
func exampleMassive() {
	for i := 0; i < 100000; i++ {
		go func(id int) {
			// Какая-то работа
			_ = id
		}(i)
	}
	fmt.Println("Запущено 100 000 горутин без проблем")
}

// 2. ПЛАНИРОВАНИЕ, СИНХРОНИЗАЦИЯ И МАСШТАБИРОВАНИЕ
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. ГЛУБОКОЕ ОТЛИЧИЕ ОТ ПОТОКОВ (M:N SCHEDULING):
   - Threads (Потоки ОС): Управляются ядром. Переключение (Context Switch) требует
     сохранения сотен регистров и перехода в режим ядра. Это долго (~1-3 мкс).
   - Goroutines: Управляются планировщиком Go. Используется кооперативная
     многозадачность с вытеснением. Переключение происходит в User Space,
     сохраняются только необходимые регистры. Это быстро (~10-20 нс).

2. ПЛАНИРОВЩИК GMP:
   - G (Goroutine): Код и стек.
   - M (Machine): Физический поток ОС.
   - P (Processor): Контекст выполнения. P — это "билет" на выполнение.
     Их количество равно GOMAXPROCS (обычно кол-во ядер).

3. СРЕДСТВА СИНХРОНИЗАЦИИ:
   - Channels: Передача владения данными. "Общайтесь, чтобы делиться памятью".
   - Mutexes: Защита общей памяти. Используются, когда нужно просто обновить
     счетчик или мапу, и логика не требует координации.
*/

// ПРИМЕР 1: Зачем нужен WaitGroup
// "Как дождаться 100 горутин без time.Sleep?".
func exampleWG() {
	var wg sync.WaitGroup

	for i := 0; i < 6; i++ {
		wg.Add(1) // Инкремент счетчика перед запуском
		go func(val int) {
			defer wg.Done() // Декремент при выходе
			fmt.Printf("Воркер %d выполнил задачу\n", val)
		}(i)
	}

	wg.Wait() // Блокировка до тех пор, пока счетчик не станет 0
	fmt.Println("Все задачи завершены")
}

// ПРИМЕР 2: Защита данных через sync.Mutex
// Если 1000 горутин будут инкрементировать одну переменную, часть данных потеряется из-за Race Condition.
func exampleMutex() {
	var mu sync.Mutex
	counter := 0
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()   // Захват замка
			counter++   // Критическая секция (только одна G здесь в один момент)
			mu.Unlock() // Освобождение
		}()
	}
	wg.Wait()
	fmt.Println("Результат с мьютексом:", counter) // Всегда 1000\
}

// ПРИМЕР 3: Каналы как средство синхронизации (Unbuffered)
// Небуферизированный канал — это "точка рандеву". Отправитель ждет получателя.
func exampleChannelSync() {
	done := make(chan struct{}) // Канал-сигнал (пустая структура не ест память)

	go func() {
		fmt.Println("Делаю тяжелую работу...")
		time.Sleep(time.Second)
		close(done) // Закрытие — это тоже сигнал для всех слушателей
	}()

	<-done // Ждем сигнала
	fmt.Println("Готово!")
}

// ПРИМЕР 4: Параллельная обработка (Batch Processing)
/* Сценарий: нужно обработать 1000 URL, но мы не хотим запускать 1000
одновременных запросов, чтобы не забанили. Используем "семафор" на каналах.
*/
func exampleBoundedParallelism() {
	urls := []string{"url1", "url2", "url3", "url4", "url5"}
	maxConcurrency := 2 // Обрабатываем строго по 2 штуки
	semaphore := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		semaphore <- struct{}{} // Занимаем слот (если занято — ждем здесь)

		go func(u string) {
			defer wg.Done()
			defer func() { <-semaphore }() // Освобождаем слот при выходе

			fmt.Println("Скачиваю:", u)
			time.Sleep(500 * time.Millisecond)
		}(url)
	}
	wg.Wait()
}

// ПРИМЕР 5: Атомарные операции (самый быстрый путь)
/* Для простых счетчиков atomic.Add быстрее мьютекса в разы.
 */
func exampleAtomic() {
	var ops uint64
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			atomic.AddUint64(&ops, 1) // На уровне инструкций процессора
			wg.Done()
		}()
	}
	wg.Wait()
	fmt.Println("Ops:", ops)
}

// ПРИМЕР 6: Профилирование — runtime.NumGoroutine
// Как понять в коде, что у нас утечка горутин?
func exampleLeakDetection() {
	fmt.Println("Горутин в начале:", runtime.NumGoroutine())

	for i := 0; i < 10; i++ {
		go func() {
			select {} // Горутина зависнет навсегда
		}()
	}

	time.Sleep(10 * time.Millisecond)
	fmt.Println("Горутин после утечки:", runtime.NumGoroutine())
}

// 3. GOROUTINES: ADVANCED PATTERNS, CHANNELS & PANIC RECOVERY
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

2. БЕЗОПАСНОСТЬ И ОШИБКИ:
   - Использование sync/errgroup — стандарт индустрии для управления группой горутин,
     возвращающих ошибки. Она сама отменит контекст, если одна горутина упадет.

3. ИЗОЛЯЦИЯ ПАНИК:
   - В высоконагруженных системах нельзя позволять одной кривой задаче убить
     весь воркер. Recover ставится максимально близко к месту возможного взрыва.
*/

// ПРИМЕР 1: errgroup для параллельных запросов
// Заменяет WaitGroup, когда нужно собрать первую ошибку и стопнуть всех.
func ExampleErrGroup() {
	g, ctx := errgroup.WithContext(context.Background())
	urls := []string{"api/v1/user", "api/v1/ERROR", "api/v1/posts"}

	for _, url := range urls {
		url = url
		g.Go(func() error {
			if url == "api/v1/ERROR" {
				return fmt.Errorf("сбой запроса к %s", url)
			}
			// Имитация работы с учетом контекста

			select {
			case <-time.After(100 * time.Millisecond):
				fmt.Println("Успешно загружено:", url)
				return nil
			case <-ctx.Done(): // Выходим, если другая горутина вернула ошибку
				return ctx.Err()
			}
		})
	}
	if err := g.Wait(); err != nil {
		fmt.Println("Группа завершена с ошибкой:", err)
	}
}

// ПРИМЕР 2: Recover внутри итерации
// Гарантируем, что паника в одной задаче не прервет обработку всего списка.
func ExampleIsolatedRecover() {
	data := []string{"safe", "BOOM", "safe_again"}
	var wg sync.WaitGroup

	for _, d := range data {
		wg.Add(1)

		go func(val string) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					fmt.Printf("Обработка [%s] упала, но мы идем дальше. Ошибка: %v\n", val, r)
				}
			}()

			if val == "BOOM" {
				panic("критическая ошибка данных")
				fmt.Println("Обработано:", val)
			}
		}(d)
	}
	wg.Wait()
}

// ПРИМЕР 3: Передача ошибок через Result Wrapper
// Правильный способ вернуть и данные, и ошибку из горутины.
type respErr struct {
	data string
	err  error
}

func exampleErrorWrapping() {
	resCh := make(chan respErr, 1)

	go func() {
		resCh <- respErr{err: fmt.Errorf("timeout connection")}
	}()
	res := <-resCh

	if res.err != nil {
		fmt.Println("Поймали ошибку из горутины:", res.err)
	}
}

// 4. RUNTIME DEEP DIVE, SCHEDULING & PROFILING
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. ПЛАНИРОВАНИЕ SYSCALL (HANDOFF VS NETPOLLER):
   - Если горутина делает блокирующий Syscall (чтение файла), M блокируется.
     P отсоединяется (Handoff) и ищет нового M. Это дорого.
   - Для сетевых вызовов Go использует Netpoller (epoll/kqueue). Горутина
     просто паркуется, а поток M идет работать дальше. Это киллер-фича Go.

2. СТЕК: КОПИРОВАНИЕ VS СЕГМЕНТАЦИЯ:
   - Go использует непрерывные стеки. При расширении создается новый кусок,
     старый копируется. Все указатели на стек ПЕРЕПИСЫВАЮТСЯ.
     Поэтому нельзя передавать указатели на стек в C-код через cgo без бубна.

3. ПРИНУДИТЕЛЬНОЕ ВЫТЕСНЕНИЕ (PREEMPTION):
   - С версии 1.14 Go использует асинхронное вытеснение через сигналы (SIGURG).
     Раньше горутина в `for {}` могла заблокировать поток навсегда. Сейчас
     планировщик может прервать её в любой момент.
*/

// ПРИМЕР 1: Защита от бесконечной рекурсии (Limit Stack)
/* Зачем это нужно: По умолчанию лимит стека — 1ГБ. Если у тебя рекурсия,
горутина сожрет всю память сервера до того, как упадет.
SetMaxStack позволяет уронить "плохую" горутину раньше, сохранив жизнь системе.
*/
func exampleSetMaxStack() {
	// Ограничиваем макс. размер стека горутины (по умолчанию 1ГБ на 64бит)
	// Помогает поймать бесконечную рекурсию раньше, чем упадет вся нода.
	previousMax := debug.SetMaxStack(100 * 1024)
	fmt.Printf("Предыдущий лимит: %d\n", previousMax)

	// 2. Функция с "бесконечной" рекурсией
	var recursiveFunc func(depth int)
	recursiveFunc = func(depth int) {
		var dummy [1024]byte
		_ = dummy
		recursiveFunc(depth + 1)
	}

	// 3. Запускаем в отдельной горутине, чтобы не упал main (хотя panic прилетит в runtime)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Printf("Поймали переполнение стека вовремя: %v\n", r)
			}
		}()
		recursiveFunc(0)
	}()

	time.Sleep(50 * time.Millisecond)
}

// ПРИМЕР 2: Использование Runtime Trace
// Трейсинг позволяет увидеть "Work Stealing" вживую: как G прыгают между P.
func exampleTrace() {
	f, _ := os.Create("trace.out")
	defer f.Close()

	trace.Start(f)
	defer trace.Stop()

	// ... тут твой сложный многозадачный код ...
	fmt.Println("Трейсинг запущен. Анализировать через: go tool trace trace.out")
}

// ПРИМЕР 3: Глубокое профилирование блокировок (Оптимизация)
/* Зачем это нужно: Чтобы понять, почему горутины "стоят" и ждут мьютексы.
runtime.SetBlockProfileRate(1) заставляет рантайм фиксировать каждое событие блокировки.
*/
func exampleProfiling() {
	// 1. Включаем сбор профиля блокировок (1 - фиксировать каждую наносекунду ожидания)
	runtime.SetBlockProfileRate(1)

	// 2. Запускаем pprof сервер
	go func() {
		log.Println("Pprof server started on :6060")
		// Теперь идем в браузер: http://localhost:6060/debug/pprof/block
		// Или через консоль: go tool pprof http://localhost:6060/debug/pprof/block
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	// 3. Создаем искусственный затор (Contention)
	var mu sync.Mutex
	for i := 0; i < 10; i++ {
		go func(id int) {
			for {
				mu.Lock()
				time.Sleep(10 * time.Millisecond) // Долгая работа под замком
				mu.Unlock()
			}
		}(i)
	}
}

// ПРИМЕР 4: Lock-Free State(Состояние без блокировки) с использованием atomic.Value
// Позволяет атомарно подменять целые структуры (например, конфиг) без Mutex.
func exampleAtomicValue() {
	type config struct {
		addr string
		port int
	}
	var cfg atomic.Value
	cfg.Store(&config{"localhost", 88088})

	// Чтение без блокировок из любой горутины
	cfgLoad := cfg.Load().(*config)
	fmt.Println("Config Addr:", cfgLoad.addr)

}

// ПРИМЕР 5: Тюнинг GC и лимитов памяти (GOMEMLIMIT)
/* Зачем это нужно: С Go 1.19 вместо GOGC лучше использовать SetMemoryLimit.
Это предотвращает "Out of Memory" (недостаточно памяти), заставляя GC работать чаще,
если мы приближаемся к границе памяти контейнера/сервера.
*/
func exampleGCTune() {
	// 1. Устанавливаем мягкий лимит памяти для всего рантайма (например, 512 МБ)
	// Это говорит GC: "делай что хочешь, но не выходи за 512 МБ"
	debug.SetMemoryLimit(512 * 1024 * 1024)

	// 2. Настраиваем агрессивность GC через GOGC
	// 50 означает: запускай сборку, когда куча выросла на 50% (дефолт 100)
	oldGC := debug.SetGCPercent(50)
	fmt.Printf("Старый GOGC был: %d\n", oldGC)

	// 3. Мониторинг в реальном времени
	go func() {
		var m runtime.MemStats
		for {
			runtime.ReadMemStats(&m)
			fmt.Printf("Alloc: %d MB | NextGC: %d MB | NumGC: %d\n",
				m.Alloc/1024/1024, m.NextGC/1024/1024, m.NumGC)
			time.Sleep(time.Second)
		}
	}()

	// Имитация нагрузки на кучу
	data := make([][]byte, 0)
	for i := 0; i < 100; i++ {
		data = append(data, make([]byte, 10*1024*1024)) // +10 MB
		time.Sleep(100 * time.Millisecond)
	}
}

// ПРИМЕР 6: Управление финализаторами (SetFinalizer)
/* Экспертная тема: что делать, если объект удаляется из памяти.
SetFinalizer - безопасное освобождение внешних ресурсов.
*/
func ExampleFinalizer() {
	type MyObj struct{ ID int }
	obj := &MyObj{1}

	runtime.SetFinalizer(obj, func(o *MyObj) {
		fmt.Printf("Объект %d был очищен из кучи\n", o.ID)
	})
	// obj больше не используется и будет удален GC
}
