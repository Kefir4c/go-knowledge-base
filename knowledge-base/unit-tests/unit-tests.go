package unit_tests

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

/*
БЛОК 1: ОСНОВЫ UNIT-ТЕСТИРОВАНИЯ

1.1 ЧТО ТАКОЕ UNIT-ТЕСТ?

Unit-тест — это автоматическая проверка МИНИМАЛЬНОЙ единицы кода (обычно функции)
в ИЗОЛЯЦИИ от внешних зависимостей.
ПРИЗНАКИ ХОРОШЕГО UNIT-ТЕСТА:
   - Быстрый (миллисекунды)
   - Изолированный (не зависит от БД, сети, файлов)
   - Детерминированный (всегда выдает одинаковый результат)
   - Самодостаточный (не требует ручной настройки)
ЭТО НЕ UNIT-ТЕСТЫ (это интеграционные):
   - Тест, который ходит в реальную БД
   - Тест, который делает HTTP-запрос к внешнему API
   - Тест, который читает/пишет файлы на диске

1.2 ИМЕНОВАНИЕ ТЕСТОВЫХ ФАЙЛОВ И ФУНКЦИЙ

ПРАВИЛА:
   1. Файл с тестами: *_test.go (например, math_test.go)
   2. Функция теста: TestXxx(t *testing.T)
   3. Xxx — любое имя с большой буквы (обычно имя тестируемой функции)
РАСПОЛОЖЕНИЕ:
   math.go       <- тестируемый код
   math_test.go  <- тесты (в той же папке)

1.3 ПАРАМЕТР *testing.T И ЕГО МЕТОДЫ

МЕТОДЫ ОБЪЕКТА t (в порядке частоты использования):
t.Errorf(format, args...)   // Лог ошибки + пометка FAIL, НО продолжает выполнение
t.Fatalf(format, args...)   // Лог ошибки + НЕМЕДЛЕННЫЙ выход из теста
t.Logf(format, args...)     // Информационное сообщение (видно с -v)
t.Skipf(format, args...)    // Пропуск теста (используй с testing.Short())
t.Parallel()                // Запуск параллельно с другими тестами
t.Helper()                  // Маркер хелпер-функции (ошибка укажет на место вызова)
ПРАВИЛО БОЛЬШОГО ПАЛЬЦА:
   - Используй Errorf, если проверка НЕ критична для дальнейших проверок
   - Используй Fatalf, если без значения дальше тест не имеет смысла (nil указатель)
*/

// ТЕСТИРУЕМЫЙ КОД (обычно лежит в отдельном файле, но для примера здесь)

func add(a, b int) int {
	return a + b
}

func divide(a, b int) (int, error) {
	if b == 0 {
		return 0, errDivisionByZero
	}
	return a / b, nil
}

// ErrDivisionByZero — стандартная ошибка
var errDivisionByZero = &divisionError{}

type divisionError struct{}

func (e *divisionError) Error() string {
	return "division by zero"
}

/*
ПРИМЕР 1: ПРОСТЕЙШИЙ ТЕСТ
Запуск: go test -v
Что проверяем: Add(2, 3) должно вернуть 5
*/
func testAddSimple(t *testing.T) {
	result := add(2, 3)
	expected := 5

	if result != expected {
		// Errorf — тест продолжит выполнение (полезно для нескольких проверок)
		t.Errorf("Add(2,3) = %d; want %d", result, expected)
	}
}

