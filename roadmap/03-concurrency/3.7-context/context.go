package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

/*
ЧТО ТАКОЕ CONTEXT.CONTEXT?

Context — это интерфейс, который несёт через весь стек вызовов:
  1. СИГНАЛ ОТМЕНЫ (cancel signal)   — когда пора закругляться
  2. ТАЙМАУТ / ДЕДЛАЙН              — deadline/timeout для операции
  3. КОНТЕКСТНЫЕ ЗНАЧЕНИЯ (request-scoped) — только для данных запроса

Создавать контекст нужно в самом верху (в main, в хендлере) и передавать
вниз по цепочке вызовов (явно, первым параметром).

ЗАЧЕМ НУЖЕН CONTEXT?

БЕЗ контекста:
  - Невозможно отменить долгую операцию (зависший запрос)
  - Нет единого таймаута на всю цепочку вызовов
  - Нет способа остановить горутины при завершении приложения

С контекстом:
  - Вызов API / БД / файловой системы можно прервать
  - Горутины получают сигнал "пора выходить"
  - Единый таймаут для всей операции (от HTTP до БД)

ОСНОВНЫЕ ФУНКЦИИ СОЗДАНИЯ КОНТЕКСТА

1. context.Background()
   - Корневой контекст для main, тестов, инициализации
   - НЕЛЬЗЯ отменить
   - НЕТ таймаута

2. context.TODO()
   - Для временного кода (пока не решишь, какой контекст передать)
   - Сигнал: "я знаю, что нужен контекст, но пока не знаю какой"

3. context.WithCancel(parent)
   - Возвращает: ctx (копия родителя) и cancel()
   - Вызов cancel() отменяет ctx и всех детей

4. context.WithTimeout(parent, duration)
   - Автоматическая отмена через duration

5. context.WithDeadline(parent, time)
   - Автоматическая отмена в конкретный момент

6. context.WithValue(parent, key, value)
   - Передача request-scoped данных

СИГНАЛЫ ОТМЕНЫ КОНТЕКСТА

Отмена происходит, когда:
  1. Вызвали cancel() (ручная отмена)
  2. Сработал таймаут / дедлайн
  3. Родительский контекст был отменён

Проверка отмены в коде:
  select {
  case <-ctx.Done():
      err := ctx.Err() // context.Canceled или context.DeadlineExceeded
      return err
  default:
      // продолжаем работу
  }

Важно: ctx.Done() возвращает канал. Когда канал закрыт — контекст отменён.

ПРАВИЛА ПЕРЕДАЧИ КОНТЕКСТА

1. Контекст всегда первый параметр функции (по соглашению)
   func DoSomething(ctx context.Context, arg int) error

2. НЕ храните контекст в структурах (долго). Контекст живёт один запрос.

3. НЕ передавайте nil вместо контекста. Используйте context.TODO().

4. Отменённый контекст НЕЛЬЗЯ "востановить". Создавайте новый.

5. cancel() должна вызываться в той же горутине, которая создала контекст.
   Обязательно с defer, даже если не ожидаете ошибок:
   ctx, cancel := context.WithTimeout(parent, timeout)
   defer cancel()

CONTEXT.WITHVALUE — ПЕРЕДАЧА ДАННЫХ (ТОЛЬКО REQUEST-SCOPED!)

Создание:
  type keyType string  // свой тип ключа (чтобы избежать коллизий)
  const userIDKey keyType = "userID"

  ctx := context.WithValue(parent, userIDKey, "alice@example.com")

Чтение:
  userID := ctx.Value(userIDKey).(string) // нужен type assertion

   Что НЕЛЬЗЯ класть в WithValue:
   - Параметры функций (они должны быть явными аргументами)
   - Данные, которые могут меняться во время жизни запроса (не потокобезопасно)
   - Логгеры, конфиги, соединения с БД (это dependency injection)

Что МОЖНО (request-scoped data):
   - ID пользователя (аутентификация)
   - Correlation ID / Trace ID (для трейсинга)
   - Deadline (уже есть в ctx)
   - Флаги фичей (A/B тестирование)

РАСПРОСТРАНЕНИЕ ЧЕРЕЗ СТЕК ВЫЗОВОВ

Контекст должен проходить через весь стек вызовов ЯВНО:

  HTTP Handler → Service → Repository → Database

Нельзя хранить контекст в структуре и ждать, что он сам обновится.

Правильно:
  func (s *Service) GetUser(ctx context.Context, id int) (*User, error) {
      return s.repo.GetUser(ctx, id)
  }

Неправильно (контекст в структуре):
  type Service struct {
      ctx context.Context  //  не делайте так
  }

ОШИБКИ КОНТЕКСТА

ctx.Err() возвращает:
  - nil — если контекст ещё не отменён
  - context.Canceled — если вызвали cancel()
  - context.DeadlineExceeded — если истёк таймаут/дедлайн

Проверка:
  if err := ctx.Err(); err != nil {
      if err == context.Canceled {
          // ручная отмена
      } else if err == context.DeadlineExceeded {
          // таймаут
      }
  }

КОГДА НУЖЕН CONTEXT

ВСЕГДА, когда операция может:
  - Зависнуть (сетевой вызов, БД, файловая система)
  - Длиться долго (API запрос, сложное вычисление)
  - Быть отменена пользователем (закрыл браузер, нажал "стоп")

Где НЕ нужен:
  - Простые внутренние функции (parse, format)
  - Инициализация (но тогда используй Background)
  - Утилиты без I/O
*/

