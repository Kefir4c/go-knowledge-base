package context

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// 1. ЧТО ТАКОЕ CONTEXT
/*
ЧТО ЭТО (Простыми словами):
- Это объект, который передается по всей цепочке вызовов (от хендлера до базы данных).
- Он сообщает функциям: "У тебя еще есть время работать" или "Всё, бросай дела, клиент ушел".

ЗАЧЕМ ЭТО НУЖНО:
1. Отмена (Cancellation): Остановить работу, если она больше не нужна.
2. Таймауты (Timeouts): Не ждать вечно ответа от тормозящего API.
3. Передача метаданных: RequestID, Auth Token (то, что не является бизнес-логикой).

ПРАВИЛА:
- Всегда первым аргументом: func Do(ctx context.Context, ...).
- context.Background(): Корень всего. Используется в main или в тестах
- context.TODO(): Заглушка, когда не знаешь, какой контекст воткнуть (но планируешь разобраться).
*/

// 1.1. ПРИМЕР: ПЕРЕДАЧА ЗНАЧЕНИЙ (WithValue)
// Используется для передачи "сквозных" данных (RequestID, JWT, TraceID).
func valueExample() {
	type key string
	const myKey key = "session_id"

	ctx := context.WithValue(context.Background(), myKey, "secret-token-123")
	checkAuth(ctx)
}

func checkAuth(ctx context.Context) {
	if val, ok := ctx.Value("session_id").(string); !ok {
		fmt.Println("Найдена сессия:", val)
	}
}

// 1.2. ПРИМЕР: ПРОВЕРКА СОСТОЯНИЯ (Done)
// Как понять внутри функции, что пора закругляться.
func worker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done(): // Канал закроется, когда контекст отменят
			fmt.Println("Воркер: Получил сигнал остановки, выхожу...")
			return
		default:
			fmt.Println("Воркер: Выполняю мелкую задачу...")
			time.Sleep(300 * time.Millisecond)
		}
	}
}

// 1.3. ПРИМЕР: ПЕРЕДАЧА КОНТЕКСТА В ЦЕПОЧКЕ
// Демонстрация того, как контекст прокидывается "сквозь" функции.
func serviceLayer(ctx context.Context) {
	fmt.Println("Service: Начало работы")
	dbLayer(ctx)
}

func dbLayer(ctx context.Context) {
	// Проверяем, не отменили ли нас еще на уровне сервиса
	select {
	case <-ctx.Done():
		return
	default:
		fmt.Println("DB: Выполняю запрос к базе...")
	}
}

// 1.4. ПРИМЕР: ИСПОЛЬЗОВАНИЕ TODO
// Когда ты знаешь, что ctx нужен, но вызывающая функция его еще не принимает.
func futureFeature() {
	// Временная заглушка, чтобы код компилировался
	ctx := context.TODO()
	serviceLayer(ctx)
}

// 2. УПРАВЛЕНИЕ СРОКАМИ И ОТМЕНОЙ
/*
ТЕОРЕТИЧЕСКАЯ БАЗА:
- Иерархия (Tree Structure): Контексты образуют дерево. Когда вы отменяете родительский
  контекст, все дочерние контексты, созданные от него, отменяются автоматически.
- Распространение отмены (Propagation): Это критично в микросервисах. Если пользователь
  отменил запрос в браузере, сигнал об отмене должен пройти через все сервисы (API -> Auth -> DB).
- Утечки ресурсов (Context Leaks): Если вы создали контекст с таймаутом и не вызвали
  функцию cancel(), таймер будет висеть в памяти до завершения времени, даже если работа закончена.
*/

// 2.1. ПРИМЕР: РУЧНАЯ ОТМЕНА (WithCancel)
// Позволяет мгновенно прекратить работу горутины по сигналу.
func demManualCancel() {
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		// Имитируем условие, при котором нужно всё остановить (например, нажали кнопку "Стоп")
		time.Sleep(1 * time.Second)
		fmt.Println("Main: Сигнал на отмену!")
		cancel() // Закрывает канал ctx.Done()
	}()

	// Передаем контекст в долгоиграющую функцию
	performLongTask(ctx)
}

func performLongTask(ctx context.Context) {
	for i := 1; i < 10; i++ {
		select {
		case <-ctx.Done():
			// Обязательно проверяем контекст, чтобы выйти из горутины
			fmt.Printf("Task: Прекращаю работу на шаге %d: %v\n", i, ctx.Err())
			return
		default:
			fmt.Printf("Task: Выполняю шаг %d...\n", i)
			time.Sleep(500 * time.Millisecond)
		}
	}
}

