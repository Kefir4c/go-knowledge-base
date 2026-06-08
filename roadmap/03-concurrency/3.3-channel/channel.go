package main

import (
	"fmt"
	"time"
)

// Ссылка на статью: https://habr.com/ru/articles/926292/

// Каналы в GO: теория и практика
// Каналы — основной механизм коммуникации между горутинами
/*
		Стуркутура канала.

type hchan struct {
    qcount   uint           // количество элементов в буфере
    dataqsiz uint           // размер кольцевого буфера
    buf      unsafe.Pointer // указатель на буфер данных
    elemsize uint16         // размер одного элемента
    closed   uint32         // флаг закрытого канала
    timer    *timer         // таймер для временных каналов
    elemtype *_type         // тип элемента канала
    sendx    uint           // индекс для записи в буфер
    recvx    uint           // индекс для чтения из буфера
    recvq    waitq          // очередь ожидающих получателей
    sendq    waitq          // очередь ожидающих отправителей
    lock     mutex          // мьютекс для синхронизации
}

		Виды каналов:
	Небуферизованные каналы: синхронная передача данных

Небуферизованные каналы (make(chan T)) реализуют синхронную передачу данных.
Отправитель блокируется до тех пор, пока получатель не будет готов принять данные,
и наоборот. Это создает точку синхронизации между горутинами — операция завершается
только тогда, когда обе стороны готовы к обмену данными.

	Буферизованные каналы: асинхронная передача и производительность

Буферизованные каналы (make(chan T, size)) позволяют асинхронную передачу данных.
Отправитель может поместить данные в буфер и продолжить выполнение, не ожидая получателя,
пока буфер не заполнится полностью.

	Выбор размера буфера также важен для производительности:

* Маленькие буферы (1-10): обеспечивают баланс между производительностью и потреблением памяти
* Средние буферы (10-100): подходят для большинства производственных задач
* Большие буферы (100+): максимизируют производительность, но требуют значительной памяти

	Nil каналы: мощный инструмент для условной логики
Nil каналы — это одна из особенностей Go. Операции с nil каналом блокируются навсегда, что может
показаться бесполезным, но на самом деле это отличный инструмент для conditional logic в selectstatements.
Nil каналы позволяют динамически включать и отключать ветки в select, что критично важно для реализации
некоторых паттернов, таких как graceful shutdown или conditional merging.

	Закрытые каналы
Закрытие канала (close(ch)) сигнализирует о том, что больше никаких данных передаваться не будет.
Это механизм для координации завершения работы:

* Попытка отправить данные в закрытый канал вызывает панику
* Получение из закрытого канала возвращает нулевое значение типа и false в качестве второго параметра
* Закрытый канал можно использовать для уведомления произвольного количества горутин
*/

//Примеры.

// 1. Небуферезированный канал.(синхронизация "рукопожатие")
func primer321() {
	done := make(chan bool)

	go func() {
		fmt.Println("Worker: начинаю тяжёлую работу...")
		time.Sleep(2 * time.Second)
		fmt.Println("Worker: работа закончена, посылаю сигнал")
		done <- true // отправитель блокируется, пока main не получит
	}()

	fmt.Println("Main: жду сигнал от worker...")
	<-done // получатель блокируется, пока worker не отправит
	fmt.Println("Main: получил сигнал, завершаюсь")
}

