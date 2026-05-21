package _testing_testify

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// 1. ОСНОВЫ ТЕСТИРОВАНИЯ И ПАКЕТ TESTING
/*
В этом блоке мы закладываем фундамент. В Go тестирование — это не сторонний инструмент,
а часть стандартной поставки. Мы разберем, как писать идиоматичные тесты и
как управлять их запуском из консоли.

ТЕОРЕТИЧЕСКАЯ БАЗА:
1. Зачем это нужно:
   - Проверка контрактов: код делает то, что обещал.
   - Рефакторинг: тесты — это страховка, что твои правки ничего не сломали.
   - Дизайн: если код сложно протестировать, значит у него "кривая" архитектура.

2. Механика запуска:
   Когда ты пишешь `go test`, рантайм ищет файлы `*_test.go`, компилирует их
   вместе с кодом и запускает функции вида `func TestXxx(t *testing.T)`.

3. Жизненный цикл теста:
   Go запускает каждый тест в отдельной горутине. Если тест падает, остальные
   продолжают выполняться (если не вызван Fatal).
*/

func sum(a, b int) int {
	return a + b
}

func divide(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("division by zero")
	}
	return a / b, nil
}

// ПРИМЕР 1: Базовая тестовая функция
/* - Название: Test + Имя (с заглавной буквы).
   - Аргумент: только *testing.T.
*/
func testAddSimple(t *testing.T) {
	res := sum(2, 3)
	expected := 5

	if res != expected {
		// Errorf: логирует ошибку, помечает тест как Fail, но НЕ прерывает функцию.
		t.Errorf("Add(2, 3) = %d; want %d", res, expected)
	}
}

// ПРИМЕР 2: Table-Driven Tests (Золотой стандарт)
/* Вместо кучи функций пишем одну таблицу. Это позволяет легко
   проверять пограничные случаи (edge cases) в одном цикле.
*/
func TestAddTableDriven(t *testing.T) {
	tests := []struct {
		name     string
		a, b     int
		expected int
	}{
		{"positive", 2, 2, 4},
		{"negative", -1, -5, -6},
		{"zero sum", 5, -5, 0},
	}

	for _, tt := range tests {
		// t.Run создает изолированный подтест.
		t.Run(tt.name, func(t *testing.T) {
			res := sum(tt.a, tt.b)
			if res != tt.expected {
				t.Errorf("%s: got %d, want %d", tt.name, res, tt.expected)
			}
		})
	}
}

// ПРИМЕР 3: Тестирование ошибок (Error Cases)
/* В Go мы обязаны проверять не только "счастливый путь", но и ошибки.
 */
func testDivide(t *testing.T) {
	t.Run("zero division error", func(t *testing.T) {
		_, err := divide(10, 0)
		if err == nil {
			t.Fatal("ожидали ошибку, но получили nil") // Fatal прерывает выполнение
		}

		expectedErr := "division by zero"
		if err.Error() != expectedErr {
			t.Errorf("got error %q, want %q", err.Error(), expectedErr)
		}
	})
}

// ПРИМЕР 4: Helper-функции и t.Helper()
/* t.Helper() сообщает рантайму, что при ошибке нужно показывать строку
   кода, где ВЫЗВАЛИ хелпер, а не саму строку ВНУТРИ хелпера.
*/
func assertString(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func testStringHelper(t *testing.T) {
	res := strings.ToUpper("go")
	assertString(t, res, "GO")
}

// ПРИМЕР 5: Пропуск тяжелых тестов (Skip)
/* Если тест идет долго (интеграция, БД), его можно пропустить флагом -short.
   Запуск: go test -v -short
*/
func testLongRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропускаем тяжелый тест в режиме -short")
	}
	// Тут логика на 10 секунд...
}