// 1. ОСНОВЫ — WITHCANCEL И РУЧНАЯ ОТМЕНА
func worker(ctx context.Context, id int) {
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("worker %d: отмена! (%v)\n", id, ctx.Err())
			return
		default:
			fmt.Printf("worker %d: работаю...\n", id)
			time.Sleep(200 * time.Millisecond)
		}
	}
}
func primer1() {
	ctx, cancel := context.WithCancel(context.Background())

	for i := 0; i < 3; i++ {
		go worker(ctx, i)
	}

	time.Sleep(1 * time.Second)
	fmt.Println("Главная: отменяю всех!")
	cancel()

	time.Sleep(500 * time.Millisecond)
	fmt.Println("primer1 - завершён")
}

// 2. TIMEOUT — АВТОМАТИЧЕСКАЯ ОТМЕНА
func slowOper(ctx context.Context) error {
	select {
	case <-time.After(2 * time.Second):
		return fmt.Errorf("операция завершена")
	case <-ctx.Done():
		return fmt.Errorf("операция отменена: %w", ctx.Err())
	}
}
func primer2() {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	err := slowOper(ctx)
	fmt.Printf("primer2 - результат: %v\n", err)
}

// 3. WITHDEADLINE — ОТМЕНА В КОНКРЕТНОЕ ВРЕМЯ
func primer3() {
	deadline := time.Now().Add(500 * time.Millisecond)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("primer3 - дедлайн в %v, сейчас %v, ошибка: %v\n",
				deadline, time.Now(), ctx.Err())
			return
		default:
			fmt.Println("жду дедлайна...")
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// 4. WITHVALUE — ПЕРЕДАЧА REQUEST-SCOPED ДАННЫХ
type ctxKey string

const (
	reqIDKey  ctxKey = "reqID"
	userIDKey ctxKey = "userID"
)

func handler4(ctx context.Context) {
	reqID := ctx.Value(reqIDKey).(string)
	userID := ctx.Value(userIDKey).(int)
	fmt.Printf("primer4 - обработка запроса %s от пользователя %d\n", reqID, userID)
	service4(ctx)
}

func service4(ctx context.Context) {
	reqID := ctx.Value(reqIDKey).(string)
	fmt.Printf("primer4 - сервис: requestID=%s\n", reqID)
}

func primer4() {
	ctx := context.Background()
	ctx = context.WithValue(ctx, reqIDKey, "req-123123")
	ctx = context.WithValue(ctx, userIDKey, 1234)
	handler4(ctx)
}

// 5. ОТМЕНА ГРУППЫ ГОРУТИН (ПОЧЕМУ НЕЛЬЗЯ ДЕЛАТЬ ВСЕМ РАЗНЫЙ КОНТЕКСТ)
func fanOutWorker(ctx context.Context, id int, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("fanOutWorker %d: отмена\n", id)
			return
		default:
			fmt.Printf("fanOutWorker %d: работа\n", id)
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func primer5() {
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go fanOutWorker(ctx, i, &wg)
	}

	time.Sleep(1 * time.Second)
	fmt.Println("primer5 - отменяю ВСЕХ одной кнопкой")
	cancel()

	wg.Wait()
	fmt.Println("primer5 - все воркеры завершены")

}

// 6. ТИПИЧНЫЙ HTTP-ХЕНДЛЕР С CONTEXT (ИМИТАЦИЯ)
type db struct{}

func (db) getUser(ctx context.Context, id int) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-time.After(100 * time.Millisecond):
		return "Alice", nil
	}
}

