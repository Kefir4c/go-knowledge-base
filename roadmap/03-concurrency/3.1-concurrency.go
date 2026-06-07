package _3_concurrency

import (
	"fmt"
	"runtime"
	"sync"
	"time"
)

//Урок 3.1. Горутины

/*
Теория.

Характеристики       Горутина                              Поток ОС
Стек                 Небольшой(~2Кб), динамически растёт   Фиксированный, объём 1-8Мб
Создание             Дёшево(неск микросекунд)              Дорогое(микросекунды или больше)
Переключение         Планировщик GO - легковесный          Ядро ОС - тяжёлый, переключ контекста
Количетсво           Можно создать сотны тысяч             Тысячу уже проблема
Индентификтаров      Нет уникально ID, но модно п          Есть ID потока
*/

// Почему горутины легче?

/*
* Стек горутины начинается с 2 КБ (у потока ОС — 1–8 МБ).

* Планировщик Go управляет несколькими горутинами внутри одного
потока ОС (модель M:N). Переключение между горутинами не требует
системных вызовов.

* При блокировке (например, на канале или системном вызове) планировщик
просто переключает выполнение на другую горутину в том же потоке.

runtime.Gosched() — отдать процессор другой горутине, приостановить текущую.
*/

// 1. Легковесность: создаём миллион горутин

func primer1() {
	var wg sync.WaitGroup
	const N = 1_000_000
	start := time.Now()
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			// имитация работы
			_ = i * i
		}(i)
	}
	wg.Wait()
	fmt.Printf("%d goroutines finished in %v\n", N, time.Since(start))
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf("Total allocated memory: %.1f MB\n", float64(m.TotalAlloc)/1e6)
}

//2.Использование runtime.Gosched() для кооперативной многозадачности
/*
Без runtime.Gosched() горутина могла бы выполнить все 5 итераций подряд
(особенно если GOMAXPROCS=1). С Gosched() они будут чередоваться. Это
демонстрирует, как можно добровольно отдавать процессор.
*/

func primer2() {
	var wg sync.WaitGroup
	// Две горутины, которые хотят выполняться "справедливо"
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				fmt.Printf("Goroutine %d: iteration %d\n", id, j)
				runtime.Gosched()
			}
		}(i)
	}
	wg.Wait()
}

//3.GOMAXPROCS и параллелизм

func primer3() {
	// Устанавливаем количество потоков ОС, которые могут выполнять горутины одновременно
	runtime.GOMAXPROCS(1) // только один поток ОС — последовательное выполнение
	// runtime.GOMAXPROCS(runtime.NumCPU()) // параллельное выполнение

	start := time.Now()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			time.Sleep(10 * time.Millisecond)
			fmt.Printf("%d ", i)
		}(i)
	}
	wg.Wait()
	fmt.Println("\nTime:", time.Since(start))
}
