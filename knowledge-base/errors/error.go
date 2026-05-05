package errors

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"sync"

	"golang.org/x/sync/errgroup"
)

//1. ЧТО ТАКОЕ ОШИБКИ
/*
ЧТО ЭТО:
- Ошибка — это значение, которое указывает на проблему в выполнении операции.
- В Go ошибки представляются через интерфейс `error`.
- Это основной механизм обработки ошибок в языке.

ПОЧЕМУ ЭТО ВАЖНО:
- Позволяет писать надежный и предсказуемый код.
- Избегает паник в непредвиденных ситуациях.
- Реализует принцип "ошибки — это нормально".

ОСНОВНЫЕ ОПЕРАЦИИ:
- Создание: errors.New("message"), fmt.Errorf("format", ...)
- Обработка: if err != nil { ... }
- Возвращение: return nil, err

ПОДВОДНЫЕ КАМНИ В ДАННОМ БЛОКЕ:
- Нельзя игнорировать ошибки — это приводит к неожиданному поведению
- nil означает отсутствие ошибки
- Нельзя сравнивать ошибки через == в некоторых случаях (см. продвинутый уровень)
*/

func basicErr() {
	err := errors.New("error db")
	fmt.Println(err)

	if err != nil {
		fmt.Println("error:", err)
	}
}

func readFile(fileName string) ([]byte, error) {
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, fmt.Errorf("file reading error %s: %w", fileName, err)
	}
	return data, nil
}

//2. СОЗДАНИЕ И ОБРАБОТКА ОШИБОК (WRAPPING)
/*
ТЕОРЕТИЧЕСКАЯ БАЗА (Для собеса):
- error в Go — это интерфейс, а не класс или исключение.
- Любой тип, у которого есть метод Error() string, автоматически считается ошибкой.
- Нулевое значение для интерфейса error — это nil. Если err == nil, значит операции прошла успешно.

Механика Wrapping (Оборачивания):
   - Появилась в Go 1.13.
   - Позволяет строить "стек" ошибок.
   - %w под капотом создает структуру, у которой есть метод Unwrap() error.

МЕХАНИЗМ ВОЗВРАТА:
- В Go принято возвращать ошибку последним аргументом.
- Мы не "выбрасываем" (throw) ошибки, а передаем их как обычные значения.

Когда использовать %w vs %v:
   - %w (Wrap): когда внешнему коду НУЖНО знать первопричину (например, "это была ошибка таймаута?").
   - %v (Value): когда мы хотим просто добавить текст, но СКРЫТЬ детали реализации (инкапсуляция ошибок).
*/

// 2.1. Использование интерфейса error и создание через fmt.Errorf
func processFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		// Создаем новую ошибку, добавляя контекст (имя файла),
		// чтобы в логах было понятно, где именно упало.
		return fmt.Errorf("failed to open config at %s: %v", path, err)
	}
	defer f.Close()
	return nil
}