type service struct {
	db db
}

func (s *service) getUser(ctx context.Context, id int) (string, error) {
	return s.db.getUser(ctx, id)
}

type handler struct {
	svc *service
}

func (h *handler) handle(ctx context.Context, id int) {
	// устанавливаем таймаут на всю обработку запроса
	ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer cancel()

	user, err := h.svc.getUser(ctx, id)
	if err != nil {
		fmt.Printf("primer6 - ошибка: %v\n", err)
		return
	}
	fmt.Printf("primer6 - пользователь: %s\n", user)
}

func primer6() {
	h := &handler{svc: &service{}}
	h.handle(context.Background(), 1)
}

// 7. ОШИБКИ — ЧТО НЕЛЬЗЯ ДЕЛАТЬ С КОНТЕКСТОМ
// ОШИБКА 1: хранение контекста в структуре
type badService struct {
	ctx context.Context // не делайте так
}

// ОШИБКА 2: передача контекста в горутину без синхронизации (забыли <-ctx.Done)
func badWorker(ctx context.Context) {
	// бесконечный цикл без проверки отмены
	for {
		fmt.Println("badWorker: работаю...")
		time.Sleep(1 * time.Second)
	}
}

// ОШИБКА 3: игнорирование отмены (просто забыли select)
func badOperation(ctx context.Context) error {
	time.Sleep(2 * time.Second) // не реагирует на ctx.Done()
	return nil
}

// ОШИБКА 4: отмена не в той горутине (cancel вызывается до defer)
func badCancel() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel() // вызывается в дочерней горутине — не ошибка, но антипаттерн
	}()
	<-ctx.Done()
}

func primer7() {
	fmt.Println("primer7 - примеры ошибок (закомментированы, раскомментируй для проверки):")
	// badWorker(context.Background())
	// badOperation(context.Background())
	// badCancel()
	fmt.Println("  см. комментарии в коде")
}

// 8. ЦЕПОЧКА ОТМЕН (ОТМЕНА РЕБЁНКА НЕ ВЛИЯЕТ НА РОДИТЕЛЯ)
func primer8() {
	parent, parentCancel := context.WithCancel(context.Background())
	child, childCancel := context.WithCancel(parent)

	// отслеживаем состояние родителя и ребёнка
	go func() {
		<-parent.Done()
		fmt.Println("  родитель отменён")
	}()
	go func() {
		<-child.Done()
		fmt.Println("  ребёнок отменён")
	}()
	fmt.Println("primer8 - отменяем ребёнка:")
	childCancel()
	time.Sleep(100 * time.Millisecond)

	fmt.Println("primer8 - отменяем родителя:")
	parentCancel()
	time.Sleep(100 * time.Millisecond)

	fmt.Println("  ребёнок отменился, родитель остался жив; родитель отменился сам")
}

//func main() {
//	primer1()
//	primer2()
//	primer3()
//	primer4()
//	primer5()
//	primer6()
//	primer7()
//	primer8()
//}