// ПРИМЕР 6: Параллельные тесты (t.Parallel)
/* Позволяет гонять тесты одновременно, ускоряя общий прогон.
   ВНИМАНИЕ: не используй с общими переменными (race condition!).
*/
func testParallel(t *testing.T) {
	t.Parallel() // Этот тест будет запущен параллельно с другими
	res := sum(10, 10)
	if res != 20 {
		t.Error("Wrong sum")
	}
}

// ПРИМЕР 7: Команды запуска (CLI Cheat Sheet)
/*
1. `go test .` — запуск тестов в текущей папке.
2. `go test -v` — подробный вывод (Verbose).
3. `go test -run=TestAdd` — запуск тестов по маске (регулярке).
4. `go test -cover` — базовый отчет по покрытию кода.
5. `go test -count=1` — запуск БЕЗ КЭША (важно при дебаге).
*/

// 2. ЮНИТ-ТЕСТЫ, ИНТЕГРАЦИЯ И TESTIFY
/*
В этом блоке мы выходим за рамки простых проверок. На реальных проектах
стандартного пакета `testing` часто не хватает для лаконичности, поэтому
мы подключаем `testify`. Также важно разделять тесты по их назначению.

ТЕОРЕТИЧЕСКАЯ БАЗА ДЛЯ СОБЕСА:
1. Unit-тесты (Юнит-тесты):
   - Проверяют минимальную единицу кода (функцию, метод) в изоляции.
   - Не ходят в базу, не лезут в сеть, не читают файлы.
   - Имитируют зависимости (Mocking). Должны работать мгновенно.

2. Integration-тесты (Интеграционные):
   - Проверяют связку нескольких компонентов (например, сервис + база данных).
   - Ходят в реальные ресурсы (часто в Docker-контейнеры через Testcontainers).
   - Выявляют ошибки в SQL-запросах или интеграции с внешними API.

3. Пакет Testify/Assert:
   - Это стандарт индустрии. Дает "синтаксический сахар".
   - Выдает понятный Diff при ошибке (сразу видно, что в слайсе не так).
   - Сокращает код: `if got != want { t.Error(...) }` превращается в одну строку.
*/

// Логика
type user struct {
	id        int
	email     string
	isActive  bool
	createdAt time.Time
}

// UserService — пример типичного сервиса с зависимостями
type UserRepository interface {
	Save(ctx context.Context, u *user) error
}

type userService struct {
	repo UserRepository
}

func (s *userService) register(ctx context.Context, email string) (*user, error) {

	user := &user{email: email}

	// Контекст пробрасывается вглубь для контроля таймаутов в БД/API
	if err := s.repo.Save(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}

	return user, nil
}

func (s *userService) createUser(email string) (*user, error) {
	if email == "" {
		return nil, errors.New("empty email")
	}
	// Имитация логики создания
	return &user{
		id:        101,
		email:     email,
		isActive:  true,
		createdAt: time.Now(),
	}, nil
}

// ПРИМЕР 1: Полноценный Unit-тест сервиса через Testify
/* Здесь мы проверяем не только факт отсутствия ошибки, но и глубокое
   соответствие полей структуры.
*/
func testUserService_CreateUser_Success(t *testing.T) {
	service := &userService{}
	email := "bygor.kent@mail.ru"

	user, err := service.createUser(email)

	// assert.NoError — база, проверяем что сервис не упал
	assert.NoError(t, err)

	// require.NotNil — критично: если юзер nil, дальше тестить поля нет смысла (паника)
	require.NotNil(t, user)

	// Глубокая проверка состояния
	assert.Equal(t, 101, user.id)
	assert.Equal(t, email, user.email)
	assert.True(t, user.isActive)

	// Проверка времени: создано в пределах последней секунды
	assert.WithinDuration(t, time.Now(), user.createdAt, time.Second)
}

