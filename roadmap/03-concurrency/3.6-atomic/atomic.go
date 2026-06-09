package main

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

/*
ЧТО ТАКОЕ АТОМАРНАЯ ОПЕРАЦИЯ?

Атомарная операция — это операция, которая выполняется целиком и неделимо
с точки зрения других горутин или потоков. Ни одна другая горутина не может
увидеть промежуточное состояние такой операции. Она либо выполнена полностью,
либо не выполнена вовсе — третьего не дано.

ПОЧЕМУ ОБЫЧНЫЕ ОПЕРАЦИИ НЕ АТОМАРНЫ?

На первый взгляд, простая операция инкремента counter++ выглядит как одно действие.
Но на уровне процессора и памяти она распадается на три отдельных шага:

   Шаг 1: ПРОЧИТАТЬ текущее значение из памяти в регистр процессора
   Шаг 2: УВЕЛИЧИТЬ значение в регистре на 1
   Шаг 3: ЗАПИСАТЬ новое значение обратно в память

Проблема возникает, когда две горутины выполняют эти шаги одновременно:

   Время   Горутина A                 Горутина B
   t1      Читает counter = 0
   t2                                Читает counter = 0
   t3      Увеличивает до 1
   t4                                Увеличивает до 1
   t5      Записывает counter = 1
   t6                                Записывает counter = 1

Результат: counter стал 1, хотя две горутины попытались увеличить его дважды.
Одно увеличение потеряно. Это называется гонкой данных (data race).

КАК РАБОТАЮТ АТОМАРНЫЕ ОПЕРАЦИИ НА УРОВНЕ ПРОЦЕССОРА?

Атомарные операции используют специальные инструкции процессора,
которые блокируют доступ к ячейке памяти на время выполнения операции.

На архитектуре x86 это инструкции с префиксом LOCK:
  - LOCK XADD    (атомарное сложение) — используется в atomic.Add
  - LOCK CMPXCHG (атомарное сравнение с обменом) — используется в CAS
  - XCHG         (атомарный обмен) — используется в atomic.Swap

На архитектуре ARM:
  - LDREX/STREX (load-linked/store-conditional) — пара инструкций,
    которые эмулируют атомарность через проверку условий.

Эти инструкции гарантируют, что между чтением и записью никакой другой
процессор или ядро не смогут обратиться к этой же ячейке памяти.

ВИДИМОСТЬ (MEMORY VISIBILITY) — НЕ МЕНЕЕ ВАЖНАЯ ПРОБЛЕМА

Даже если операция неделима, есть другая проблема: современные процессоры
имеют многоуровневый кэш (L1, L2, L3). Одно ядро может записать значение
в свой кэш L1, а другое ядро всё ещё видит старое значение из своего кэша.

Без специальных инструкций нет гарантии, что запись одной горутины
станет видимой для других горутин. Данные могут "застрять" в кэше.

Решение: атомарные операции в Go автоматически добавляют барьеры памяти
(memory barriers / fences), которые:

  1. STORE BARRIER (барьер записи) — сбрасывает кэш записи, принудительно
     выталкивая данные в основную память, чтобы другие ядра увидели их.

  2. LOAD BARRIER (барьер чтения) — инвалидирует кэш чтения, заставляя
     процессор читать актуальные данные из памяти, а не из локального кэша.

Без этих барьеров атомарная операция была бы только "локально атомарной",
но не глобально видимой.

ЧТО ГАРАНТИРУЕТ АТОМАРНАЯ ОПЕРАЦИЯ В GO?

1. НЕДЕЛИМОСТЬ (atomicity)
   Операция выполняется как единое целое. Никто не увидит промежуточное
   состояние. Для инкремента: никто не увидит ситуацию, когда значение
   уже прочитано, но ещё не записано.

2. ВИДИМОСТЬ (visibility)
   Результат операции становится немедленно виден всем другим горутинам.
   Барьеры памяти гарантируют, что запись не застрянет в кэше ядра.

3. УПОРЯДОЧЕННОСТЬ (ordering)
   Атомарные операции в Go упорядочены относительно друг друга
   (sequentially consistent). Если одна горутина выполняет A.Store(1),
   а затем B.Store(2), то другая горутина, которая увидит B.Load() == 2,
   гарантированно увидит и A.Load() == 1.

ПРИМЕР: АТОМАРНЫЙ ИНКРЕМЕНТ ПРОТИВ ОБЫЧНОГО

Обычный инкремент (НЕ атомарный):
  x++  // компилятор разворачивает в:
       //   temp = x   (чтение из памяти)
       //   temp = temp + 1
       //   x = temp   (запись в память)
       // Между этими инструкциями может вклиниться другая горутина

Атомарный инкремент:
  atomic.AddInt64(&x, 1)
       // одна инструкция LOCK XADD (на x86)
       // + барьеры памяти
       // НИКТО не может вмешаться между чтением и записью

ЧТО НЕ ЯВЛЯЕТСЯ АТОМАРНЫМ (даже если кажется)

Операция                     Почему не атомарна
x = y                        Два действия: чтение y, запись x
x = x + 1                    Три действия: чтение, увеличение, запись
x = x + y                    Чтение x, чтение y, сложение, запись
x = someFunction()           Внутри функции может быть что угодно
s[i] = v                     Для типов > машинного слова (int64 на 32-bit)
x, y = y, x                  Несколько операций обмена
slice = append(slice, v)     Может вызвать переаллокацию и копирование

ЧТО ГАРАНТИРОВАННО АТОМАРНО БЕЗ sync/atomic (только неделимость, без видимости):

  - Чтение и запись указателей (uintptr)
  - Чтение и запись int32 на 32-битных системах
  - Чтение и запись int64 на 64-битных системах
  - Чтение и запись bool (на всех архитектурах)

НО! Это только неделимость, без гарантии видимости между ядрами!
Для флага остановки без atomic.Bool другая горутина может никогда не увидеть true.

СРАВНЕНИЕ ATOMIC И MUTEX (ПОДРОБНО)

Характеристика                 ATOMIC                    MUTEX
Стоимость (нс)                 5-30 нс                   50-200 нс
Блокировка горутины            Нет (спин или инстр. CPU) Да (парковка)
Переключение контекста         Нет                       Да
Барьеры памяти                 Да                        Да
При высоком contention         Busy-loop (высокий CPU)   Парковка (экономия CPU)
Для одной переменной           Отлично                   Избыточно
Для нескольких переменных      Только через Pointer      Естественно
Для сложной логики (if+update) Требует CAS в цикле       Естественно
Удобство чтения кода           Понятно для простого     Понятно для любого

КОГДА ATOMIC, А КОГДА MUTEX?

ATOMIC (быстрее, 5-30 нс):
  - Простые счётчики, флаги, семафоры
  - Инкремент/декремент (одна операция)
  - Load/Store одиночного значения
  - CompareAndSwap (неблокирующие алгоритмы)

MUTEX (медленнее, 50-200 нс):
  - Несколько связанных переменных
  - Сложная логика (if + update)
  - Защита целого блока кода
  - Критическая секция с несколькими операциями

Когда atomic проигрывает:
Если много горутин активно пишут в одну переменную (high contention),
атомарные операции вызовут busy-loop на уровне кэша процессора
(все ядра будут синхронизировать кэш-линию). В этом случае Mutex иногда
оказывается эффективнее, потому что паркует горутины и снижает contention.

ТИПЫ АТОМАРНЫХ ПЕРЕМЕННЫХ (Go 1.19+)

Тип                            Назначение
atomic.Bool                    Флаги, сигналы остановки, переключатели
atomic.Int32 / Int64           Счётчики, метрики, суммы, индексы
atomic.Uint32 / Uint64         Битовые маски, идентификаторы
atomic.Pointer[T]              Атомарная замена целой структуры
atomic.Value                   УСТАРЕЛ! Используйте atomic.Pointer[T]

Почему atomic.Value устарел:
  - Не типобезопасен: можно положить int, а прочитать string → паника
  - Не поддерживает Swap и CompareAndSwap
  - Pointer[T] даёт статическую типизацию и больше возможностей

ОСНОВНЫЕ МЕТОДЫ (на примере atomic.Int64)

Метод                          Что делает
Load()                         Атомарное чтение (с барьером памяти)
Store(v)                       Атомарная запись (с барьером памяти)
Add(delta)                     Атомарное прибавление (возвращает новое значение)
Swap(new)                      Атомарная замена (возвращает старое значение)
CompareAndSwap(old, new)       Замена только если текущее == old
                               Возвращает true если замена произошла

ПРИМЕРЫ ИСПОЛЬЗОВАНИЯ МЕТОДОВ

var v atomic.Int64

v.Store(42)                    // v = 42
x := v.Load()                  // x = 42
newVal := v.Add(10)            // v = 52, newVal = 52
old := v.Swap(100)             // v = 100, old = 52
swapped := v.CompareAndSwap(100, 200)  // swapped = true, v = 200
swapped = v.CompareAndSwap(100, 300)   // swapped = false, v = 200

ПАТТЕРН: CAS В ЦИКЛЕ (OPTIMISTIC CONCURRENCY CONTROL)

Это классический неблокирующий паттерн для обновления значения,
которое могло измениться другой горутиной.

var val atomic.Int64

func update(newValue int64) {
    for {
        old := val.Load()
        // Если кто-то уже изменил val, old не совпадёт с текущим
        if val.CompareAndSwap(old, newValue) {
            break  // успешно обновили
        }
        // не вышло — пробуем снова с новым old
    }
}

Этот паттерн используется в реализации многих неблокирующих структур данных:
связанные списки, очереди, стеки.

РЕАЛЬНЫЙ ПРИМЕР ПОТЕРИ ОПЕРАЦИИ (ГОНКА ДАННЫХ)

Запустите этот код с флагом -race, чтобы увидеть гонку:

  var counter int
  var wg sync.WaitGroup
  for i := 0; i < 1000; i++ {
      wg.Add(1)
      go func() {
          counter++  // ГОНКА ДАННЫХ! три операции, не атомарно
          wg.Done()
      }()
  }
  wg.Wait()
  fmt.Println(counter)  // может быть 999, 1000, 998...

Исправление с atomic:

  var counter atomic.Int64
  var wg sync.WaitGroup
  for i := 0; i < 1000; i++ {
      wg.Add(1)
      go func() {
          counter.Add(1)  // АТОМАРНО!
          wg.Done()
      }()
  }
  wg.Wait()
  fmt.Println(counter.Load())  // ВСЕГДА 1000

ЗОЛОТЫЕ ПРАВИЛА (КОРОТКО)

1. НЕ КОПИРУЙ атомарные переменные — передавай по указателю.
   Копирование копирует внутреннее состояние, атомарность ломается.

2. Атомарность работает ТОЛЬКО для одной переменной.
   Для нескольких переменных используй Mutex или atomic.Pointer[T].

3. Для высокого contention (много писателей) Mutex может быть лучше,
   так как он паркует горутины, а atomic вызывает busy-loop.

4. atomic.Pointer[T] позволяет атомарно менять целую структуру целиком.
   Это мощный паттерн для конфигов, роутеров, состояний сервиса.

5. Всегда используй атомарные операции для флагов и счётчиков,
   которые используются из разных горутин. Это дешевле и проще мьютекса.

6. Для проверки гонок всегда запускай тесты с флагом -race.
   Это ловит 99% проблем с атомарностью и видимостью.
*/