/*
ПРИМЕР 2: TABLE-DRIVEN TEST (ЗОЛОТОЙ СТАНДАРТ)
Запуск: go test -run=TestAddTableDriven
Что это: Один тест с таблицей кейсов. Подтесты изолированы через t.Run
*/
func testAddTableDriven(t *testing.T) {
	// Таблица тестовых кейсов
	tests := []struct {
		name     string
		a, b     int
		expected int
	}{
		{name: "positive numbers", a: 2, b: 3, expected: 5},
		{name: "negative numbers", a: -1, b: -5, expected: -6},
		{name: "with zero", a: 5, b: 0, expected: 5},
		{name: "mixed signs", a: -3, b: 7, expected: 4},
	}

	for _, tt := range tests {
		// t.Run создает подтест — отличная читаемость и изоляция
		t.Run(tt.name, func(t *testing.T) {
			result := add(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("%s: Add(%d,%d) = %d; want %d",
					tt.name, tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

/*
ПРИМЕР 3: ТЕСТИРОВАНИЕ ОШИБОК
Запуск: go test -run=TestDivideErrors
Важно: проверяем не только успешные, но и "пограничные" случаи
*/
func testDivideErrors(t *testing.T) {
	tests := []struct {
		name       string
		a, b       int
		wantResult int
		wantErr    bool
	}{
		{name: "normal division", a: 10, b: 2, wantResult: 5, wantErr: false},
		{name: "division by zero", a: 10, b: 0, wantResult: 0, wantErr: true},
		{name: "zero divided", a: 0, b: 5, wantResult: 0, wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := divide(tt.a, tt.b)

			if tt.wantErr {
				// Ожидаем ошибку — проверяем, что она есть
				if err == nil {
					t.Fatalf("ожидали ошибку, но получили nil") // Fatalf — дальше нет смысла проверять
				}
				// Дополнительно можно проверить текст ошибки
				if err.Error() != "division by zero" {
					t.Errorf("неправильная ошибка: %v", err)
				}
			} else {
				// Не ожидаем ошибку — проверяем, что её нет
				if err != nil {
					t.Fatalf("не ожидали ошибку, получили %v", err)
				}
				if result != tt.wantResult {
					t.Errorf("Divide(%d,%d) = %d; want %d", tt.a, tt.b, result, tt.wantResult)
				}
			}
		})
	}
}

/*
ПРИМЕР 4: t.HELPER() — ПОМЕЧАЕМ ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ
Без t.Helper() при ошибке будет указана строка ВНУТРИ assertString.
С t.Helper() — строка, где ВЫЗВАЛИ assertString.
*/

// assertString — кастомная проверка строк
func assertString(t *testing.T, got, want string) {
	t.Helper() // <-- КЛЮЧЕВОЙ МОМЕНТ: указываем, что это хелпер
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func testWithHelper(t *testing.T) {
	result := add(1, 1) // 2
	// Если assertString упадёт, ошибка укажет на ЭТУ строку (а не внутрь хелпера)
	assertString(t, string(result), "2")
	// assertString(t, result, "3") // раскомментируй — ошибка покажет строку выше
}

/*
ПРИМЕР 5: ПРОПУСК ТЯЖЕЛЫХ ТЕСТОВ (testing.Short)
Запуск с пропуском: go test -short -v
Запуск всех тестов: go test -v
Используется для разделения быстрых юнит-тестов и медленных интеграционных
*/
func testHeavyIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("пропускаем тяжелый тест в режиме -short")
	}
	// Здесь могла быть проверка работы с реальной БД или API
	t.Log("Полноценный интеграционный тест...")
}

/*
ПРИМЕР 6: ПАРАЛЛЕЛЬНЫЕ ТЕСТЫ (t.Parallel)
Запуск: go test -v -parallel=4
ВНИМАНИЕ! Используй только когда тесты НЕ используют общие переменные
*/
func testParallelA(t *testing.T) {
	t.Parallel()
	// имитация работы
	_ = add(1, 1)
	t.Log("TestParallelA завершен")
}

func testParallelB(t *testing.T) {
	t.Parallel()
	_ = add(2, 2)
	t.Log("TestParallelB завершен")
}

/*
ПРИМЕР 7: СУБТЕСТЫ С АВТОНОМНЫМИ ПРОВЕРКАМИ
t.Run() позволяет создавать иерархию тестов. Отлично для документации
*/
func testUserScenario(t *testing.T) {
	t.Run("шаг 1: создание пользователя", func(t *testing.T) {
		// проверка создания
		t.Log("Пользователь создан")
	})

	t.Run("шаг 2: обновление профиля", func(t *testing.T) {
		// проверка обновления
		t.Log("Профиль обновлен")
	})

	t.Run("шаг 3: удаление", func(t *testing.T) {
		// проверка удаления
		t.Log("Пользователь удален")
	})
}

/*
БЛОК 2: ГЛУБОКОЕ ПОНИМАНИЕ UNIT-ТЕСТИРОВАНИЯ

2.1 РОЛЬ UNIT-ТЕСТОВ В КАЧЕСТВЕ КОДА

Unit-тесты — это не просто "проверка что работает". Это:
   - Документация: Тесты показывают, как функция должна себя вести
   - Страховка при рефакторинге: Сломавшийся тест = сразу видно где ошибка
   - Дизайн-инструмент: Если сложно тестировать — значит плохая архитектура
   - Регрессионная защита: Баг → Тест → Баг не вернется

ПРАВИЛО: Хороший тест падает ТОЛЬКО когда реально сломана бизнес-логика.
Плохой тест падает из-за изменения порядка полей в структуре или форматирования.

2.2 СЛОЖНЫЕ ТЕСТОВЫЕ СЦЕНАРИИ

КЛАССЫ ЭКВИВАЛЕНТНОСТИ:
   - Нормальные данные (happy path)
   - Граничные значения (0, пустой слайс, nil)
   - Некорректные данные (отрицательные числа, неверные форматы)
   - Комбинации условий (несколько ошибок одновременно)

ПРИНЦИПЫ ПОСТРОЕНИЯ КЕЙСОВ:
   - Один тест — одна концепция (не проверяй всё в одном t.Run)
   - Имена кейсов должны описывать СЦЕНАРИЙ, а не входные данные
   - Покрывай не только успешные, но и ошибочные пути

2.3 НЕЗАВИСИМОСТЬ И ИЗОЛЯЦИЯ ТЕСТОВ

ЗОЛОТОЕ ПРАВИЛО: Тесты не должны влиять друг на друга.

ПЛОХО (зависимость):
   var globalCounter int
   func TestA(t *testing.T) { globalCounter = 1 }
   func TestB(t *testing.T) { if globalCounter != 1 { ... } } // ОПАСНО!

ХОРОШО:
   - Каждый тест создает свои данные
   - Нет глобальных переменных
   - Используем t.Parallel() только когда уверены в изоляции

ИЗОЛЯЦИЯ ОТ ВНЕШНИХ ЗАВИСИМОСТЕЙ:
   - Нет реальной БД → используем моки
   - Нет реального HTTP → используем httptest
   - Нет реальных файлов → используем temp файлы или bytes.Buffer

2.4 ПРОВЕРКА ОШИБОК И УСЛОВИЙ

ЧТО ПРОВЕРЯТЬ В ОШИБКЕ:
   1. Что ошибка ВООБЩЕ произошла (err != nil)
   2. Тип ошибки (errors.Is, errors.As)
   3. Текст ошибки (только если тип не дает инфы)
   4. Что при ошибке НЕТ полезных данных (user == nil)

ЧЕГО НЕ ДЕЛАТЬ:
   - Не проверяй точный текст ошибки (хрупкие тесты)
   - Не игнорируй ошибки через _
   - Не проверяй ошибку если её быть не должно
*/

// ТЕСТИРУЕМЫЙ КОД (пример сервиса с зависимостями)

// user — модель пользователя
type user struct {
	id    int
	name  string
	email string
	age   int
}

// UserRepository — интерфейс зависимостей (будем мокать)
type userRepository interface {
	findByID(id int) (*user, error)
	save(user *user) error
}

// EmailValidator — ещё одна зависимость
type emailValidator interface {
	isValid(email string) bool
}

// UserService — сервис с несколькими зависимостями
type userService struct {
	repo      userRepository
	validator emailValidator
}

// NewUserService — конструктор
func newUserService(repo userRepository, validator emailValidator) *userService {
	return &userService{
		repo:      repo,
		validator: validator,
	}
}

// GetUser — получение пользователя с бизнес-логикой
func (s *userService) getUser(id int) (*user, error) {
	if id <= 0 {
		return nil, errors.New("id must be positive")
	}
	return s.repo.findByID(id)
}

// CreateUser — создание с валидацией
func (s *userService) createUser(name, email string, age int) (*user, error) {
	// Валидация
	if name == "" {
		return nil, errors.New("name is required")
	}
	if !s.validator.isValid(email) {
		return nil, errors.New("invalid email")
	}
	if age < 0 || age > 150 {
		return nil, errors.New("age must be between 0 and 150")
	}

	user := &user{
		name:  name,
		email: email,
		age:   age,
	}

	if err := s.repo.save(user); err != nil {
		return nil, fmt.Errorf("failed to save user: %w", err)
	}
	return user, nil
}

// UpdateUserEmail — обновление email с проверками
func (s *userService) updateUserEmail(id int, newEmail string) error {
	if id <= 0 {
		return errors.New("invalid id")
	}
	if !s.validator.isValid(newEmail) {
		return errors.New("invalid email")
	}

	user, err := s.repo.findByID(id)
	if err != nil {
		return fmt.Errorf("user not found: %w", err)
	}
	if user == nil {
		return errors.New("user not found")
	}

	user.email = newEmail
	return s.repo.save(user)
}

// МОКИ ДЛЯ ТЕСТИРОВАНИЯ (ручные, без testify)

// mockUserRepository — ручной мок для тестов
type mockUserRepository struct {
	users      map[int]*user
	saveCalled bool
	lastSaved  *user
	findError  error
	saveError  error
}

func newMockUserRepository() *mockUserRepository {
	return &mockUserRepository{
		users: make(map[int]*user),
	}
}

func (m *mockUserRepository) findByID(id int) (*user, error) {
	if m.findError != nil {
		return nil, m.findError
	}
	user, ok := m.users[id]
	if !ok {
		return nil, nil // не найдено, но не ошибка
	}
	return user, nil
}

func (m *mockUserRepository) save(user *user) error {
	m.saveCalled = true
	m.lastSaved = user
	if m.saveError != nil {
		return m.saveError
	}
	if user.id == 0 {
		user.id = 1 // имитация генерации ID
	}
	m.users[user.id] = user
	return nil
}

// mockEmailValidator — ручной мок
type mockEmailValidator struct {
	valid bool
}

func (m *mockEmailValidator) isValid(email string) bool {
	return m.valid
}

// ПРИМЕР 1: HAPPY PATH — ОСНОВНОЙ СЦЕНАРИЙ
func testUserService_GetUser_Success(t *testing.T) {
	// Подготовка
	mockRepo := newMockUserRepository()
	mockRepo.users[1] = &user{
		id:    1,
		name:  "Kirill",
		email: "Clep.k@mail.ru",
	}

	service := newUserService(mockRepo, &mockEmailValidator{})

	// Действие
	user, err := service.getUser(1)

	// Проверки
	if err != nil {
		t.Errorf("не ожидали ошибку, получили %v", err)
	}
	if user == nil {
		t.Fatal("пользователь не должен быть nil")
	}
	if user.name != "Alice" {
		t.Errorf("ожидали Name=Alice, получили %s", user.name)
	}
	if user.email != "alice@example.com" {
		t.Errorf("ожидали Email=alice@example.com, получили %s", user.email)
	}
}

// ПРИМЕР 2: ТЕСТИРОВАНИЕ ГРАНИЧНЫХ УСЛОВИЙ
func testUserService_GetUser_InvalidID(t *testing.T) {
	mockRepo := newMockUserRepository()
	service := newUserService(mockRepo, &mockEmailValidator{})

	tests := []struct {
		name      string
		id        int
		wantError string
	}{
		{name: "ID равен нулю", id: 0, wantError: "id must be positive"},
		{name: "ID отрицательный", id: -1, wantError: "id must be positive"},
		{name: "ID очень большой", id: 999999, wantError: ""}, // тут ошибки не ждем, просто не найдено
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			user, err := service.getUser(tt.id)

			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("ожидали ошибку '%s', но получили nil", tt.wantError)
				}
				if err.Error() != tt.wantError {
					t.Errorf("ожидали ошибку '%s', получили '%s'", tt.wantError, err.Error())
				}
				if user != nil {
					t.Errorf("при ошибке user должен быть nil, получили %+v", user)
				}
			} else {
				if err != nil {
					t.Errorf("не ожидали ошибку, получили %v", err)
				}
				// nil — это нормально (пользователь не найден)
			}
		})
	}
}

