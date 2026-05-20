package _testing_testify

import (
	"errors"
	"strings"
	"testing"
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