// 1. СЧЁТЧИК С ATOMIC
type atomicCounter struct {
	value atomic.Int64
}

func (ac *atomicCounter) inc() {
	ac.value.Add(1)
}

func (c *atomicCounter) get() int64 {
	return c.value.Load()
}

func primer1() {
	var wg sync.WaitGroup
	ac := atomicCounter{}

	for i := 0; i < 10000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ac.inc()
		}()
	}
	wg.Wait()
	fmt.Println("primer1 - atomic counter:", ac.get())
}

// 2. ATOMIC.BOOL ДЛЯ ФЛАГА ОСТАНОВКИ

type worker struct {
	stop atomic.Bool
}

func (w *worker) run() {
	for !w.stop.Load() {
		time.Sleep(10 * time.Millisecond)
	}
	fmt.Println("primer2 - worker stopped")
}

func primer2() {
	w := worker{}
	go w.run()

	time.Sleep(100 * time.Millisecond)
	w.stop.Store(true)
	time.Sleep(50 * time.Millisecond)
}

// 3. COMPAREANDSWAP (CAS)

func primer3() {
	var val atomic.Int64
	val.Store(43)

	swapped := val.CompareAndSwap(42, 100)
	fmt.Printf("primer3 - swapped 42->100: %v, value: %d\n", swapped, val.Load())

	swapped = val.CompareAndSwap(42, 200)
	fmt.Printf("primer3 - swapped 42->200: %v, value: %d\n", swapped, val.Load())
}