// ПРИМЕР 3: ТЕСТИРОВАНИЕ МНОЖЕСТВЕННЫХ УСЛОВИЙ (VALIDATION)
func testUserService_CreateUser_Validation(t *testing.T) {
	tests := []struct {
		name      string
		userName  string
		email     string
		age       int
		valid     bool
		wantError string
	}{
		{
			name:      "всё правильно",
			userName:  "Bob",
			email:     "bob@example.com",
			age:       25,
			valid:     true,
			wantError: "",
		},
		{
			name:      "пустое имя",
			userName:  "",
			email:     "bob@example.com",
			age:       25,
			valid:     true,
			wantError: "name is required",
		},
		{
			name:      "невалидный email",
			userName:  "Bob",
			email:     "not-an-email",
			age:       25,
			valid:     false,
			wantError: "invalid email",
		},
		{
			name:      "отрицательный возраст",
			userName:  "Bob",
			email:     "bob@example.com",
			age:       -5,
			valid:     true,
			wantError: "age must be between 0 and 150",
		},
		{
			name:      "возраст 150 (граница)",
			userName:  "Bob",
			email:     "bob@example.com",
			age:       150,
			valid:     true,
			wantError: "",
		},
		{
			name:      "возраст 151 (за границей)",
			userName:  "Bob",
			email:     "bob@example.com",
			age:       151,
			valid:     true,
			wantError: "age must be between 0 and 150",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := newMockUserRepository()
			mockValidator := &mockEmailValidator{valid: tt.valid}
			service := newUserService(mockRepo, mockValidator)

			user, err := service.createUser(tt.name, tt.email, tt.age)

			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("ожидали ошибку '%s', но получили nil", tt.wantError)
				}
				if err.Error() != tt.wantError {
					t.Errorf("ожидали '%s', получили '%s'", tt.wantError, err.Error())
				}
				if user != nil {
					t.Errorf("при ошибке user должен быть nil")
				}
			} else {
				if err != nil {
					t.Fatalf("не ожидали ошибку, получили %v", err)
				}
				if user == nil {
					t.Fatal("user не должен быть nil")
				}
				if user.name != tt.userName {
					t.Errorf("имя: ожидали %s, получили %s", tt.userName, user.name)
				}
				if user.email != tt.email {
					t.Errorf("email: ожидали %s, получили %s", tt.email, user.email)
				}
				if user.age != tt.age {
					t.Errorf("возраст: ожидали %d, получили %d", tt.age, user.age)
				}
				// ID должен был сгенерироваться
				if user.id == 0 {
					t.Error("ID не должен быть 0")
				}
			}
		})
	}
}

