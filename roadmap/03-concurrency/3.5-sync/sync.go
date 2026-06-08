package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"sync"
	"time"
)

//Урок 3.5. Пакет sync
// Теория: Mutex, RWMutex, WaitGroup, Once, Cond, Pool.
// Читатели vs писатели, deadlock'и.

// Mutex

/*
Зачем нужен мьютекс?

Когда несколько горутин одновременно обращаются к одной переменной
(например, увеличивают счётчик), возникает гонка данных
(race condition). Результат непредсказуем: мы можем потерять часть операций.
Мьютекс позволяет защитить критическую секцию — код, который одновременно
может выполнять только одна горутина.

Как работает:
Мьютекс имеет два состояния: заблокирован и разблокирован.

* Lock() — захватывает мьютекс. Если он уже занят, горутина засыпает
  (блокируется) до его освобождения.
* Unlock() — освобождает мьютекс, пробуждая одну из ждущих горутин.

Когда использовать:

* Защита общей переменной (счётчики, кэши, разделяемые структуры)
* Гарантия атомарности группы операций (например, проверка и обновление мапы)
* Когда вам не подходят атомарные операции (пакет atomic) или каналы (избыточны)

Важные правила:

* Мьютекс нельзя копировать — скопируется его внутреннее состояние
(флаг блокировки), что приведёт к deadlock'ам.

* Всегда используйте defer mu.Unlock() после Lock() — это гарантирует
освобождение даже при панике.

* Не делайте Lock() внутри другой критической секции,
если это не продумано (риск deadlock).

* Мьютекс не реентерабелен — одна и та же горутина не может повторно
захватить мьютекс (будет deadlock).

Когда НЕ использовать:

* Для простого увеличения счётчика — лучше atomic.AddInt64.
* Для синхронизации через сообщения — каналы более идиоматичны.
* Для одноразовой инициализации — sync.Once.
*/

// Примеры

// 1. защита счётчика от гонок
type counter struct {
	mu    sync.Mutex
	value int
}

func (c *counter) inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

func (c *counter) get() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

func primerMu1() {
	var wg sync.WaitGroup
	c := counter{}

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.inc()
		}()
	}

	wg.Wait()
	fmt.Println("primer1 итог:", c.get())
}

// 2. deadlock при копировании мьютекса (НЕПРАВИЛЬНЫЙ код)
type container struct {
	mu   sync.Mutex
	data map[string]string
}

// receiver по значению → копия мьютекса
func (c container) setBad(key, val string) {
	defer c.mu.Unlock()
	c.mu.Lock()
	c.data[key] = val
}

func primerMu2() {
	c := container{data: make(map[string]string)}
	c.setBad("key", "value") // мьютекс не работает, гонка!
	fmt.Println("primer2 (плохо):", c.data)
}

// 3. правильное использование указателя
type containerGood struct {
	mu   sync.Mutex
	data map[string]string
}

func (c *containerGood) set(key, val string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = val
}

func (c *containerGood) get(key string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.data[key]
}

func primerMu3() {
	c := containerGood{data: make(map[string]string)}
	c.set("key", "value")
	fmt.Println("primer3 (хорошо):", c.get("key"))
}

// 4. deadlock при повторном захвате мьютекса (НЕПРАВИЛЬНЫЙ код)

var mu sync.Mutex

func a() {
	defer mu.Unlock()
	mu.Lock()
	b()
}

func b() {
	defer mu.Unlock()
	mu.Lock()
}

func primerMu4() {
	// раскомментировать с осторожностью
	// a()
	fmt.Println("primer4: deadlock если раскомментировать a()")
}

// 5. защита мапы с обычным Mutex
type safeMap struct {
	mu sync.Mutex
	m  map[string]int
}

func (s *safeMap) set(key string, val int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = val
}

func (s *safeMap) get(key string) (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	val, ok := s.m[key]
	return val, ok
}

func primerMu5() {
	sm := safeMap{m: make(map[string]int)}
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			sm.set(fmt.Sprintf("key%d", i), i)
		}(i)
	}

	wg.Wait()

	for i := 0; i < 10; i++ {
		val, _ := sm.get(fmt.Sprintf("key%d", i))
		fmt.Printf("key%d: %d\n", i, val)
	}
}

