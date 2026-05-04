package channel

import (
	"context"
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

//3. RANGE, SELECT
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

//3. Pattern. Прописаны в отдельном файле,вот так вот.

//4. ПОДВОДНЫЕ КАМНИ
/*
ПОДВОДНЫЕ КАМНИ:
1. DEADLOCK (взаимная блокировка)
2. ОТПРАВКА В ЗАКРЫТЫЙ КАНАЛ (panic)
3. SELECT БЕЗ DEFAULT (блокировка)
4. ГЛОБАЛЬНЫЕ КАНАЛЫ (кто закрывает?)
5. FOR RANGE БЕЗ CLOSE (deadlock)
*/

// Deadlock и for range без close:
func deadlock() {
	ch := make(chan int)

	go func() {
		for i := 0; i < 3; i++ {
			ch <- i
		}
		// close(ch) закоммитил специально
	}()

	for i := 0; i < 4; i++ { // читаем больше, чем отправляем
		fmt.Println(<-ch)
	}
}

// ОТПРАВКА В ЗАКРЫТЫЙ КАНАЛ -> PANIC
func closedChannelPanic() {
	ch := make(chan int)
	close(ch)

	// ch <- 42  //  PANIC: send on closed channel

	//А читать из закрытого можно
	v, ok := <-ch
	fmt.Printf("value: %v, ok: %v\n", v, ok) // value: 0, ok: false
}

// SELECT БЕЗ DEFAULT -> ВЕЧНАЯ БЛОКИРОВКА
func selectWithoutDefault() {
	ch := make(chan int)

	select {
	case <-ch:
		fmt.Println("received")
		// нет default → будет ждать вечно
	}
}

// SELECT С DEFAULT -> НЕ БЛОКИРУЕТСЯ (как исправить)
func selectWithDefault() {
	ch := make(chan int)

	select {
	case <-ch:
		fmt.Println("received")
	default:
		fmt.Println("no data, continue")
	}
}

// ГЛОБАЛЬНЫЙ КАНАЛ (проблема: кто закрывает?)
var globalCh = make(chan int) // ⚠️ антипаттерн

func problemGlobalChannel() {
	// Кто должен закрыть globalCh?
	// Если закроет одна горутина, другие не смогут отправлять
	// Если не закроет никто — range никогда не завершится
}

// 5. BEST PRACTICES
/*
1.НАПРАВЛЕННЫЕ КАНАЛЫ — ВСЕГДА УКАЗЫВАЙ НАПРАВЛЕНИЕ
2.КТО СОЗДАЛ — ТОТ И ЗАКРЫВАЕТ (В ИДЕАЛЕ)
3.ВСЕГДА ПРОВЕРЯЙ ОТКРЫТ КАНАЛ (ПРИ ПОЛУЧЕНИИ)
4.select ВСЕГДА ДОЛЖЕН ИМЕТЬ default ИЛИ TIMEOUT (в неблокирующих сценариях)
5.РАЗМЕР БУФЕРА — ОСМЫСЛЕННО, А НЕ НАУГАД
6.CLOSE() — ТОЛЬКО ДЛЯ СИГНАЛА "ДАННЫЕ КОНЧИЛИСЬ"
7.ПЕРЕДАВАЙТЕ КАНАЛЫ КАК АРГУМЕНТЫ, А НЕ ВОЗВРАЩАЙТЕ ИХ БЕЗ НЕОБХОДИМОСТИ
8.ИСПОЛЬЗУЙТЕ nil КАНАЛЫ ДЛЯ ОТКЛЮЧЕНИЯ CASE'ов В SELECT
*/

// 1.
// ПЛОХО: непонятно, кто что делает
func process(ch chan int) {
	// можно и отправить, и получить — ошибки закрадываются легко
}

// ХОРОШО: сразу видно, что функция только получает
func consumer(ch <-chan int) {
	for v := range ch {
		fmt.Println(v)
	}
	// ch <- 42 — не скомпилируется, защита от дурака
}

// ХОРОШО: функция только отправляет
func producer(ch chan<- int, values ...int) {
	for _, v := range values {
		ch <- v
	}
	close(ch)
	// <-ch — не скомпилируется
}

// 2.
// ПЛОХО: потребитель закрывает канал
func badPattern() {
	ch := make(chan int)

	// Потребитель
	go func() {
		for v := range ch {
			fmt.Println(v)
		}
		close(ch) // потребитель закрыл — но кто его просил?
	}()

	// Производитель
	for i := 0; i < 5; i++ {
		ch <- i
	}
}

// ХОРОШО: производитель создал и закрыл
func goodPattern() {
	ch := make(chan int)

	// Производитель
	go func() {
		for i := 0; i < 5; i++ {
			ch <- i
		}
		close(ch) //производитель закрывает
	}()

	// Потребитель
	for v := range ch {
		fmt.Println(v)
	}
}

//3.
// ПЛОХО: не знаем, закрыт канал или нет
value := <-ch  // если канал закрыт — получим zero value, но не поймем

// ✅ ХОРОШО: проверяем через ok
value, ok := <-ch
if !ok {
fmt.Println("канал закрыт, данных больше не будет")
return
}
fmt.Println("получено:", value)

// ЛУЧШЕ: range сам проверяет закрытие
for value := range ch {
fmt.Println(value)
}

//4.
// ПЛОХО: блокировка навечно
func badSelect() {
	ch := make(chan int)
	select {
	case v := <-ch:
		fmt.Println(v)
		// нет default — будет ждать вечно
	}
}
// ХОРОШО: неблокирующее чтение
func nonBlockingRead() {
	ch := make(chan int)
	select {
	case v := <-ch:
		fmt.Println(v)
	default:
		fmt.Println("no data, continue")  // не блокируется
	}
}
// ХОРОШО: с таймаутом
func withTimeout(){
	ch:= make(chan int)
	select {
	case v:= <-ch:
		fmt.Println(v)
	case <-time.After(1 * time.Second):
		fmt.Println("timeout")
	}
}
// ХОРОШО: с отменой через context
func withContext(){
	ctx,cancel:= context.WithTimeout(context.Background(),1 * time.Second)
	defer cancel()

	ch:= make(chan int)
	select {
	case v:= <-ch:
		fmt.Println(v)
	case <-ctx.Done():
		fmt.Println("cancelled:", ctx.Err())
	}
}

//5.
// ПЛОХО: буфер 1000 "на всякий случай"
ch := make(chan int, 1000)  // пустая трата памяти

// ПЛОХО: буфер 1 — почти как небуфер, но без понимания
ch := make(chan int, 1)

// ХОРОШО: буфер = ожидаемое количество задач
type Task struct { /* ... */ }
tasks := make(chan Task, 100)  // ожидаем до 100 одновременных задач

// ХОРОШО: буфер сглаживает пики
requests := make(chan Request, 50)  // 50 запросов могут подождать

// ЛУЧШЕ: без буфера, когда синхронность важна
sync := make(chan struct{})  // для синхронизации

//6.
// ПЛОХО: закрыл после одного сообщения
func badClose() {
	ch := make(chan int)
	go func() {
		ch <- 42
		close(ch)  // зачем? читатель и так знает, что одно сообщение
	}()
	fmt.Println(<-ch)  // всё равно читаем
}

// ХОРОШО: закрываем, когда много сообщений и range
func goodClose() {
	ch := make(chan int)
	go func() {
		for i := 0; i < 10; i++ {
			ch <- i
		}
		close(ch)  // сигнал: всё, больше не будет
	}()
	for v := range ch {  // range завершится после close
		fmt.Println(v)
	}
}

// ХОРОШО: закрываем для разблокировки нескольких читателей
func broadcastClose(){
	ch:= make(chan int)

	for i:=0; i < 3; i++{
		go func(id int) {
			<-ch // блокируем
			fmt.Printf("worker %d released\n", id)
		}(i)
	}

	time.Sleep(time.Second)
	close(ch) // все 3 читателя разблокируются одновременно
}

//7.
// ПЛОХО: возвращает канал — непонятно, кто закрывает
func badGenerator() chan int {
	ch := make(chan int)
	go func() {
		ch <- 42
	}()
	return ch
}

// ХОРОШО: возвращает read-only канал, ясно что закрывает генератор
func goodGenerator() <-chan int {
	ch := make(chan int)
	go func() {
		defer close(ch)  // генератор закрывает
		ch <- 42
	}()
	return ch
}

// Использование
for v := range goodGenerator() {
fmt.Println(v)
}

//8.
func dynamicSelect(){
	ch1 := make(chan int)
	ch2 := make(chan int)

	go func() {
		time.Sleep(time.Second)
		ch1<- 1
	}()

	go func() {
		time.Sleep(2* time.Second)
		ch2<- 2
	}()

	for i:=0; i < 2;i++{
		select {
		case v:= <-ch1:
			fmt.Println("from ch1:", v)
			ch1 = nil
		case v := <-ch2:
			fmt.Println("from ch2:", v)
			ch2 = nil
		}
	}
}