// ПРИМЕР 4: ТЕСТИРОВАНИЕ ОШИБОК ЗАВИСИМОСТЕЙ (ИЗОЛЯЦИЯ)
func testUserService_CreateUser_RepoError(t *testing.T) {
	// Проверяем, как сервис реагирует на ошибку от репозитория
	mockRepo := newMockUserRepository()
	mockRepo.saveError = errors.New("connection refused")

	mockValidator := &mockEmailValidator{valid: true}
	service := newUserService(mockRepo, mockValidator)

	user, err := service.createUser("Alice", "alice@example.com", 30)

	// Проверки
	if err == nil {
		t.Fatal("ожидали ошибку от репозитория, но получили nil")
	}
	if user != nil {
		t.Errorf("при ошибке сохранения user должен быть nil, получили %+v", user)
	}
	// Проверяем что ошибка обернута правильно
	if !errors.Is(err, mockRepo.saveError) {
		t.Errorf("ошибка должна содержать original error, получили %v", err)
	}
}

// ПРИМЕР 5: ТЕСТИРОВАНИЕ КОМБИНАЦИЙ УСЛОВИЙ
func testUserService_UpdateUserEmail_Scenarios(t *testing.T) {
	tests := []struct {
		name         string
		setupRepo    func(*mockUserRepository)
		id           int
		newEmail     string
		validatorRes bool
		wantError    string
		expectSave   bool
	}{
		{
			name: "успешное обновление",
			setupRepo: func(r *mockUserRepository) {
				r.users[1] = &user{id: 1, name: "Alice", email: "old@example.com"}
			},
			id:           1,
			newEmail:     "new@example.com",
			validatorRes: true,
			wantError:    "",
			expectSave:   true,
		},
		{
			name:         "невалидный ID",
			setupRepo:    func(r *mockUserRepository) {},
			id:           0,
			newEmail:     "new@example.com",
			validatorRes: true,
			wantError:    "invalid id",
			expectSave:   false,
		},
		{
			name: "пользователь не найден",
			setupRepo: func(r *mockUserRepository) {
				// не добавляем пользователя
			},
			id:           999,
			newEmail:     "new@example.com",
			validatorRes: true,
			wantError:    "user not found",
			expectSave:   false,
		},
		{
			name: "невалидный email",
			setupRepo: func(r *mockUserRepository) {
				r.users[1] = &user{id: 1, name: "Alice"}
			},
			id:           1,
			newEmail:     "invalid",
			validatorRes: false,
			wantError:    "invalid email",
			expectSave:   false,
		},
		{
			name: "ошибка репозитория при поиске",
			setupRepo: func(r *mockUserRepository) {
				r.findError = errors.New("database down")
			},
			id:           1,
			newEmail:     "new@example.com",
			validatorRes: true,
			wantError:    "user not found",
			expectSave:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := newMockUserRepository()
			if tt.setupRepo != nil {
				tt.setupRepo(mockRepo)
			}
			mockValidator := &mockEmailValidator{valid: tt.validatorRes}
			service := newUserService(mockRepo, mockValidator)

			err := service.updateUserEmail(tt.id, tt.newEmail)

			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("ожидали ошибку '%s', но получили nil", tt.wantError)
				}
				// Проверяем что ошибка содержит ожидаемый текст
				if err.Error() != tt.wantError {
					t.Errorf("ожидали '%s', получили '%s'", tt.wantError, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("не ожидали ошибку, получили %v", err)
				}
			}

			// Проверяем, был ли вызван Save (важно для изоляции!)
			if mockRepo.saveCalled != tt.expectSave {
				t.Errorf("Save был вызван: %v, ожидалось: %v", mockRepo.saveCalled, tt.expectSave)
			}

			// Если ожидали успешное обновление — проверяем email
			if tt.wantError == "" && tt.expectSave && mockRepo.lastSaved != nil {
				if mockRepo.lastSaved.email != tt.newEmail {
					t.Errorf("email не обновился: ожидали %s, получили %s",
						tt.newEmail, mockRepo.lastSaved.email)
				}
			}
		})
	}
}