// 2.2. ПРИМЕР: ЖЕСТКИЕ ТАЙМАУТЫ (WithTimeout)
// Идеально для запросов к внешним API или базе данных.
func demTimeuot() {
	// Устанавливаем лимит: задача должна выполниться за 2 секунды
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

	// Чистим ресурсы (таймер) сразу после выхода из функции
	defer cancel()

	resultChan := make(chan string)

	go func() {
		time.Sleep(3 * time.Second) // Имитируем, что задача тормозит
		resultChan <- "Данные из БД"
	}()

	select {
	case res := <-resultChan:
		fmt.Println("Успех:", res)
	case <-ctx.Done():
		// Выведет: context deadline exceeded
		fmt.Println("Ошибка: работа заняла слишком много времени:", ctx.Err())
	}
}

// 2.3. ПРИМЕР: РАБОТА В ЦЕПОЧКЕ МИКРОСЕРВИСОВ (net/http)
// Демонстрируем, как контекст прокидывается от входящего запроса к исходящему.
func demMicroserviceHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Берем контекст из входящего запроса (он уже содержит таймаут от клиента)
	ctx := r.Context()

	// 2. Создаем исходящий запрос к другому микросервису (например, к базе данных)
	// Важно: передаем ctx дальше, чтобы отмена клиента долетела до БД
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://db-service:8080/user", nil)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		// Если клиент отменил запрос, пока мы ждали БД, Do(req) вернет ошибку
		fmt.Println("Запрос прерван:", err)
		return
	}
	defer resp.Body.Close()

	io.Copy(w, resp.Body)
}

// 2.4. ПРИМЕР: ДЕДЛАЙН (WithDeadline)
// В отличие от таймаута (длительность), дедлайн — это конкретный момент времени
func demDeadline() {
	// Устанавливаем время "смерти" контекста: через 5 секунд от текущего момента
	deadline := time.Now().Add(5 * time.Second)

	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	fmt.Println("Работаем до:", deadline.Format("15:04:05"))
	<-ctx.Done()
	fmt.Println("Время вышло!")
}

// 2.5. ПРИМЕР: ИЕРАРХИЯ И НАСЛЕДОВАНИЕ ОТМЕНЫ
func demHierarchyExample() {
	parentCtx, parentCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer parentCancel()

	childCtx, childCancel := context.WithTimeout(parentCtx, 10*time.Second)
	defer childCancel()

	start := time.Now()
	<-childCtx.Done()

	fmt.Printf("Дочерний контекст завершился через %.0f сек. Причина: %v\n",
		time.Since(start).Seconds(), childCtx.Err())
}

// 2.6. ПРАКТИКА: ПРОВЕРКА ПЕРЕД НАЧАЛОМ
func heavyWork(ctx context.Context) error {
	// Проверяем статус сразу: если клиент уже ушел, выходим
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("работа не начата: %w", err)
	}

	// Далее идет какая-то логика...
	return nil
}

// 3.СЛОЖНЫЕ ЦЕПОЧКИ И АСИНХРОННОСТЬ
/*
ТЕОРЕТИЧЕСКАЯ БАЗА:
1. Изоляция жизненного цикла: В микросервисах часто возникает задача —
   выполнить что-то в фоне после того, как основной запрос завершен.
   Передача "умирающего" контекста в горутину — критическая ошибка.
2. Линейный поиск Value: Контекст — это связный список (parent -> child).
   Поиск ключа идет от ребенка к родителю. Сложность O(N).
   Если в цепочке 100 слоев, поиск Value станет "бутылочным горлышком".
3. Безопасность типов: Поскольку ctx.Value работает с interface{},
   разработчик всегда должен инкапсулировать логику работы с данными
   внутри пакета (get/set функции).
*/

// 3.1. ПРИМЕР: ПРАВИЛЬНАЯ АСИНХРОННОСТЬ (Go 1.21+)
// Проблема: Клиент ждет ответа 200 OK, но нам нужно отправить метрики в фоне.
func demProcessOrderAsync(ctx context.Context) {
	// Делаем важную часть
	fmt.Println("Заказ оформлен!")

	// Плохой пример: go sendMetrics(ctx)
	// как только эта функция завершится, ctx будет отменён, и метрики не уйдут.

	// Как надо: Создаем контекст-клон, который не отменяется, но сохраняет данные (Value).
	detachedCtx := context.WithoutCancel(ctx)
	go func() {
		// sendMetrics(detachedCtx)
	}()
}

