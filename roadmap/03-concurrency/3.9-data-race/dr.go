package __9_data_race

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

/*
ЧТО ТАКОЕ DATA RACE (ГОНКА ДАННЫХ)?

Data Race — это ситуация, когда две или более горутин одновременно обращаются
к одной и той же переменной, хотя бы одна из них пишет, и нет синхронизации.

Пример гонки:
  var counter int  // data race!
  Горутина 1: counter++
  Горутина 2: counter++

ПОЧЕМУ ЭТО ОПАСНО?
  - Результат непредсказуем (может быть 1, 2 или 0)
  - Может привести к панике или некорректным данным
  - Может проявиться только на продакшене под нагрузкой

DATA RACE VS RACE CONDITION — В ЧЁМ РАЗНИЦА?

Data Race:
  - Одновременный доступ к памяти без синхронизации
  - Пример: две горутины пишут в counter без мьютекса

Race Condition:
  - Поведение программы зависит от непредсказуемого порядка выполнения
  - Пример: банковский перевод (проверили баланс, списали, а между ними пришёл другой перевод)

ВАЖНО: Data Race — это всегда баг. Race Condition может быть и без Data Race.
*/

/*
 КОМАНДЫ ДЛЯ ОБНАРУЖЕНИЯ ГОНОК

 Запуск программы с детектором гонок
go run -race main.go

 Компиляция с детектором (бинарник будет тяжелее и медленнее)
go build -race -o myapp main.go

 Тесты с детектором гонок (ОБЯЗАТЕЛЬНО!)
go test -race ./...

 Бенчмарки с детектором
go test -race -bench=. ./...

 Установка утилиты с детектором
go install -race .

 ЦЕНА ВКЛЮЧЕНИЯ -race:
 - Память: в 5-10 раз больше
 - Скорость: в 2-10 раз медленнее
 - Размер бинарника: в 2-3 раза больше

 КОГДА ИСПОЛЬЗОВАТЬ:
 - Всегда при разработке и тестировании
 - Никогда на продакшене (кроме отладки проблем)

 ЦЕНА ВКЛЮЧЕНИЯ -race:
 - Память: в 5-10 раз больше
 - Скорость: в 2-10 раз медленнее
 - Размер бинарника: в 2-3 раза больше

 КОГДА ИСПОЛЬЗОВАТЬ:
 - Всегда при разработке и тестировании
 - Никогда на продакшене (кроме отладки проблем)
*/

// 1. КЛАССИЧЕСКАЯ ГОНКА СЧЁТЧИКА

// С гонкой данных
func counterWithRace() {
	var counter int
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter++ // DATA RACE!
		}()
	}
	wg.Wait()
	fmt.Println("counterWithRace:", counter)
}

// Исправление 1: атомарные операции
func counterAtomic() {
	var counter atomic.Int64
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter.Add(1)
		}()
	}
	wg.Wait()
	fmt.Println("counterAtomic:", counter.Load())
}

// Исправление 2: мьютекс
func counterMutex() {
	var mu sync.Mutex
	var counter int
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			counter++
			mu.Unlock()
		}()
	}
	wg.Wait()
	fmt.Println("counterMutex:", counter)
}

func primer1() {
	fmt.Println("=== primer1: Счётчик с гонкой ===")
	counterWithRace()
	counterAtomic()
	counterMutex()
}

// 2. ГОНКА ПРИ РАБОТЕ СО СЛАЙСОМ

// Гонка: одновременное добавление в слайс
func sliceRace() {
	results := make([]int, 0)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results = append(results, i) // DATA RACE!
		}(i)
	}
	wg.Wait()
	fmt.Println("sliceRace длина:", len(results))
}

// Исправление: мьютекс
func sliceFixed() {
	var mu sync.Mutex
	results := make([]int, 0)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mu.Lock()
			results = append(results, i)
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	fmt.Println("sliceFixed длина:", len(results))
}

func primer2() {
	fmt.Println("\n=== primer2: Гонка со слайсом ===")
	sliceRace()
	sliceFixed()
}

// 3. ГОНКА С КАРТОЙ (MAP)

// Гонка: map небезопасен для конкурентного доступа
func mapRace() {
	m := make(map[int]string)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m[i] = fmt.Sprintf("value%d", i) // DATA RACE! + может panic
		}(i)
	}
	wg.Wait()
	fmt.Println("mapRace размер:", len(m))
}

// Исправление: sync.Map

// Гонка: map небезопасен для конкурентного доступа
func mapRace() {
	m := make(map[int]string)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m[i] = fmt.Sprintf("value%d", i) // DATA RACE! + может panic
		}(i)
	}
	wg.Wait()
	fmt.Println("mapRace размер:", len(m))
}

// Исправление: sync.Map
func mapFixed() {
	var m sync.Map
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			m.Store(i, fmt.Sprintf("value%d", i))
		}(i)
	}
	wg.Wait()

	count := 0
	m.Range(func(_, _ any) bool {
		count++
		return true
	})
	fmt.Println("mapFixed размер:", count)
}

// Исправление: мьютекс
func mapFixedMutex() {
	var mu sync.Mutex
	m := make(map[int]string)
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mu.Lock()
			m[i] = fmt.Sprintf("value%d", i)
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	fmt.Println("mapFixedMutex размер:", len(m))
}

func primer3() {
	fmt.Println("\n=== primer3: Гонка с map ===")
	mapRace()
	mapFixed()
	mapFixedMutex()
}

// 4. ГОНКА С ЗАКРЫТИЕМ КАНАЛА

// Правильно: закрывает только один
func channelFixed() {
	ch := make(chan int)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := 0; i < 10; i++ {
			ch <- i
		}
	}()

	go func() {
		<-done
		close(ch)
	}()

	for range ch {
		// чтение
	}
	fmt.Println("channelFixed: завершено")
}

func primer4() {
	fmt.Println("\n=== primer4: Каналы ===")
	channelFixed()
}

// 5. HAPPENS-BEFORE

// Канал гарантирует порядок (happens-before)
func channelHappensBefore() {
	var data string
	done := make(chan struct{})

	go func() {
		data = "hello" // пишем
		close(done)    // happens-before чтения done
	}()

	<-done            // happens-before чтения data
	fmt.Println(data) // гарантированно видит "hello"
}

// Без синхронизации — гонка
func noSynchronization() {
	var data string
	done := make(chan bool)

	go func() {
		data = "world"
		done <- true
	}()

	// Нет гарантии, что data видна
	<-done
	fmt.Println(data) // может быть "world", а может и нет
}

func primer5() {
	fmt.Println("\n=== primer5: Happens-Before ===")
	channelHappensBefore()
	noSynchronization()
}

// 6. ТИПИЧНЫЕ АНТИ-ПАТТЕРНЫ

// Антипаттерн: time.Sleep для синхронизации
func sleepSync() {
	var data string

	go func() {
		time.Sleep(10 * time.Millisecond)
		data = "hello"
	}()

	time.Sleep(20 * time.Millisecond) // никогда не гарантирует порядок!
	fmt.Println("sleepSync:", data)   // может сработать, может нет
}

// Антипаттерн: флаг без атомиков
func flagSync() {
	var done bool
	var data string

	go func() {
		data = "hello"
		done = true // DATA RACE!
	}()

	for !done { // DATA RACE!
		// busy loop
	}
	fmt.Println("flagSync:", data)
}

func primer6() {
	fmt.Println("\n=== primer6: Антипаттерны ===")
	sleepSync()
	// flagSync() // раскомментировать осторожно — может зависнуть
	fmt.Println("flagSync: закомментирован (вызывает data race)")
}