// ПРИМЕР 6: ИЗОЛЯЦИЯ ТЕСТОВ (НЕТ ГЛОБАЛЬНЫХ ПЕРЕМЕННЫХ)
/*
 ПЛОХО (НЕ ДЕЛАЙ ТАК):
 var globalRepo *mockUserRepository  // <- глобальная переменная

 func TestA(t *testing.T) {
     globalRepo = newMockUserRepository()
     globalRepo.users[1] = &User{ID: 1}
 }

 func TestB(t *testing.T) {
	 ОПАСНО! TestB зависит от порядка запуска
     user, _ := globalRepo.FindByID(1)
}
*/
// ХОРОШО (каждый тест создаёт свои данные):
func testIsolation_TestA(t *testing.T) {
	repo := newMockUserRepository()
	repo.users[1] = &user{id: 1, name: "Alice"}

	user, _ := repo.findByID(1)
	if user.name != "Alice" {
		t.Error("wrong name")
	}
}

func testIsolation_TestB(t *testing.T) {
	// СВОЙ репозиторий, не связанный с TestA
	repo := newMockUserRepository()
	// даже если не добавить пользователя — тест не сломается от TestA
	user, _ := repo.findByID(1)
	if user != nil {
		t.Error("ожидали nil, но получили пользователя из другого теста?")
	}
}

/*
БЛОК 3: ОРГАНИЗАЦИЯ ТЕСТОВ, ПОКРЫТИЕ И МОКИРОВАНИЕ

3.1 ОРГАНИЗАЦИЯ ТЕСТОВОГО КОДА В СТРУКТУРИРОВАННЫЕ КАТАЛОГИ

СТАНДАРТНАЯ СТРУКТУРА ПРОЕКТА:
   /project
   ├── go.mod
   ├── internal/
   │   └── service/
   │       ├── user.go
   │       └── user_test.go      <- тесты рядом с кодом
   ├── pkg/
   │   └── api/
   │       ├── handler.go
   │       └── handler_test.go
   └── test/
       ├── integration/          <- интеграционные тесты
       └── fixtures/             <- тестовые данные (JSON, golden files)

ПРАВИЛА:
   - Тесты лежат В ТОЙ ЖЕ ПАПКЕ, что и код (рядом)
   - Файл *_test.go компилируется ТОЛЬКО при запуске тестов
   - Интеграционные тесты можно выносить в /test, но чаще просто используют тег +build

РАЗДЕЛЕНИЕ ПАКЕТОВ:
   package service        // белый ящик — видим приватные поля
   package service_test   // чёрный ящик — только публичный API (рекомендуется)

3.2 ПОКРЫТИЕ КОДА ТЕСТАМИ

ЧТО ТАКОЕ ПОКРЫТИЕ:
   - Процент строк кода, которые выполняются во время тестов
   - НЕ гарантирует качество (можно покрыть 100% но не проверить логику)

КОМАНДЫ ДЛЯ РАБОТЫ С ПОКРЫТИЕМ:
   go test -cover                    # процент в консоли
   go test -coverprofile=cover.out   # сохраняем профиль
   go tool cover -func=cover.out     # покрытие по функциям
   go tool cover -html=cover.out     # открываем в браузере (зелёное — покрыто)

НОРМЫ ПОКРЫТИЯ:
   - 50-60%: нормально для утилит
   - 70-80%: хорошо для бизнес-логики
   - 90%+: обычно оверхед (тратишь время на тривиальные геттеры)

ВАЖНО: Не гонись за 100%. 100% покрытие не значит "нет багов".

3.3 МОКИРОВАНИЕ И ФЕЙКОВЫЕ ОБЪЕКТЫ

МОК (Mock):
   - Объект, который запоминает вызовы (был ли вызван, с какими параметрами)
   - Проверяет, что метод вызвали ОЖИДАЕМЫМ образом
   - Используется для проверки взаимодействий

ФЕЙК (Fake):
   - Упрощённая РАБОЧАЯ реализация (например, БД в памяти)
   - Не проверяет вызовы, просто работает
   - Используется когда нужна реальная логика, но без внешних зависимостей

СТАБ (Stub):
   - Возвращает заданные ответы, не проверяя вызовы
   - Самый простой тип

КОГДА ЧТО ИСПОЛЬЗОВАТЬ:
   - Mock: когда важно, что метод вызвали (например, Save вызван 1 раз)
   - Fake: когда нужна работающая логика (сортировка, фильтрация)
   - Stub: когда просто нужно вернуть значение, не важно как

РУЧНЫЕ МОКИ vs TESTIFY/MOCK:
   - Ручные: больше кода, но полный контроль
   - Testify: меньше кода, удобные AssertExpectations

3.4 ТАБЛИЧНЫЕ ТЕСТЫ И ПОДГОТОВКА ТЕСТОВЫХ ДАННЫХ

СТРУКТУРА ТАБЛИЧНОГО ТЕСТА:
   tests := []struct {
       name     string
       input    int
       want     int
       wantErr  bool
       setup    func() // подготовка данных
   }

ПОДГОТОВКА ТЕСТОВЫХ ДАННЫХ:
   - Встроенные данные (в коде): для простых кейсов
   - testdata/папка: для больших JSON/файлов
   - Golden files: эталонные выходные данные
   - Fixtures: предустановленные данные для БД

ЗОЛОТЫЕ ФАЙЛЫ (Golden Files):
   testdata/
   ├── expected_user.json.golden
   └── expected_list.json.golden

   Используй testdata, когда результат слишком большой для хранения в коде.
*/