// ПРИМЕР 2: Тестирование граничных условий (Edge Cases)
// Проверяем, как сервис реагирует на заведомо плохие данные.
func testUserService_CreateUser_Failures(t *testing.T) {
	service := &userService{}

	tests := []struct {
		name    string
		email   string
		wantErr string
	}{
		{"empty email string", "", "empty email"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := service.createUser(tt.email)

			assert.Nil(t, user)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// ПРИМЕР 3: Сложная проверка коллекций (Слайсы и Мапы)
// Testify позволяет сравнивать сложные объекты без написания циклов.
func testUserRoles_ComplexMatch(t *testing.T) {
	actualRoles := []string{"admin", "editor", "viewer"}
	expectedRoles := []string{"viewer", "admin", "editor"}

	// ElementsMatch проверяет, что элементы те же самые, игнорируя порядок.
	// Обычный Equal тут бы упал.
	assert.ElementsMatch(t, actualRoles, expectedRoles, "Набор ролей должен совпадать")
}

// ПРИМЕР 4: Асинхронные проверки (Eventually)
/* Представь, что сервис шлет уведомление в фоновом режиме.
   Нам нужно дождаться изменения статуса, не блокируя поток навсегда.
*/
func еestAsyncWork_Eventually(t *testing.T) {
	type task1 struct{ status string }
	task := &task1{status: "new"}

	go func() {
		time.Sleep(50 * time.Millisecond)
		task.status = "ready"
	}()

	// Ждем 'ready' в течение 200мс, проверяем каждые 10мс
	assert.Eventually(t, func() bool {
		return task.status == "ready"
	}, 200*time.Millisecond, 10*time.Millisecond, "Статус должен был измениться на ready")
}

// ПРИМЕР 5: Интеграционный тест (Имитация работы с Context)
// Тут контекст РЕАЛЬНО нужен, чтобы проверить, как код реагирует на отмену.
func testIntegration_WithContextCancellation(t *testing.T) {
	if testing.Short() {
		t.Skip("Пропуск тяжёлого теста")
	}

	service := &userService{repo: &slowRepo{}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := service.register(ctx, "test@test.com")

	// Проверяем, что ошибка пробросилась именно от контекста
	assert.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled) || strings.Contains(err.Error(), "canceled"))
}

// ВСПОМОГАТЕЛЬНЫЕ СТРУКТУРЫ
type mockRepo struct{}

func (m *mockRepo) Save(ctx context.Context, u *user) error { return nil }

type slowRepo struct{}

func (m *slowRepo) Save(ctx context.Context, u *user) error {
	select {
	case <-time.After(1 * time.Second):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// 3. ОРГАНИЗАЦИЯ ТЕСТОВ, MOCKING И TESTIFY SUITE
/*
На хорошем уровне недостаточно просто писать тесты — нужно уметь изолировать
код от внешнего мира (БД, API, очереди) и правильно структурировать проект.
В этом блоке разберем, как делать "честный" Unit через интерфейсы и как
управлять состоянием через Suite.

ТЕОРЕТИЧЕСКАЯ БАЗА:
1. Организация пакетов:
   - Внутренние тесты (package mypkg): тестируют логику "изнутри", видят приватные поля.
   - Внешние тесты (package mypkg_test): тестируют публичный API. Это лучший подход,
     так как тесты не ломаются при рефакторинге внутренних названий.

2. Mocks vs Fakes:
   - Mock: объект, который проверяет сам факт вызова метода, аргументы и количество вызовов.
   - Fake: упрощенная рабочая реализация (например, БД в памяти).

3. Testify Suite:
   - Позволяет группировать тесты в структуру (ООП-стиль).
   - Дает методы Setup/Teardown для подготовки окружения (поднять БД, почистить таблицы).
*/

// ПОДОПЫТНЫЙ КОД
type users struct {
	id   int
	name string
}

type userRepo interface {
	getByID(ctx context.Context, id int) (*users, error)
	save(ctx context.Context, u *users) error
}

type services struct {
	repo userRepo
}

func (s *services) getUserGreeting(ctx context.Context, id int) (string, error) {
	u, err := s.repo.getByID(ctx, id)
	if err != nil {
		return "", nil
	}
	return "Hello" + u.name, nil
}

// ПРИМЕР 1: Mocking через testify/mock
// Создаем структуру-заглушку. Это "честный" мок, так как он проверяет вызовы.
type mockUserRepo struct {
	mock.Mock
}

func (m *mockUserRepo) getByID(ctx context.Context, id int) (*users, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*users), args.Error(1)
}

func (m *mockUserRepo) save(ctx context.Context, user *users) error {
	return m.Called(ctx, user).Error(0)
}

func testService_GetUserGreeting_Mock(t *testing.T) {
	mRepo := new(mockUserRepo)
	svc := &services{repo: mRepo}
	ctx := context.Background()

	mRepo.On("getByID", ctx, 1).Return(&users{
		id:   1,
		name: "Gopher",
	}, nil)
	mRepo.On("getByID", ctx, 99).Return(nil, errors.New("not found"))

	// Кейс 1: Успех
	msg, err := svc.getUserGreeting(ctx, 1)
	assert.NoError(t, err)
	assert.Equal(t, "Hello, Gopher", msg)

	// Кейс 2: Ошибка
	_, err = svc.getUserGreeting(ctx, 99)
	assert.Error(t, err)

	// ПРОВЕРКА: Были ли вызваны методы именно так, как мы ожидали
	mRepo.AssertExpectations(t)
}

// ПРИМЕР 2: Testify Suite
// Удобно для интеграционных тестов, где нужно общее состояние.
type userIntegrationSuite struct {
	suite.Suite
	fakeDB map[int]*users // Имитируем тестовую БД
}

// setupTest запускается ПЕРЕД КАЖДЫМ тестом в Suite
func (s *userIntegrationSuite) setupTest() {
	s.fakeDB = make(map[int]*users)
	s.fakeDB[1] = &users{
		id:   1,
		name: "Alex",
	}
}
func (s *userIntegrationSuite) testGetExistingUser() {
	user := s.fakeDB[1]
	s.Equal("Alex", user.name)
}

func (s *userIntegrationSuite) testAddUser() {
	s.fakeDB[2] = &users{
		id:   2,
		name: "Bob",
	}
	s.Len(s.fakeDB, 2)
}

func testUserIntegrationSuite(t *testing.T) {
	suite.Run(t, new(userIntegrationSuite))
}

// --- ПРИМЕР 3: Тестирование с фейковым HTTP-сервером ---
// Когда нужно проверить интеграцию с внешним API, не делая реальных запросов.

func testExternalAPI_Integration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	}))
	defer server.Close()

	// Делаем запрос на URL этого сервера
	resp, err := http.Get(server.URL)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// ПРИМЕР 5: Тестирование Middleware (httptest.NewRecorder)
