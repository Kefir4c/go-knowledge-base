package channel

import (
	"fmt"
	"time"
)

//КАНАЛЫ В GO: ПОЛНОЕ ПОНИМАНИЕ ОТ БАЗОВЫХ ПРИНЦИПОВ К ПРОДВИНУТОМУ ПОНИМАНИЮ

/*
Как создавать и использовать каналы
Направленные каналы и их применение
Буферизованные и небуферизованные каналы
Range по каналам и select
Паттерны использования в реальных сценариях
Подводные камни и best practices
*/

//1. ЧТО ТАКОЕ КАНАЛЫ
/*
ЧТО ЭТО:
- Канал — это механизм для передачи данных между горутинами.
- Реализует принцип "коммуникация через обмен" (communicate by sharing).
- Это основной инструмент для синхронизации горутин.

ПОЧЕМУ ЭТО ВАЖНО:
- Позволяет безопасно обмениваться данными между горутинами.
- Избегает race conditions (гонок данных).
- Упрощает реализацию параллельных алгоритмов.

ОСНОВНЫЕ ОПЕРАЦИИ:
- Создание: make(chan T) или make(chan T, N)
- Отправка: ch <- value
- Получение: value := <-ch
- Закрытие: close(ch)
- Range: for value := range ch
ПРИМЕР:
*/
// Базовое использование каналов
func demBasicChannel() {
	// Создание небуферизованного канала
	ch := make(chan string)

	// Горутина для отправки данных
	go func() {
		ch <- "Hello, World!"
		close(ch) // Закрываем канал после отправки
	}()

	// Получение данных
	message := <-ch
	fmt.Println(message) // "Hello, World!"
}

// Блокирующая отправка в небуферизованный канал
func demBlockingSecond() {
	ch := make(chan int)

	// Горутина отправляет данные
	go func() {
		fmt.Println("Sending 1")
		ch <- 777
		fmt.Println("Send 1")
	}()

	// Горутина получения данных
	go func() {
		fmt.Println("Receiving...")
		value := <-ch
		fmt.Printf("Receiving: %d", value)
	}()

	time.Sleep(200 * time.Millisecond)
}

/*
// Закрытие канала обязательно для range
func demRangeWithoutClose() {
	ch := make(chan int)

	// Горутина отправляет данные
	go func() {
		for i := 0; i < 5; i++ {
			ch <- i
		}
		//close(ch) --- закрытие канала не происходит
	}()
	// Range будет блокироваться навсегда
	// for value := range ch {
	// 	fmt.Println(value)
	// }
}
*/
/*
Подводные камни данного блока:
Отправка в закрытий канал == panic
Получение из закрытого канала -> нулевое значение
Отправка в небуферизованный канал блокирует горутину, пока не будет получено данные
*/

//2.НАПРАВЛЕННЫЕ КАНАЛЫ И БУФЕРИЗАЦИЯ
/*
НАПРАВЛЕННЫЕ КАНАЛЫ:
- chan<- T — канал только для отправки в канал данных
- <-chan T — канал только для получения из канала данных
- Позволяют явно указать намерения при передаче данных
- Улучшают читаемость и безопасность кода

ПОЧЕМУ ЭТО ВАЖНО:
- Явно указывает, как будет использоваться канал
- Предотвращает случайные ошибки (попытка получить из отправного канала)
- Помогает при проектировании API

БУФЕРИЗОВАННЫЕ КАНАЛЫ:
- Создается как make(chan T, N)
- Позволяет отправить N элементов без блокировки
- Полезен для декоплинга отправки и получения
*/

// Функция принимает только канал для отправки
func sendMessage(ch chan<- string) {
	ch <- "Hello Bibik"
}

// Функция принимает только канал для получения
func receiveMessage(ch <-chan string) {
	message := <-ch
	fmt.Println(message)
}

func demDirectedChannels() {
	ch := make(chan string)
	go sendMessage(ch)
	receiveMessage(ch)
}

func demBufferedChannels() {
	ch := make(chan int, 3)

	// Отправка без блокировок
	ch <- 1
	ch <- 2
	ch <- 3 // Третий элемент не блокирует горутину

	go func() {
		ch <- 4 //Блокируется пока не освободиться место в буфере
	}()

	fmt.Println(<-ch) // 1
	fmt.Println(<-ch) // 2
	fmt.Println(<-ch) // 3
	fmt.Println(<-ch) // 4
}

// Использование буфера
func demBufferUsage() {
	ch := make(chan string, 5)

	for i := 0; i < 5; i++ {
		ch <- fmt.Sprintf("Message: %d", i)
	}

	go func() {
		ch <- "Message: 6"
	}()

	for i := 0; i < 5; i++ {
		fmt.Println(<-ch)
	}
}

// Ошибочное использование направленных каналов
/*
func demDirectedChannelError() {
	ch := make(chan string)

	go func(ch chan<- string) {
		value := <-ch // Ошибка. В функции мы указали что данные записываем
		fmt.Println("Received:", value)
	}(ch)
	ch <- "Hello"
}
*/

/*
Подводные камни:
Направленные каналы нельзя использовать не по назначению
Буферизованный канал не гарантирует порядок
Избыточный буфер → избыточное потребление памяти
Недостаточный буфер → блокировки
*/

//RANGE, SELECT
/*
RANGE ПО КАНАЛАМ:
- Позволяет получать данные из канала до его закрытия
- Автоматически завершается при закрытии канала

SELECT:
- Позволяет ожидать несколько операций с каналами
- Реализует неблокирующую синхронизацию
- Может использоваться с таймаутами
*/
// RANGE

// Range по каналу до закрытия
func demRange() {
	ch := make(chan int)

	// Горутина отправляет данные и закрывает канал
	go func() {
		for i := 0; i < 5; i++ {
			ch <- i
		}
		close(ch)
	}()

	for value := range ch {
		fmt.Println("Received:", value)
	}
}

// Range без закрытия канала блокируеться
func demRangeWithoutClose() {
	ch := make(chan int)

	// Горутина отправляет данные
	go func() {
		for i := 0; i < 5; i++ {
			ch <- i
		}
		// close(ch) // Закрытие не происходит
	}()

	// Range будет блокироваться навсегда
	// for value := range ch {
	// 	fmt.Println(value)
	// }
}

//SELECT

// Select с несколькими каналами
func demSelect() {
	ch1 := make(chan string)
	ch2 := make(chan string)

	go func() {
		time.Sleep(100 * time.Millisecond)
		ch1 <- "from ch1"
	}()
	go func() {
		time.Sleep(200 * time.Millisecond)
		ch1 <- "from ch2"
	}()

	// Ожидаем первые готовые каналы
	select {
	case msg1 := <-ch1:
		fmt.Println("Message 1:", msg1)
	case msg2 := <-ch2:
		fmt.Println("Message 1:", msg2)
	default:
		fmt.Println("No message received")
	}
}

// Select с таймаутом
func demSelectWithTimeout() {
	ch := make(chan string)

	go func() {
		time.Sleep(100 * time.Millisecond)
		ch <- "data"
	}()

	select {
	case data := <-ch:
		fmt.Println("Data received:", data)
	case <-time.After(100 * time.Millisecond):
		fmt.Println("timeout")
	}
}