// ТЕСТИРУЕМЫЙ КОД

type product struct {
	id    int
	name  string
	price int
}

type productRepository interface {
	getByID(id int) (*product, error)
	save(product *product) error
}

type productService struct {
	repo productRepository
}

func newProductService(repo productRepository) *productService {
	return &productService{repo: repo}
}

func (s *productService) getProduct(id int) (*product, error) {
	if id <= 0 {
		return nil, errors.New("invalid id")
	}
	return s.repo.getByID(id)
}

func (s *productService) createProduct(name string, price int) (*product, error) {
	if name == "" {
		return nil, errors.New("name required")
	}
	if price <= 0 {
		return nil, errors.New("price must be positive")
	}
	product := &product{name: name, price: price}
	if err := s.repo.save(product); err != nil {
		return nil, err
	}
	return product, nil
}

// ПРИМЕР 1: РУЧНЫЕ МОКИ (С ПРОВЕРКОЙ ВЫЗОВОВ)
type mockProductRepo struct {
	// Данные
	products map[int]*product

	// Контроль вызовов
	getByIDCalled bool
	getByIDArg    int
	saveCalled    bool
	saveArg       *product

	// Возвращаемые значения
	getByIDResult *product
	getByIDError  error
	saveError     error
}

func newMockProductRepo() *mockProductRepo {
	return &mockProductRepo{
		products: make(map[int]*product),
	}
}

func (m *mockProductRepo) getByID(id int) (*product, error) {
	m.getByIDCalled = true
	m.getByIDArg = id

	if m.getByIDError != nil {
		return nil, m.getByIDError
	}
	if m.getByIDResult != nil {
		return m.getByIDResult, nil
	}
	// возвращаем из мапы если есть
	product, ok := m.products[id]
	if !ok {
		return nil, nil
	}
	return product, nil
}

func (m *mockProductRepo) save(product *product) error {
	m.saveCalled = true
	m.saveArg = product

	if m.saveError != nil {
		return m.saveError
	}
	if product.id == 0 {
		product.id = len(m.products) + 1
	}
	m.products[product.id] = product
	return nil
}

// Методы для проверки ожиданий
func (m *mockProductRepo) assertGetByIDCalled(t *testing.T, expectedID int) {
	t.Helper()
	if !m.getByIDCalled {
		t.Errorf("GetByID не был вызван")
	}
	if m.getByIDArg != expectedID {
		t.Errorf("GetByID вызван с id=%d, ожидалось %d", m.getByIDArg, expectedID)
	}
}

func (m *mockProductRepo) assertSaveCalled(t *testing.T) {
	t.Helper()
	if !m.saveCalled {
		t.Errorf("Save не был вызван")
	}
}

func testProductService_GetProduct_WithMock(t *testing.T) {
	mockRepo := newMockProductRepo()
	mockRepo.getByIDResult = &product{id: 5, name: "Laptop", price: 1000}

	service := newProductService(mockRepo)

	product, err := service.getProduct(5)

	if err != nil {
		t.Fatalf("не ожидали ошибку, получили %v", err)
	}
	if product.name != "Laptop" {
		t.Errorf("ожидали Laptop, получили %s", product.name)
	}

	// Проверяем, что метод был вызван с правильным параметром
	mockRepo.assertGetByIDCalled(t, 5)
}

// ПРИМЕР 2: ПОКРЫТИЕ КОДА (ДЕМОНСТРАЦИЯ)
/*
Запусти эти команды:
go test -cover
go test -coverprofile=cover.out
go tool cover -func=cover.out
go tool cover -html=cover.out
*/

func testCoverageExample(t *testing.T) {
	tests := []struct {
		name      string
		productID int
		wantError bool
	}{
		{"valid id", 5, false},
		{"invalid id zero", 0, true},
		{"invalid id negative", -1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := newMockProductRepo()
			mockRepo.products[5] = &product{id: 5, name: "Test"}

			service := newProductService(mockRepo)
			_, err := service.getProduct(tt.productID)

			if tt.wantError && err == nil {
				t.Error("ожидали ошибку")
			}
			if !tt.wantError && err != nil {
				t.Errorf("не ожидали ошибку, получили %v", err)
			}
		})
	}
}

// ПРИМЕР 3: ТЕСТОВЫЕ ДАННЫЕ ИЗ ФАЙЛОВ (GOLDEN FILES)
/*
Предполагаем файл testdata/expected_product.json:
{"id": 1, "name": "Test Product", "price": 100}
*/
func loadExpectedProduct(t *testing.T) *product {
	t.Helper()

	// Читаем golden файл
	path := filepath.Join("testdata", "expected_product.json.golden")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Skipf("golden file not found: %v", err)
	}

	var product product
	if err := json.Unmarshal(data, &product); err != nil {
		t.Fatalf("failed to parse golden file: %v", err)
	}
	return &product
}
func testProduct_GoldenFile(t *testing.T) {
	// Создаем тестовый продукт
	actual := &product{
		id:    1,
		name:  "Test Product",
		price: 100,
	}

	expected := loadExpectedProduct(t)

	if actual.id != expected.id {
		t.Errorf("ID: got %d, want %d", actual.id, expected.id)
	}
	if actual.name != expected.name {
		t.Errorf("Name: got %s, want %s", actual.name, expected.name)
	}
	if actual.price != expected.price {
		t.Errorf("Price: got %d, want %d", actual.price, expected.price)
	}
}