// Проверяем, как наш обработчик пишет заголовки и тело ответа.
func testAuthMiddleware(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})
	req := httptest.NewRequest("GET", "http://example.com/foo", nil)
	w := httptest.NewRecorder() // "Записыватель" ответа

	handler.ServeHTTP(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	assert.Equal(t, "ok", string(body))
}

// ПРИМЕР 6: Golden Files (Тестирование больших выходных данных)
/* Если функция возвращает огромный JSON, его неудобно хранить в коде теста.
   Мы сравниваем результат с файлом на диске.
*/
func testLargeOutput(t *testing.T) {
	actual := "{\"long\": \"json\", \"complex\": \"structure\"}" // Представим, что это результат функции

	// Читаем эталон из файла
	expected, err := os.ReadFile("testdata/output.json.golden")
	require.NoError(t, err)

	assert.JSONEq(t, string(expected), actual)
}

// 4. ЭКСПЕРТНОЕ ТЕСТИРОВАНИЕ: TDD, CI/CD И ПРОФИЛИРОВАНИЕ
/*
На экспертном уровне тестирование перестает быть "проверкой после написания кода"
и становится инструментом дизайна архитектуры. Здесь мы внедряем тесты в
процесс поставки (CI/CD) и учимся искать узкие места через профилирование.

ТЕОРЕТИЧЕСКАЯ БАЗА:
1. TDD (Test-Driven Development):
   - Цикл: Red (тест падает) -> Green (код написан) -> Refactor (код причесан).
   - Это не про поиск багов, а про проектирование интерфейсов "от потребителя".

2. BDD (Behavior-Driven Development):
   - Описание поведения через сценарии (Given/When/Then).
   - Позволяет связать технические тесты с требованиями бизнеса.

3. Профилирование и Бенчмарки:
   - Benchmark-тесты позволяют замерить скорость (ns/op) и аллокации (B/op).
   - Профилировщик pprof помогает найти, какая функция "жрет" CPU или память.

4. CI/CD и Quality Gates:
   - Автоматизация запуска тестов при каждом пуше.
   - Использование race detector и линтеров как обязательное условие деплоя.
*/