// RWMutex

/*
Зачем нужен RWMutex, если есть обычный Mutex?
Когда у вас много горутин читают общую структуру и лишь иногда пишут
в неё, обычный Mutex заставляет читателей ждать друг друга, хотя они
не мешают друг другу. RWMutex позволяет неограниченному числу читателей
владеть блокировкой одновременно, а писатель — эксклюзивно.

Как работает:

* RLock() / RUnlock() — блокировка для чтения. Много читателей
могут входить одновременно.
* Lock() / Unlock() — эксклюзивная блокировка для записи. Блокирует всех
читателей и других писателей.

Когда использовать:

* Кэш (часто читаем, редко обновляем)
* Конфигурация, загружаемая из файла (больше чтений)
* База данных с большим количеством запросов SELECT и редкими UPDATE/INSERT

Важные правила:

* Если писатель уже взял Lock(), все последующие RLock() будут ждать его освобождения.
* Если читатели уже взяли RLock(), попытка писателя взять Lock()
заблокируется, пока все читатели не отпустят блокировку.
* Нельзя обновить RWMutex до write-блокировки из read-блокировки — это deadlock.
* RLock() можно вызывать рекурсивно (одна горутина может взять несколько
read-блокировок, нужно столько же раз вызвать RUnlock()).
* RWMutex тоже нельзя копировать (как и обычный Mutex).

Когда НЕ использовать:

* Чтение и запись примерно одинаково часты → лучше обычный
Mutex (накладные расходы на RWMutex выше).
* Короткие критические секции → обычный Mutex проще и достаточно быстр.
*/

// 1. простая защита кэша с RWMutex

type cache struct {
	mu    sync.RWMutex
	items map[string]string
}

func (c *cache) get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.items[key]
	return val, ok
}

func (c *cache) set(key, val string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items[key] = val
}

func primerRMu1() {
	c := cache{items: make(map[string]string)}
	c.set("foo", "bar")
	fmt.Println(c.get("foo")) // bar, true
}

// 2. конкурентное чтение без блокировок друг друга

func primerRMu2() {
	c := cache{items: make(map[string]string)}
	c.set("a", "1")
	c.set("b", "2")

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			val, _ := c.get("a")
			fmt.Println("reader got", val)
			// все 10 горутин читают параллельно
		}()
	}
	wg.Wait()
}

// 3. писатель блокирует читателей
func primerRMu3() {
	c := cache{items: make(map[string]string)}
	c.set("key", "many")

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("writer: trying to lock...")
		defer c.mu.Unlock()
		c.mu.Lock()
		fmt.Println("writer: locked, updating...")
		time.Sleep(2 * time.Second)
		c.items["key"] = "noy noy noy"
		fmt.Println("writer: done")
	}()

	time.Sleep(100 * time.Millisecond) // даём писателю начать первым (не обязательно)

	// читатели
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			fmt.Printf("reader %d: trying to read...\n", id)
			defer c.mu.RUnlock()
			c.mu.RLock()
			fmt.Printf("reader %d: read %s\n", id, c.items["key"])
		}(i)
	}

	wg.Wait()
}

// sync.Once

/*
Зачем нужен sync.Once?
Гарантирует, что переданная функция выполнится только один раз, даже если
её вызовут из сотни горутин одновременно. Это решает проблему ленивой
инициализации и «одноразовых» действий без гонок.

Как работает:
Внутри есть счётчик и мьютекс. Первая горутина, вызвавшая Do(), выполняет
функцию, остальные блокируются до завершения первого вызова. После этого
Do() больше не вызывает функцию (просто возвращает управление).

Когда использовать:

* Ленивая инициализация синглтона (например, подключение к БД, чтение конфига)

* Закрытие канала (чтобы закрыть только один раз)

* Настройка глобального состояния (например, установка логгера, парсера флагов)

Важные правила:

* Функция не принимает аргументов (если нужно передать параметры — используйте замыкание).

* Once не возвращает ошибку; если внутри функции паника — паника
пробьёт наружу, и Once останется неинициализированным
(повторный вызов снова выполнит функцию).

* Once нельзя копировать (как и Mutex).

* Не используйте Once для инициализации, которая может вернуть ошибку
— лучше сделайте отдельную функцию, возвращающую ошибку, и проверяйте её.

Когда НЕ использовать:

* Если инициализация может быть вызвана только в одном месте — просто
сделайте её явно в main() или init().

* Если нужна зависимость от параметров (например, открыть соединение
с разными адресами) — Once не подходит.
*/