// ПРИМЕР 4: ПОДГОТОВКА ТЕСТОВЫХ ДАННЫХ (HELPER FUNCTIONS)
// setupTestProducts — создаёт тестовые данные и возвращает функцию очистки
func setupTestProducts(t *testing.T) (*productService, *mockProductRepo, func()) {
	t.Helper()

	mockRepo := newMockProductRepo()
	mockRepo.products[1] = &product{id: 1, name: "Product A", price: 100}
	mockRepo.products[2] = &product{id: 2, name: "Product B", price: 200}

	service := newProductService(mockRepo)

	cleanup := func() {
		// ничего не делаем, но в реальном коде можно закрыть соединения
		t.Log("cleanup completed")
	}

	return service, mockRepo, cleanup
}

func TestWithSetupHelper(t *testing.T) {
	service, mockRepo, cleanup := setupTestProducts(t)
	defer cleanup()

	product, err := service.getProduct(1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if product.name != "Product A" {
		t.Errorf("expected Product A, got %s", product.name)
	}

	mockRepo.assertGetByIDCalled(t, 1)
}

// ПРИМЕР 7: СТРУКТУРА ПАПОК ДЛЯ ТЕСТОВ
/*
Правильная организация тестов в проекте:

/myapp
├── go.mod
├── internal/
│   ├── service/
│   │   ├── user.go
│   │   ├── user_test.go           <- юнит-тесты
│   │   ├── mocks/
│   │   │   └── user_repo_mock.go  <- ручные моки (или использовать testify)
│   │   └── testdata/
│   │       └── expected_user.json
│   └── repository/
│       ├── postgres.go
│       └── postgres_test.go
├── pkg/
│   └── api/
│       ├── handler.go
│       └── handler_test.go
└── tests/
    ├── integration/                <- интеграционные тесты
    │   └── api_test.go
    └── e2e/
        └── full_flow_test.go
*/

/*
БЛОК 4: TDD, СЛОЖНЫЕ СЦЕНАРИИ, ПРОИЗВОДИТЕЛЬНОСТЬ И CI/CD

4.1 TDD (TEST-DRIVEN DEVELOPMENT) — РАЗРАБОТКА ЧЕРЕЗ ТЕСТИРОВАНИЕ

ЧТО ТАКОЕ TDD:
   - КРАСНЫЙ: написать тест, который падает (red)
   - ЗЕЛЁНЫЙ: написать минимальный код, чтобы тест прошёл (green)
   - РЕФАКТОРИНГ: улучшить код, не ломая тесты (refactor)

ПОЧЕМУ TDD ВАЖЕН:
   - тесты становятся спецификацией поведения
   - код получается тестируемым по определению
   - уменьшает количество багов
   - даёт уверенность при рефакторинге

ПРАВИЛА TDD:
   - не пиши код без падающего теста
   - не пиши больше теста, чем нужно для падения
   - не пиши больше кода, чем нужно для прохождения теста

4.2 СЛОЖНЫЕ ТЕСТОВЫЕ СЦЕНАРИИ

ЧТО ДОЛЖНО БЫТЬ ПОКРЫТО:
   - счастливый путь (happy path)
   - ошибки зависимостей (репозиторий упал, сеть недоступна)
   - граничные значения (0, -1, пустой слайс, nil)
   - комбинации условий (несколько ошибок одновременно)
   - конкурентные сценарии (гонки данных, deadlock)
   - таймауты и отмена контекста

ПАТТЕРНЫ СЛОЖНЫХ ТЕСТОВ:
   - тест на панику (panic recovery)
   - тест на идемпотентность (повторный вызов)
   - тест на конкурентность (с -race)
   - тест на timeout (select с таймером)

4.3 ТЕСТИРОВАНИЕ ПРОИЗВОДИТЕЛЬНОСТИ И ПРОФИЛИРОВАНИЕ

БЕНЧМАРКИ (BENCHMARK):
   - функции вида BenchmarkXxx(b *testing.B)
   - запуск: go test -bench=.
   - флаги: -benchmem (показать аллокации), -count=10 (усреднить)
   - важны не только ns/op, но и allocs/op

ПРОФИЛИРОВАНИЕ В ТЕСТАХ:
   - cpu профиль: go test -cpuprofile=cpu.out
   - память: go test -memprofile=mem.out
   - анализ: go tool pprof -http=:8080 cpu.out

ЧТО СМОТРЕТЬ В PPROF:
   - flat: сколько времени заняла функция (без вложенных)
   - cum: сколько времени заняла функция (с вложенными)
   - самые горячие точки — кандидаты на оптимизацию

4.4 CI/CD И АВТОМАТИЗАЦИЯ

ОБЯЗАТЕЛЬНЫЕ ШАГИ В CI:
   - go test -race ./...        (детектор гонок)
   - go test -cover              (покрытие)
   - go vet ./...                (статический анализ)
   - staticcheck ./...           (линтер)

ПОРЯДОК ЗАПУСКА В CI:
   1. go mod download
   2. go vet ./...
   3. go test -race -coverprofile=cover.out ./...
   4. go tool cover -func=cover.out (проверка порога покрытия)
   5. go test -bench=. -benchmem ./...

QUALITY GATES (КАЧЕСТВЕННЫЕ ВОРОТА):
   - покрытие не ниже 70%
   - нет гонок данных (-race)
   - все тесты проходят
   - бенчмарки не деградировали (хранить историю)
*/

// ТЕСТИРУЕМЫЙ КОД (будем писать через TDD)
// calculator — простой калькулятор с историей
type calculator struct {
	history []string
	mu      sync.RWMutex
}

// newCalculator — конструктор
func newCalculator() *calculator {
	return &calculator{
		history: make([]string, 0),
	}
}

// add — складывает два числа
func (c *calculator) add(a, b int) int {
	result := a + b
	c.mu.Lock()
	c.history = append(c.history, fmt.Sprintf("%d+%d=%d", a, b, result))
	c.mu.Unlock()
	return result
}

// divide — делит a на b (с защитой от деления на ноль)
func (c *calculator) divide(a, b int) (int, error) {
	if b == 0 {
		return 0, errors.New("division by zero")
	}
	result := a / b
	c.mu.Lock()
	c.history = append(c.history, fmt.Sprintf("%d/%d=%d", a, b, result))
	c.mu.Unlock()
	return result, nil
}

// history — возвращает историю операций
func (c *calculator) History() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	// возвращаем копию, чтобы избежать гонок
	result := make([]string, len(c.history))
	copy(result, c.history)
	return result
}