// ПОДОПЫТНЫЙ КОД
type paymentProcessor interface {
	pay(amount int) error
}
type orderService struct {
	processor paymentProcessor
}

// ПРИМЕР 1: TDD подход (Сначала тест — потом реализация)
/* Допустим, нам нужно реализовать метод Checkout.
   Мы сначала описываем ожидания в тесте.
*/
func testOrderService_Checkout_TDD(t *testing.T) {
	mockPay := new(mockProcessor)
	svc := &orderService{processor: mockPay}

	// Given: Ожидаем вызов оплаты на 100 рублей
	mockPay.On("pay", 100).Return(nil).Once()

	// When: Вызываем чекаут
	err := svc.Checkout(100)

	// Then: Ошибок быть не должно
	assert.NoError(t, err)
	mockPay.AssertExpectations(t)
}

// ВСПОМОГАТЕЛЬНЫЕ МЕТОДЫ
type mockProcessor struct{ mock.Mock }

func (m *mockProcessor) pay(amount int) error { return m.Called(amount).Error(0) }

// Сама реализация метода (написана ПОСЛЕ теста)
func (s *orderService) Checkout(amount int) error {
	return s.processor.pay(amount)
}

// ПРИМЕР 2: BDD-сценарии через t.Run
// Структурируем тест так, чтобы он читался как описание процесса.
func testUserRegistration_BDD(t *testing.T) {
	t.Run("Given: A new user with valid email", func(t *testing.T) {
		//email := "expert@go.dev"

		t.Run("When: User attempts to register", func(t *testing.T) {
			// Тут логика вызова...
			isSuccess := true

			t.Run("Then: Account should be created and active", func(t *testing.T) {
				assert.True(t, isSuccess)
			})
		})
	})
}

// ПРИМЕР 3: Профилирование производительности (Benchmark)
/* Сравним два способа конкатенации строк.
   Запуск: go test -bench=. -benchmem
*/
func benchmarkStringConcat(b *testing.B) {
	b.Run("plus operator", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = "Hello" + "Word"
		}
	})

	b.Run("fmt. Sprintf", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = fmt.Sprintf("%s %s", "Hello", "Word")
		}
	})
}

// ПРИМЕР 4: Продвинутый Mocking (Mocking с побочными эффектами)
// Используем .Run(), чтобы имитировать изменение данных моком.
func testMockWithSideEffect(t *testing.T) {
	m := new(mockProcessor)

	// При вызове Pay мы хотим выполнить дополнительный код внутри мока
	m.On("Pay", mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		amount := args.Get(0).(int)
		fmt.Printf("Logging payment of %d\n", amount)
	})

	_ = m.pay(100)
	m.AssertExpectations(t)
}

// ПРИМЕР 5: Тестирование времени выполнения (Timeout Pattern)
// Проверка, что операция не виснет и укладывается в дедлайн.
func testFastOperations(t *testing.T) {
	done := make(chan bool)
	go func() {
		// Какая-то работа...
		done <- true
	}()

	select {
	case <-done:
		// Успех
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Тест упал по таймауту")
	}
}