// 1. ленивый синглтон
var (
	singleton     *bigStruct
	singletonOnce sync.Once
)

type bigStruct struct {
	data map[string]int
}

func getSingleton() *bigStruct {
	singletonOnce.Do(func() {
		fmt.Println("  -> создаём singleton один раз")
		singleton = &bigStruct{data: make(map[string]int)}
		singleton.data["counter"] = 1
	})
	return singleton
}

func primerOn1() {
	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s := getSingleton()
			s.data["counter"]++
		}()
	}
	wg.Wait()
	fmt.Println("primer1 counter:", getSingleton().data["counter"])
}

// 2. закрытие канала один раз
var (
	closeSignalOnce sync.Once
	done            = make(chan struct{})
)

func closeOnce() {
	closeSignalOnce.Do(func() {
		fmt.Println("  -> закрываем done канал")
		close(done)
	})
}

func primerOn2() {
	for i := 0; i < 3; i++ {
		go func(id int) {
			fmt.Printf(" горутина %d вызывает closeOnce\n", id)
			closeOnce()
		}(i)
	}
	<-done
	fmt.Println("primer2: канал закрыт, все горутины получили сигнал")
}

// 3. инициализация с возможной паникой
var (
	appConfig     *appConfigStruct
	appConfigOnce sync.Once
)

type appConfigStruct struct {
	env string
}

func loadAppConfig() *appConfigStruct {
	// имитация загрузки
	return &appConfigStruct{env: "production"}
}

func getAppConfig() *appConfigStruct {
	appConfigOnce.Do(func() {
		fmt.Println("загружаем конфиг один раз")
		appConfig = loadAppConfig()
		if appConfig.env == "" {
			panic("пустое окружение") // пример паники
		}
	})
	return appConfig
}

func primerOn3() {
	cfg := getAppConfig()
	fmt.Printf("primer3: config env = %s\n", cfg.env)
}

// 4. неправильное использование Once (не рекомендуется)
var (
	onceWrong sync.Once
	initError error
)

func initResource() error {
	return fmt.Errorf("неудачная инициализация")
}

func getResource() (interface{}, error) {
	onceWrong.Do(func() {
		// ошибка не может быть возвращена, сохраняем в переменную
		initError = initResource()
	})
	return nil, initError
}

func primerOn4() {
	_, err := getResource()
	fmt.Printf("primer4: ошибка инициализации (плохой паттерн): %v\n", err)
}

// sync.Cond

/*
Зачем нужен Cond, если есть каналы?
Каналы хороши для простого сигнала «данные готовы». Но если условие сложнее
(например: «очередь не пуста И размер очереди больше 10 ИЛИ пришёл сигнал
завершения»), то код на каналах становится громоздким. Cond позволяет ждать
выполнения произвольного условия и эффективно будить одну или все горутины при
изменении общего состояния.

Как работает:

* Cond содержит внутренний sync.Locker (обычно Mutex или RWMutex).
* Wait() — атомарно отпускает блокировку и усыпляет горутину; при пробуждении
снова захватывает блокировку.
* Signal() — пробуждает одну ждущую горутину (если есть).
* Broadcast() — пробуждает все горутины.

Важные правила:

* Wait() всегда должен вызываться внутри цикла, проверяющего условие:
go
for !condition() {
    cond.Wait()
}
Потому что пробуждение может быть ложным (spurious wakeup) или условие могло
измениться другой горутиной до того, как эта получила блокировку.

* Блокировка должна быть захвачена перед вызовом Wait(), Signal(), Broadcast().

* Signal() разбудит одну горутину (если есть). Broadcast() — все.

Когда использовать:

* Очередь задач с несколькими потребителями (ждут, пока очередь не станет непустой).
* Ожидание изменения сложного состояния (готовность нескольких ресурсов,
загрузка конфигурации).
* Когда нужно разбудить все горутины одновременно (Broadcast).

Когда НЕ использовать:

* Простая синхронизация производитель-потребитель с каналом (канал проще).
* Ожидание одного сигнала от одной горутины — используйте chan struct{} + close().
*/