// 2. Буферизированный канал (асинхронный)
// Пример: производитель и потребитель работают с разной скоростью
func primer322() {
	jobs := make(chan int, 5)

	// Потребитель (медленный)
	go func() {
		for job := range jobs {
			fmt.Printf("Consumer: обрабатываю задачу %d\n", job)
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// Производитель (быстрый)
	for i := 0; i <= 10; i++ {
		fmt.Printf("Producer: отправляю задачу %d\n", i)
		jobs <- i // не блокируется, пока буфер не заполнится
		// после заполнения буфера (5) — отправитель засыпает, пока потребитель не освободит место
	}

	close(jobs)
	time.Sleep(2 * time.Second)
}

// 3. Nil канал (отключение select)
func primer323() {
	var dataChan chan int = make(chan int) // сначала канал активен
	var disabledChan chan int = nil        // отключён

	// Производитель данных
	go func() {
		for i := 0; i <= 10; i++ {
			dataChan <- i
			time.Sleep(200 * time.Millisecond)
		}
		close(dataChan)
	}()

	// Обработчик с возможностью отключения
	for {
		// ВАЖНО: проверяем, не закрыт ли dataChan ДО select
		if dataChan == nil {
			fmt.Println("dataChan nil, выходим")
			break
		}
		select {
		case val, ok := <-dataChan:
			if !ok {
				fmt.Println("dataChan закрыт, отключаем его")
				dataChan = nil
				continue
			}
			fmt.Printf("Обработано: %d\n", val)
		case <-disabledChan:
			// эта ветка никогда не выполнится, потому что disabledChan == nil
			fmt.Println("Никогда не увидите этот текст")
		}

		// Выход, если оба канала nil
		if dataChan == nil {
			fmt.Println("Оба канала отключены, завершаемся")
			break
		}
	}
}

// 4. Закрытый канал
func primer324() {
	stop := make(chan struct{}) // пустой struct{} не занимает память

	// Запускаем 10 воркеров
	for i := 0; i < 10; i++ {
		go func(id int) {
			for {
				select {
				case <-stop: // сигнал остановки
					fmt.Printf("Worker %d: получил stop, выхожу\n", id)
					return
				default:
					// работаем...
					fmt.Printf("Worker %d: работаю...\n", id)
					time.Sleep(500 * time.Millisecond)
				}
			}
		}(i)
	}

	time.Sleep(3 * time.Second)
	fmt.Println("\nОтправляем сигнал всем воркерам!")
	close(stop) // закрытие канала разблокирует все <-stop
	time.Sleep(1 * time.Second)
	fmt.Println("Главная завершается")
}

// 5. range по каналу
/*
Что демонстрирует: range сам определяет, когда канал закрыт, и завершает цикл. Не нужно вручную проверять ok.
*/
func primer325() {
	ch := make(chan int, 3)

	// Пишем в канал
	for i := 0; i < 3; i++ {
		ch <- i
	}
	close(ch)

	// Читаем все значения, пока канал не закрыт
	for val := range ch {
		fmt.Println("range получил:", val)
	}
	// fmt.Println(<-ch)  // вернёт 0, false, но паники не будет
}

// 6. Примеру deadlock (нельзя!!!)

// deadlock 1: небуферизованный канал без получателя
func deadlock1() {
	ch := make(chan int)
	ch <- 42 // отправитель заблокирован навсегда — получателя нет
}

// deadlock 2: получатель без отправителя
func deadlock2() {
	ch := make(chan int)
	<-ch // получатель заблокирован навсегда — отправителя нет
}

// deadlock 3: буферизованный канал и waitgroup
func deadlock3() {
	ch := make(chan int, 2)
	ch <- 1
	ch <- 2
	ch <- 3 // заблокируется — буфер полон, а горутин для чтения нет
}

// deadlock 4: закрытие канала из получателя
func deadlock4() {
	ch := make(chan int)
	go func() {
		close(ch) // должен закрывать отправитель, но это может сработать...
	}()
	ch <- 42 // panic: send on closed channel
}

// ping-pong

// player представляет игрока в пинг-понге
type player struct {
	name string
	hits int
}

// play запускает игру: мяч передаётся между игроками через канал
func play(player1, player2 *player, maxHits int) {
	ball := make(chan *player, 1) // ← решение проблемы!
	done := make(chan bool)

	go func() {
		for p := range ball {
			if p.hits >= maxHits {
				fmt.Printf("\n🏆 %s wins!\n", p.name)
				close(ball)
				done <- true
				return
			}
			fmt.Printf("🏓 %s hits the ball (hit #%d)\n", p.name, p.hits)
			p.hits++
			time.Sleep(500 * time.Millisecond)

			if p == player1 {
				ball <- player2
			} else {
				ball <- player1
			}
		}
	}()

	ball <- player1
	<-done
	fmt.Println("Game over!")
}

func primer326() {
	alice := &player{name: "Alice", hits: 0}
	bob := &player{name: "Bob", hits: 0}
	play(alice, bob, 10)
}

func main() {
	//fmt.Println("Задача 1")
	//primer321()
	//fmt.Println("Задача 2")
	//primer322()
	//fmt.Println("Задача 3")
	//primer323()
	//fmt.Println("Задача 4")
	//primer324()
	//fmt.Println("Задача 5")
	//primer325()
	fmt.Println("Задача 6")
	primer326()
}