// ПРИМЕР 1: TDD ПОДХОД (СНАЧАЛА ТЕСТ, ПОТОМ КОД)
/*
ШАГ 1 (КРАСНЫЙ): Пишем тест на функцию, которой ещё нет
func TestMultiply(t *testing.T) {
    calc := newCalculator()
    result := calc.multiply(3, 4)  // функция ещё не существует
    if result != 12 {
        t.Errorf("expected 12, got %d", result)
    }
}
// Запускаем: go test -run=TestMultiply
// ОШИБКА: calc.multiply undefined (тест падает — красный)

ШАГ 2 (ЗЕЛЁНЫЙ): Пишем минимальную реализацию
func (c *calculator) multiply(a, b int) int {
    return a * b
}
// Запускаем: тест проходит — зелёный

ШАГ 3 (РЕФАКТОРИНГ): Улучшаем код, тест остаётся зелёным
// можно добавить логирование, историю и т.д.
*/

// итоговый тест после всех шагов:
func TestCalculator_Multiply_TDD(t *testing.T) {
	calc := newCalculator()

	tests := []struct {
		name     string
		a, b     int
		expected int
	}{
		{"positive", 3, 4, 12},
		{"negative", -2, 3, -6},
		{"zero", 5, 0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// умножаем — функция уже существует
			result := calc.multiply(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("%s: got %d, want %d", tt.name, result, tt.expected)
			}
		})
	}
}

// реализация multiply (дописываем после теста)
func (c *calculator) multiply(a, b int) int {
	result := a * b
	c.mu.Lock()
	c.history = append(c.history, fmt.Sprintf("%d*%d=%d", a, b, result))
	c.mu.Unlock()
	return result
}

// ПРИМЕР 2: ТЕСТ НА ПАНИКУ (PANIC RECOVERY)
// безопасный доступ к слайсу
func safeGet(s []int, idx int) (value int, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()
	return s[idx], nil
}

func testSafeGet_Panic(t *testing.T) {
	tests := []struct {
		name      string
		slice     []int
		idx       int
		wantValue int
		wantErr   bool
	}{
		{"valid index", []int{1, 2, 3}, 1, 2, false},
		{"out of range", []int{1, 2, 3}, 10, 0, true},
		{"nil slice", nil, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value, err := safeGet(tt.slice, tt.idx)

			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if value != tt.wantValue {
				t.Errorf("value: got %d, want %d", value, tt.wantValue)
			}
		})
	}
}

// ПРИМЕР 3: ТЕСТ НА КОНКУРЕНТНОСТЬ (RACE DETECTOR)
// counter — счётчик с защитой от гонок
type counter struct {
	mu    sync.Mutex
	value int
}

func (c *counter) inc() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
}

func (c *counter) val() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// raceTest — тест, который должен запускаться с -race
func testCounter_Concurrency(t *testing.T) {
	c := &counter{}
	var wg sync.WaitGroup

	// запускаем 1000 горутин, каждая увеличивает счётчик
	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.inc()
		}()
	}
	wg.Wait()

	// проверяем итоговое значение
	if c.val() != 1000 {
		t.Errorf("expected 1000, got %d", c.val())
	}
}

// ПРИМЕР 4: БЕНЧМАРКИ (ТЕСТИРОВАНИЕ ПРОИЗВОДИТЕЛЬНОСТИ)
// запуск: go test -bench=. -benchmem -count=5

func benchmarkCalculator_Add(b *testing.B) {
	calc := newCalculator()

	// b.N — количество итераций, которые определяет рантайм
	for i := 0; i < b.N; i++ {
		calc.add(1, 2)
	}
}

func benchmarkCalculator_Multiply(b *testing.B) {
	calc := newCalculator()

	for i := 0; i < b.N; i++ {
		calc.multiply(3, 4)
	}
}

// сравниваем два способа конкатенации строк
func benchmarkStringConcat_Plus(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = "hello" + " " + "world"
	}
}

func benchmarkStringConcat_Sprintf(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = fmt.Sprintf("%s %s", "hello", "world")
	}
}

// ПРИМЕР 5: ПРОФИЛИРОВАНИЕ В ТЕСТАХ
// чтобы сохранить профиль: go test -cpuprofile=cpu.out -bench=.
// анализ: go tool pprof -http=:8080 cpu.out

func benchmarkHeavyCalculation(b *testing.B) {
	calc := newCalculator()

	for i := 0; i < b.N; i++ {
		for j := 0; j < 1000; j++ {
			calc.add(j, j+1)
			calc.multiply(j, j+1)
		}
	}
}

// ПРИМЕР 6: ТЕСТ НА УТЕЧКУ ГОРУТИН
func TestNoGoroutineLeak(t *testing.T) {
	// запоминаем количество горутин до теста
	runtime.GC()
	before := runtime.NumGoroutine()

	// запускаем что-то, что должно корректно завершаться
	done := make(chan bool)
	go func() {
		time.Sleep(10 * time.Millisecond)
		done <- true
	}()
	<-done

	runtime.GC()
	after := runtime.NumGoroutine()

	if after > before {
		t.Errorf("goroutine leak: before=%d, after=%d", before, after)
	}
}