// 3.2. ПРИМЕР: WITHCANCELCAUSE (Уточнение причины отмены - Go 1.20+)
// Теперь можно передать конкретную ошибку в cancel, чтобы понять, ПОЧЕМУ всё упало.
func demCriticalOperation() {
	ctx, cancel := context.WithCancelCause(context.Background())

	go func() {
		err := errors.New("база знаний перегружена")
		cancel(err)
	}()
	<-ctx.Done()

	if err := context.Cause(ctx); err != nil {
		fmt.Printf("Работа остановлена по причине: %v\n", err)
	}
}

// 3.3. ПРИМЕР: ИНКАПСУЛЯЦИЯ ЗНАЧЕНИЙ (Best Practice)
// Не заставляем пользователя знать ключи контекста. Пакет сам управляет своими данными.

type keyID int

const userKey keyID = 1

func demContextWithUserID(parent context.Context, id int) context.Context {
	return context.WithValue(parent, userKey, id)
}
func demUserIDFromContext(ctx context.Context) (int, bool) {
	id, ok := ctx.Value(userKey).(int)
	return id, ok
}

// 3.4. ПРИМЕР: AFTERFUNC (Go 1.21+)
// Позволяет зарегистрировать функцию, которая выполнится СРАЗУ при отмене контекста.
// Удобно для очистки ресурсов, которые не поддерживают контекст нативно.
func demCleanupWithAfterFunc(ctx context.Context) {
	stop := context.AfterFunc(ctx, func() {
		fmt.Println("Контекст отменён, закрываем соединения")
	})
	// Если работа завершилась успешно раньше отмены, AfterFunc нужно остановить
	defer stop()
	time.Sleep(2 * time.Second)
}

// 3.5. ПРИМЕР: СЛОЖНАЯ ОБРАБОТКА ОШИБОК (Is/As)
func demAdvancedErrorCheck(ctx context.Context) {
	err := errors.New("рандомная ошибка")

	switch {
	case errors.Is(err, context.DeadlineExceeded):
		log.Println("Таймаут: сервис не ответил вовремя")
	case errors.Is(err, context.Canceled):
		log.Println("Отмена: пользователь нажал 'Назад' или закрыл вкладку")
	case err != nil:
		log.Printf("Другая ошибка: %v\n", err)
	}
}

// 3.6. ПРИМЕР: СУЖЕНИЕ ТАЙМАУТА (Inheritance)

func demDynamicTimeout() {
	// Родитель дает 5 секунд на всё
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Дочерняя функция даёт 2 секунды
	_, childCancel := context.WithTimeout(ctx, 2*time.Second)
	defer childCancel()

	// Если родитель отменится через 1 сек, ребенок тоже умрет.
	// Если ребенок не успеет за 2 сек — он умрет сам, но родитель будет жить до 5 сек.
}

// 3.7. ПРИМЕР: ГАРАНТИЯ ПРОВЕРКИ КОНТЕКСТА
func demSafeOperation(ctx context.Context, ch <-chan int) {
	for {
		select {
		case <-ctx.Done():
			return
		case v := <-ch:
			// ВАЖНО: Если данные и отмена пришли одновременно, select выберет рандомно.
			// Для критичных задач разработчик делает еще одну проверку:
			select {
			case <-ctx.Done():
				return
			default:
			}
			fmt.Println("Обработка данных:", v)
		}
	}
}

// 4. АРХИТЕКТУРА И ВЫСОКИЕ НАГРУЗКИ
/*
ТЕОРЕТИЧЕСКАЯ БАЗА:
1. ПАТТЕРН PROPAGATION (РАСПРОСТРАНЕНИЕ):
   В распределенных системах контекст — это "паспорт" запроса.
   если сервис А вызывает сервис Б по gRPC или HTTP, TraceID и Deadline
   должны передаваться в заголовках, а на той стороне — восстанавливаться обратно в ctx.
2. ПРОБЛЕМА ТЯЖЕЛЫХ ЦИКЛОВ:
   Если функция делает тяжелые вычисления в цикле (например, обработка 1 млн записей),
   простая проверка в начале функции не поможет. Контекст нужно проверять ВНУТРИ цикла,
   чтобы остановить вычисления сразу, как только клиент отменил запрос.
3. ПРЕДСКАЗУЕМОЕ ТЕСТИРОВАНИЕ:
   Разработчик не использует time.Sleep в тестах. Он использует каналы и сигнализацию Done(),
   чтобы тесты проходили за миллисекунды, а не ждали реальные таймауты.
4. ПРЕДЕЛЫ ИСПОЛЬЗОВАНИЯ Value:
   Нужно понимать, что каждое context.WithValue создает новый объект в куче (heap).
   В высоконагруженных системах (100k+ RPS) бездумное навешивание контекстов
   может привести к деградации производительности из-за работы Garbage Collector (GC).
*/