// 1. базовая очередь (один потребитель)
type queue struct {
	cond  *sync.Cond
	items []int
}

func newQueue() *queue {
	return &queue{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (q *queue) push(item int) {
	defer q.cond.L.Unlock()
	q.cond.L.Lock()
	q.items = append(q.items, item)
	q.cond.Signal() // будим одного ждущего
}

func (q *queue) pop() int {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	for len(q.items) == 0 {
		q.cond.Wait()
	}
	item := q.items[0]
	q.items = q.items[1:]
	return item
}

func primerCon1() {
	q := newQueue()
	go func() {
		time.Sleep(1 * time.Second)
		q.push(42)
	}()
	fmt.Println("ожидание...")
	fmt.Println("получено:", q.pop())
}

// 2. несколько потребителей и Broadcast при закрытии
type broadcastQueue struct {
	cond   *sync.Cond
	items  []int
	closed bool
}

func newBroadcastQueue() *broadcastQueue {
	return &broadcastQueue{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (q *broadcastQueue) push(item int) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if q.closed {
		return
	}
	q.items = append(q.items, item)
	q.cond.Signal()
}

func (q *broadcastQueue) pop() (int, bool) {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	if len(q.items) == 0 && !q.closed {
		q.cond.Wait()
	}

	if q.closed && len(q.items) == 0 {
		return 0, false
	}

	item := q.items[0]
	q.items = q.items[1:]
	return item, true
}

func (q *broadcastQueue) close() {
	q.cond.L.Lock()
	defer q.cond.L.Unlock()
	q.closed = true
	q.cond.Broadcast()
}

func primerCon2() {
	q := newBroadcastQueue()
	var wg sync.WaitGroup

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for {
				val, ok := q.pop()
				if !ok {
					fmt.Printf("consumer %d: closed\n", id)
					return
				}
				fmt.Printf("consumer %d got %d\n", id, val)
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		q.push(i)
		time.Sleep(100 * time.Millisecond)
	}
	q.close()
	wg.Wait()
}

// 3. Broadcast как стартовый пистолет (одновременный старт)

type starter struct {
	cond  *sync.Cond
	ready bool
}

func newStarter() *starter {
	return &starter{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (s *starter) waitStart() {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	for !s.ready {
		s.cond.Wait()
	}
}

func (s *starter) start() {
	s.cond.L.Lock()
	defer s.cond.L.Unlock()
	s.ready = true
	s.cond.Broadcast()
}

func primerCon3() {
	s := newStarter()
	for i := 0; i < 3; i++ {
		go func(id int) {
			fmt.Printf("worker %d waiting\n", id)
			s.waitStart()
			fmt.Printf("worker %d started\n", id)
		}(i)
	}

	time.Sleep(2 * time.Second)
	fmt.Println("START!")
	s.start()
	time.Sleep(1 * time.Second)
}

//sync.pool

/*
Зачем нужен Pool, если можно просто создавать объекты?
Создание новых объектов — это аллокация памяти, которая нагружает сборщик мусора
(GC). Если вы часто создаёте и уничтожаете одинаковые временные объекты (буферы,
слайсы, структуры), GC может работать слишком интенсивно. sync.Pool переиспользует
объекты: вы берёте объект из пула, работаете с ним, возвращаете обратно. GC время от
времени чистит пул, но в целом количество аллокаций и объём работы GC снижаются.

Как работает:

* Pool управляет набором временных объектов, которые могут быть удалены GC в
любой момент.
* Get() — возвращает объект из пула (или создаёт новый через функцию New, если
пул пуст или был очищен GC).
* Put(x) — возвращает объект обратно в пул для переиспользования.

Важные правила:

* Объекты в пуле не должны хранить критически важные данные (например,
соединения с БД, файловые дескрипторы), потому что GC может удалить их неожиданно.
* Перед возвратом объекта в пул нужно сбросить его состояние (очистить буфер, обнулить поля), чтобы следующий пользователь не получил «грязные» данные.
* Функция New должна быть потокобезопасной и неблокирующей (обычно это просто конструктор).
* Размер пула неограничен, но GC может периодически очищать его (обычно при каждом цикле GC).
* sync.Pool можно копировать (в отличие от Mutex), но обычно используют один глобальный пул.

Когда использовать:

* Частое создание временных буферов (bytes.Buffer, []byte).
* JSON/Protobuf/XML сериализация/десериализация (например, json.Encoder).
* Сложные структуры, которые дорого создавать (большие слайсы, мапы).
* Логгеры, пакеты обработки строк (strings.Builder).

Когда НЕ использовать:

Объекты, которые не имеют чёткого момента освобождения (слишком долго живут).
* Если стоимость создания объекта мала (например, маленькая структура с парой полей).
* Если объекты имеют ссылки на другие ресурсы, которые нужно закрывать.
*/

// 1. базовое использование пула для байтового буфера
var bufferPool = sync.Pool{
	New: func() any {
		return bytes.NewBuffer(make([]byte, 0, 4096))
	},
}

func primerPool1() {
	//берём буфер из пула
	buf := bufferPool.Get().(bytes.Buffer)
	defer bufferPool.Put(buf) // обязательно возвращаем

	buf.Reset() // очищаем состояние
	buf.WriteString("hello people")
	fmt.Println(buf.String())
}

// 2. пул для больших слайсов (снижение аллокаций)
var slicePool = sync.Pool{
	New: func() any {
		return make([]int, 0, 1024)
	},
}

func processInts(data []int) {
	slice := slicePool.Get().([]int)
	defer slicePool.Put(slice)

	slice = slice[0:] // очищаем, сохраняя copacity
	for _, v := range data {
		slice = append(slice, v*2)
	}
	//работаем в slice
	_ = slice
}

func primerPool2() {
	// без пула каждая итерация аллоцировала бы новый слайс
	for i := 0; i < 100; i++ {
		processInts([]int{1, 2, 3, 4, 5})
	}
	fmt.Println("primer2: done, аллокации снижены")
}

// 3. пул для сложной структуры (экономия на GC)
type bigStructPool struct {
	id     int
	data   [1024]byte
	buffer *bytes.Buffer
}

var bigPool = sync.Pool{
	New: func() interface{} {
		return &bigStructPool{
			buffer: bytes.NewBuffer(make([]byte, 0, 2048)),
		}
	},
}

func getBigStruct() *bigStructPool {
	return bigPool.Get().(*bigStructPool)
}

func putBigStruct(b *bigStructPool) {
	b.id = 0
	b.buffer.Reset()
	bigPool.Put(b)
}

func primerPool3() {
	b := getBigStruct()
	defer putBigStruct(b)

	b.id = 42
	b.buffer.WriteString("hello")
	fmt.Printf("id: %d, buf: %s\n", b.id, b.buffer.String())
}

// 4. предупреждение – не хранить критические ресурсы

type dbConn struct {
	conn *sql.DB
}

var connPool = sync.Pool{
	New: func() interface{} {
		return &dbConn{conn: createDBConn()}
	},
}

func primer4() {
	// ПЛОХО: соединение БД может быть удалено GC в любой момент
	// Когда это произойдёт, программа потеряет соединение.
	// Пулы не предназначены для хранения долгоживущих ресурсов.
	c := connPool.Get().(*dbConn)
	defer connPool.Put(c)

	// ... использование c.conn ...
	fmt.Println("primer4: так делать нельзя для БД/файлов/сокетов")
}