// 2.2. Обработка сетевых запросов (базовый случай)
func fetchStatus(url string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("network call failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return nil
}

// 2.3. ПЕРЕДАЧА ОШИБОК МЕЖДУ ГОРУТИНАМИ
/*
Проблема: горутина не может просто вернуть error в main.
Решение: используем каналы для доставки ошибок.
*/

func concurrentWorker(urls []string) error {
	errChan := make(chan error, len(urls))
	var wg sync.WaitGroup

	for _, url := range urls {
		wg.Add(1)
		go func(u string) {
			defer wg.Done()
			if u == "" {
				errChan <- errors.New("empty url")
				return
			}
			errChan <- nil
		}(url)
	}

	// Ждем в отдельной горутине, чтобы не заблокировать чтение
	go func() {
		wg.Wait()
		close(errChan) // Закрываем, когда все закончили
	}()

	// Читаем все ошибки
	for err := range errChan {
		if err != nil {
			return err // Все еще возвращаем первую, но остальные не зависнут (благодаря буферу или закрытию)
		}
	}
	return nil
}

// 3. ПРОДВИНУТАЯ ПРОВЕРКА И АНАЛИЗ (IS, AS, WRAPPING)
/*
ТЕОРЕТИЧЕСКИЙ ГЛУБОКИЙ РАЗБОР:

1. Эволюция и pkg/errors:
   - До Go 1.13 стандартная библиотека не умела "оборачивать" ошибки.
   - Все использовали пакет github.com/pkg/errors (функции Wrap, Cause).
   - В Go 1.13 концепцию добавили в стандарт (errors.Is/As и %w), но pkg/errors
     до сих пор любят за Stack Trace (трассировку стека), которой нет в стандарте.

2. Механика Sentinel Errors (Ошибки-маркеры):
   - Это глобальные переменные. Проблема в том, что их можно изменить из другого пакета
     (т.к. это переменные). Поэтому их стоит проверять ТОЛЬКО через errors.Is.

3. Кастомные типы:
   - Создаются, когда нужно передать не просто текст, а структуру данных.
   - Обязательно должны реализовывать метод Error() string.
   - Чтобы кастомная ошибка поддерживала цепочку (wrapping), она должна иметь метод Unwrap().
*/

// 3.1. SENTINEL ERRORS И errors.Is
var errInternal = errors.New("internal server error")

func databaseCall() error {
	return fmt.Errorf("db failure: %w", errInternal)
}

func demErrIs() {
	err := databaseCall()

	if errors.Is(err, errInternal) {
		fmt.Println("Is: Это внутренняя ошибка, можно показать 500 пользователю")
	}

	if errors.Is(err, io.EOF) {
		fmt.Println("Is: Данные закончились")
	}
}

//3.2. CUSTOM ERRORS И errors.As

type networkError struct {
	Attempt int
	URL     string
}

func (e *networkError) Error() string {
	return fmt.Sprintf("failed after %d attempts to %s", e.Attempt, e.URL)
}

func callService() error {
	return fmt.Errorf("api layer: %w", &networkError{Attempt: 3, URL: "localhost:8080"})
}

func demAs() {
	err := callService()

	var netErr *networkError
	// errors.As проверяет, есть ли в цепочке тип *NetworkError.
	// Если да — копирует её в переменную netErr.
	if errors.As(err, &netErr) {
		fmt.Printf("As: Ошибка на %d попытке. URL: %s\n", netErr.Attempt, netErr.URL)
	}
}

// 3.3. МЕХАНИКА UNWRAP
func demUnwrap() {
	base := errors.New("base error")
	wrapped := fmt.Errorf("wrapped :%w", base)

	// errors.Unwrap достает следующий уровень.
	// Если уровней много, нужно вызывать в цикле или использовать errors.Is/As

	inner := errors.Unwrap(wrapped)
	fmt.Println("Unwrap:", inner)
}

//3.3. ПРИМЕР: МНОГОУРОВНЕВЫЙ WRAPPING И UNWRAP

func step1_Base() error          { return errors.New("root_error") }
func step2_Wrap(err error) error { return fmt.Errorf("level_2: %w", err) }
func step3_Wrap(err error) error { return fmt.Errorf("level_3: %w", err) }

func demDeepUnwrap() {
	err := step3_Wrap(step2_Wrap(step1_Base()))

	u1 := errors.Unwrap(err)
	u2 := errors.Unwrap(u1)
	u3 := errors.Unwrap(u2)

	fmt.Println(u3)
}

// --- 3.4. РАБОТА С pkg/errors (Исторический контекст) ---
// В старых проектах или там, где нужен StackTrace, используют github.com/pkg/errors
func legacyStyle() {
	// cause := errors.New("root cause")

	// Вместо fmt.Errorf используется Wrap
	// err := pkgErrors.Wrap(cause, "additional info")

	// Вместо errors.Unwrap используется Cause
	// root := pkgErrors.Cause(err)
}

// 3.5. ОБЪЕДИНЕНИЕ ОШИБОК (Go 1.20+)
func collectErrors() error {
	var errs error
	errs = errors.Join(errs, errors.New("ошибка 1"))
	errs = errors.Join(errs, errors.New("ошибка 2"))

	// errors.Is и errors.As работают для всех ошибок в наборе
	return errs
}

// 4. АРХИТЕКТУРА И КОНТРОЛЬ ОШИБОК
/*
ГЛУБОКАЯ ТЕОРИЯ:
1. Panic vs Error: Паника прерывает нормальное выполнение горутины и запускает
   цепочку defer-вызовов. Используется только для "невозможных" состояний.
2. Recover — используется в middleware или верхних слоях, чтобы упавшая горутина
   не "сложила" всё приложение.
3. Смерть приложения: Паника в любой горутине убивает ВСЁ приложение, если
   внутри этой горутины нет своего recover. Это критично для надежности.
4. Проектирование Domain Errors: На верхних уровнях (API/UI) мы не должны
   показывать SQL-запросы. Мы мапим внутренние ошибки в "бизнес-ошибки".
5. Библиотеки: Использование github.com/pkg/errors оправдано там, где нужен
   Stack Trace для отладки в продакшене.
*/

// 4.1. СЛОЖНЫЙ RECOVER И СТЕКТРЕЙС
// Используется в Middleware для веб-серверов.
func recoveryMiddleware(next func()) {
	defer func() {
		if r := recover(); r != nil {
			// Получаем стектрейс, чтобы понять, ГДЕ упало
			stack := debug.Stack()
			log.Printf("Критический сбой: %v\n%s", r, stack)
			// Здесь можно отправить отчет в Sentry или другую систему мониторинга
		}
	}()
	next()
}

// 4.2. ТЕХНИКА OPAQUE ERRORS (Непрозрачные ошибки)
// Вместо проверки типа, мы проверяем поведение (Behaviour).
type temporary interface {
	temporary() bool
}

func isTemporary(err error) bool {
	var t temporary
	if errors.As(err, &t) {
		return t.temporary()
	}
	return false
}

// 4.3. ЦЕНТРАЛИЗОВАННЫЙ МАППИНГ ОШИБОК
type appError struct {
	Code    string // "USER_NOT_FOUND"
	Message string // "Пользователь не существует"
	Status  int    // 404
	Cause   error  // Исходная ошибка
}

func (e *appError) Error() string { return fmt.Sprintf("[%s] %s", e.Code, e.Message) }
func (e *appError) Unwrap() error { return e.Cause }

// Пример "умного" обработчика для логов и клиента
func handleError(err error) {
	var appErr *appError
	if errors.As(err, &appErr) {
		// Для логов пишем всё
		log.Printf("API Error: %s, Internal: %v", appErr.Code, appErr.Cause)
		// Для клиента отдаем только Status и Message
	}
}

// 4.4. ERRGROUP: СИНХРОНИЗАЦИЯ ОШИБОК В ГОРУТИНАХ ---
// Позволяет запустить пачку задач и получить ПЕРВУЮ ошибку, отменив остальные.
func fetchAllReliably(ctx context.Context, urls []string) error {
	g, ctx := errgroup.WithContext(ctx)

	for _, url := range urls {
		u := url // capture variable
		g.Go(func() error {
			// Если один запрос упадет, контекст отменится для всех остальных
			return fetchUrl(ctx, u)
		})
	}
	return g.Wait() // Вернет первую ошибку из всех горутин
}

// 4.5. ГЛАВНАЯ ЛОВУШКА: Интерфейс не nil, если тип не nil
func returnCustomNil() error {
	var err *appError = nil // У нас есть типизированный nil-указатель
	return err              // ОШИБКА: возвращаем его как интерфейс error
}

func demonstrateNilTrap() {
	err := returnCustomNil()

	// ВНИМАНИЕ: Это условие выполнится!
	if err != nil {
		fmt.Printf("Ловушка! Тип: %T, Значение: %v\n", err, err)
		// Выведет: Ловушка! Тип: *errors.AppError, Значение: <nil>
	}
}

// ПРАВИЛЬНЫЙ СПОСОБ:
func returnProperNil() error {
	var err *appError = nil
	if err == nil {
		return nil // Возвращаем чистый nil (где и тип nil, и значение nil)
	}
	return err
}

// 5. BEST PRACTICES & GOTCHAS (ДЛЯ SENIOR ИНТЕРВЬЮ)
/*
 1: Log OR Return
- Зачем: Избегаем дублирования логов (spamming).
- Правило: Ошибку обрабатывает тот, кто её получил последним.

2: Indent Error Flow (Happy Path)
- Зачем: Читаемость кода. Основная логика не должна "тонуть" в if/else.
- Правило: Ошибки обрабатываются сверху вниз с немедленным return.

3: Wrap with Context (%w)
- Зачем: Понимание цепочки событий (StackTrace для бедных).
- Правило: Добавляй имя функции или действия в fmt.Errorf.

4: Behavior Assertions
- Зачем: Уменьшение связности (Decoupling) между пакетами.
- Правило: Проверяй методы (интерфейсы), а не конкретные типы.
*/

// 5.1. ПРИМЕР: Log OR Return ---
// Правило: Либо логируй, либо возвращай. Не делай оба действия сразу.
func logOrReturn(fileName string) error {
	data, err := os.ReadFile(fileName)
	if err != nil {
		// ПЛОХО: log.Printf("error: %v", err); return err
		// ХОРОШО: Пробрасываем выше. Залогирует тот, кто вызвал (на границе системы).
		return fmt.Errorf("read data: %w", err)
	}
	_ = data
	return nil
}

// 5.2. ПРИМЕР: Indent Error Flow (Happy Path)
// Правило: Основная логика (успех) идет по левому краю, ошибки — в отступах.
func happyPathExample(name string) (int, error) {
	if name == "" {
		return 0, errors.New("name is required") // Быстрый возврат при ошибке
	}

	user, err := findUserInDB(name)
	if err != nil {
		return 0, fmt.Errorf("find user: %w", err) // Ошибка "отстрелена"
	}

	// Основная логика (Happy Path) не забита в else
	return user.Age, nil
}

// 5.3. ПРИМЕР: Wrap with Context (%w)
// Правило: Добавляй контекст (название функции/действия), чтобы понимать путь ошибки.
func wrapContext() error {
	err := errors.New("not file")
	if err != nil {
		// %w позволяет сохранить исходную ошибку для errors.Is/As[cite: 1]
		return fmt.Errorf("wrapContext: failed to execute task: %w", err)
	}
	return nil
}

// 5.4. ПРИМЕР: Behavior Assertions (Проверка поведения)
// Правило: Вместо проверки конкретного типа, проверяй наличие нужного метода.
type retryable interface {
	CanRetry() bool
}

func handleNetwork(err error) {
	var r retryable
	// Проверяем поведение через интерфейс (декаплинг от конкретных пакетов)
	if errors.As(err, &r) && r.CanRetry() {
		fmt.Println("Повторяем операцию...")
	}
}