// 4. ATOMIC.POINTER[T] — атомарная замена целой структуры

type config struct {
	host string
	port int
}

var globalCfg atomic.Pointer[config]

func updateCfg(host string, port int) {
	newCfg := &config{
		host: host,
		port: port,
	}
	globalCfg.Store(newCfg)
}

func getConfig() *config {
	return globalCfg.Load()
}

func primer4() {
	updateCfg("localhost", 8080)
	cfg := getConfig()
	fmt.Printf("primer4 - config: %s:%d\n", cfg.host, cfg.port)

	updateCfg("0.0.0.0", 9090)
	cfg = getConfig()
	fmt.Printf("primer4 - updated config: %s:%d\n", cfg.host, cfg.port)
}

// 5. СПИНЛОК (SPINLOCK) НА ATOMIC

type spinlock struct {
	flag atomic.Int32
}

func (s *spinlock) lock() {
	for s.flag.CompareAndSwap(0, 1) {
		runtime.Gosched()
	}
}
func (s *spinlock) unlock() {
	s.flag.Store(0)
}

func primer5() {
	var (
		lock    spinlock
		counter int32
		wg      sync.WaitGroup
	)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				lock.lock()
				atomic.AddInt32(&counter, 1)
				lock.unlock()
			}
		}()
	}
	wg.Wait()
	fmt.Println("primer5 - spinlock counter:", counter)
}

// 6. ATOMIC VS MUTEX (СРАВНЕНИЕ)
type mutexCounter struct {
	mu    sync.Mutex
	value int64
}

func (c *mutexCounter) inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

type atomicCounter2 struct {
	value atomic.Int64
}

func (c *atomicCounter2) inc() {
	c.value.Add(1)
}

func primer6() {
	runBenchmark := func(name string, f func()) {
		start := time.Now()
		f()
		fmt.Printf("%s: %v\n", name, time.Since(start))
	}

	// Mutex
	mc := mutexCounter{}
	runBenchmark("Mutex", func() {
		var wg sync.WaitGroup
		for i := 0; i < 1000000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				mc.inc()
			}()
		}
		wg.Wait()
	})

	// Atomic
	ac := atomicCounter2{}
	runBenchmark("Atomic", func() {
		var wg sync.WaitGroup
		for i := 0; i < 1000000; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				ac.inc()
			}()
		}
		wg.Wait()
	})
}

func main() {
	primer1()
	primer2()
	primer3()
	primer4()
	primer5()
	primer6()
}