// 4.1. ПРИМЕР: ПРОВЕРКА КОНТЕКСТА В ТЯЖЕЛЫХ ВЫЧИСЛЕНИЯХ
// Задача: обработать огромный массив данных, не тратя ресурсы зря.
func demHeavyComputation(ctx context.Context, data []int) error {
	for i, v := range data {
		// Каждые N итераций проверяем контекст, не отменили ли нас
		// (Проверка на каждой итерации может быть дорогой, делаем раз в 1000)
		if i%1000 == 0 {
			select {
			case <-ctx.Done():
				fmt.Printf("Вычисления прерваны на индексе %d\n", i)
				return ctx.Err()
			default:
				// Продолжаем работу
			}
		}
		// Имитация сложного расчета
		_ = v * v
	}
	return nil
}

// 4.2. ПРИМЕР: ТЕСТИРОВАНИЕ АСИНХРОННОГО КОДА (Determenistic Testing)
// Как написать тест, который не зависит от удачи и скорости процессора.
func demTestWorkerContext(t interface{ Errorf(string, ...any) }) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		// Какая-то работа, которая слушает ctx
		worker(ctx)
		close(done)
	}()
	cancel()

	select {
	case <-done:
		// Успех: воркер отреагировал на отмену
	case <-time.After(200 * time.Millisecond):
		// Провал: воркер завис или проигнорировал ctx
		t.Errorf("worker did not stop on context cancellation")
	}
}

// 4.3. ПРИМЕР: РЕАЛИЗАЦИЯ СОБСТВЕННОГО КОНТЕКСТА (Custom Context)
// Используется крайне редко, например, в обертках над старыми фреймворками.
type customContext struct {
	context.Context // Встраиваем стандартный контекст
}

// Переопределяем метод Done, чтобы он никогда не сигналил об отмене
func (cc customContext) Done() <-chan struct{} {
	return nil
}

func (cc customContext) Err() error {
	return nil
}

// 4.4. ПАТТЕРН: ДИНАМИЧЕСКИЙ ТАЙМАУТ В ЦЕПОЧКЕ (Context Budget)
// мы выделяем общий "бюджет" времени на цепочку вызовов.
func demHandleWorkflow(ctx context.Context) {
	// На всю цепочку (DB + API) даем 5 секунд
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Вызов БД - если БД съест 4.9 сек, на API останется всего 0.1 сек.
	// Это защищает систему от накопления "хвостов" запросов.
	_ = callDatabase(ctx)
	_ = callExternalAPI(ctx)
}

// 5. BEST PRACTICES & ANTI-PATTERNS
/*
Тут собраны правила, которые помогаю при написания кода.
*/

// 5.1. ПРАВИЛО: КОНТЕКСТ НЕ ХРАНИТСЯ В СТРУКТУРАХ
// АНТИПАТТЕРН:
type BadService struct {
	ctx context.Context // НИКОГДА ТАК НЕ ДЕЛАЙ
}

// Почему? Потому что контекст имеет жизненный цикл ЗАПРОСА,
// а структура сервиса живет всё время работы приложения.
// Это ломает саму идею дерева контекстов.

// ИСКЛЮЧЕНИЕ: стандартная библиотека (http.Request) или системные обертки,
// но в бизнес-логике — это табу.

// 5.2. ПРАВИЛО: ЯВНОЕ ПЕРЕДАВАНИЕ (Explicit over Implicit)
// Контекст всегда идет ПЕРВЫМ аргументом.
func GoodFunc(ctx context.Context, data string) {} // OK
func BadFunc(data string, ctx context.Context)  {} // Ноу ноу

// 6.3. ПРАВИЛО: ТИПИЗАЦИЯ КЛЮЧЕЙ
// Мы это уже писали, но закрепим: используй непубличные типы,
// чтобы никто снаружи не мог перезатереть твои данные в ctx.
type privateKey int

const requestIDKey privateKey = 0

// 6.4. ПАТТЕРН: DOUBLE CHECK SELECT
// Нюанс для высоконагруженных систем.
func demDoubleSelect(ctx context.Context, ch <-chan int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// Если контекст не закрыт, идем в основной select
			select {
			case <-ctx.Done():
				return
			case v := <-ch:
				fmt.Println(v)
			}
		}
	}
}

// Зачем? В обычном select, если и канал готов, и ctx.Done() закрыт,
// Go выберет рандомно. Double check гарантирует, что мы не начнем
// обработку данных, если контекст уже "всё".

// 6.5. ПРАВИЛО: НЕ ПЕРЕДАВАЙ NIL
// Если функция просит контекст, а у тебя его нет — передай context.TODO().
// nil context вызовет панику во многих стандартных методах.
