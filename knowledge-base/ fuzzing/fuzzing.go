package _fuzzing

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

/*
БЛОК 1: ОСНОВЫ ФАЗЗ-ТЕСТИРОВАНИЯ

1.1 ЧТО ТАКОЕ ФАЗЗ-ТЕСТИРОВАНИЕ (FUZZ TESTING)

Фазз-тестирование — это автоматический метод тестирования, который подаёт на вход функции
СЛУЧАЙНЫЕ, НЕОЖИДАННЫЕ или НЕКОРРЕКТНЫЕ данные (мусор, спецсимволы, огромные строки, -1, 0, nil)
чтобы найти скрытые ошибки, паники и уязвимости.

ОТЛИЧИЕ ОБЫЧНЫХ ТЕСТОВ ОТ ФАЗЗИНГА:
   - Обычные тесты: ты сам пишешь входные данные (2, 3, "hello")
   - Фаззинг: компьютер сам генерирует тысячи/миллионы вариантов

ПРОСТАЯ АНАЛОГИЯ:
   - Юнит-тест: ты проверяешь, что дверь открывается ключом
   - Фазз-тест: ты дёргаешь ручку, пинаешь дверь, пробуешь отмычку, ломик — ищешь слабые места

1.2 КАКИЕ ОШИБКИ НАХОДИТ ФАЗЗИНГ

ЧАСТО НАХОДИТ:
   - паника из-за выхода за границы слайса (index out of range)
   - бесконечные циклы (неправильное условие выхода)
   - утечки памяти (память не освобождается)
   - deadlock (зависание из-за блокировок)
   - паника при разыменовании nil указателя

РЕАЛЬНЫЕ ПРИМЕРЫ ОШИБОК, НАЙДЕННЫХ ФАЗЗИНГОМ:
   - в стандартной библиотеке Go находили баги в парсерах JSON, regexp, image
   - в crypto/tls находили уязвимости через фаззинг
   - в компиляторах (gcc, llvm) регулярно находят баги через фаззинг

1.3 ПОЧЕМУ ФАЗЗИНГ ВАЖЕН

ПРОБЛЕМЫ, КОТОРЫЕ НЕ НАЙТИ ОБЫЧНЫМИ ТЕСТАМИ:
   - разработчик пишет тесты на то, что ОЖИДАЕТ увидеть (хорошие данные)
   - хрупкие места — это всегда НЕОЖИДАННЫЕ данные
   - невозможно предугадать все возможные входные данные вручную

ФАЗЗИНГ ДАЁТ:
   - автоматическое исследование входного пространства
   - находит баги, о которых ты даже не думал
   - может работать сутками/неделями в CI

1.4 КОГДА ИСПОЛЬЗОВАТЬ ФАЗЗИНГ

ЛУЧШЕ ВСЕГО ПОДХОДИТ ДЛЯ:
   - парсеры (json, xml, yaml, protobuf)
   - декодеры (изображения, аудио, видео)
   - сетевые протоколы (http, grpc, tcp)
   - функции, принимающие []byte или string
   - математические функции (с граничными значениями)

ХУЖЕ ПОДХОДИТ ДЛЯ:
   - чистые бизнес-правила (скидка не может быть больше 100%)
   - функции с чёткими ограничениями (id должен быть положительным)
   - сложные структуры с зависимостями (БД, API)

1.5 КАК РАБОТАЕТ ФАЗЗИНГ (КОРОТКО)

ОСНОВНОЙ ЦИКЛ:
   1. генератор создаёт случайные данные (seed corpus — начальные данные)
   2. фаззер подаёт эти данные в тестируемую функцию
   3. если функция упала (panic, ошибка, зависла) — данные сохраняются
   4. мутация: фаззер изменяет уже найденные "интересные" данные
   5. повторяет цикл миллионы раз

"ИНТЕРЕСНЫЕ" ДАННЫЕ — это те, которые:
   - вызвали новую ветку в коде (новое покрытие)
   - вызвали ошибку/панику
   - привели к новому поведению

1.6 ФАЗЗИНГ В GO (ОБЩЕЕ ПРЕДСТАВЛЕНИЕ)

КОГДА ПОЯВИЛСЯ:
   - Go 1.18 (февраль 2022) — нативная поддержка фаззинга
   - до этого использовали сторонние инструменты (go-fuzz, github.com/dvyukov/go-fuzz)

ОСОБЕННОСТИ ФАЗЗИНГА В GO:
   - встроен в стандартный пакет testing
   - работает параллельно на всех ядрах
   - автоматически сохраняет найденные ошибки в testdata/fuzz
   - умеет воспроизводить найденные паники (go test -run=FuzzXxx/seed)

ФОРМАТ ФУНКЦИИ:
   func FuzzXxx(f *testing.F) {
       // f.Add() — добавляем seed corpus (начальные данные)
       // f.Fuzz(func(t *testing.T, data []byte) { ... })
   }
*/

// ПРИМЕР 1: ФУНКЦИЯ С ПОТЕНЦИАЛЬНЫМИ БАГАМИ
/*
reverseBytes — переворачивает слайс байт (ЕСТЬ БАГ!)
проблема: не обрабатывает пустой слайс и nil правильно
*/
func reverseBytes(data []byte) []byte {
	if data == nil {
		return nil
	}
	result := make([]byte, len(data))
	for i := 0; i < len(data); i++ {
		result[i] = data[len(data)-1]
	}
	return result
}

// ПРИМЕР 2: ОБЫЧНЫЕ ТЕСТЫ (НЕ НАХОДЯТ ВСЕ БАГИ)
func testReverseBytes_Manual(t *testing.T) {
	tests := []struct {
		name string
		in   []byte
		want []byte
	}{
		{"simple", []byte("hello"), []byte("olleh")},
		{"empty", []byte{}, []byte{}},
		{"nil", nil, nil},
		{"single", []byte{'a'}, []byte{'a'}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reverseBytes(tt.in)
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("reverseBytes(%q) = %q, want %q", tt.in, got, tt.want)
				}
			}
		})
	}
}

// ПРИМЕР 3: ФАЗЗ-ТЕСТ (НАХОДИТ НЕОЧЕВИДНЫЕ БАГИ)
// запуск: go test -fuzz=FuzzReverseBytes -fuzztime=30s
func fuzzReverseBytes(f *testing.F) {
	// добавляем seed corpus (начальные данные для фаззера)
	f.Add([]byte("hello"))
	f.Add([]byte(""))
	f.Add([]byte("a"))
	f.Add([]byte("1234567890"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// свойство 1: reverse(reverse(data)) == data
		reversed := reverseBytes(data)
		reversedTwice := reverseBytes(reversed)

		// проверяем, что свойство сохраняется
		for i := 0; i < len(data); i++ {
			if data[i] != reversedTwice[i] {
				t.Errorf("property 1 failed: original[%d]=%d, double reversed[%d]=%d",
					i, data[i], i, reversedTwice[i])
			}
		}

		if len(data) != len(reversed) {
			t.Errorf("property 2 failed: len(original)=%d, len(reversed)=%d",
				len(data), len(reversed))
		}
	})
}

// ПРИМЕР 4: ФАЗЗИНГ СТРОКОВЫХ ФУНКЦИЙ
// isValidUTF8 — проверяет, что строка валидный UTF-8 (ЕСТЬ БАГ!)
// проблема: неправильно обрабатывает некоторые граничные случаи
func isValidUTF8(s string) bool {
	// ПЛОХАЯ РЕАЛИЗАЦИЯ: не проверяет все случаи
	if len(s) == 0 {
		return true
	}
	if len(s) == 1 {
		return s[0] < 0x80 // только ASCII
	}
	return utf8.ValidString(s)
}
func fuzzIsValidUTF8(f *testing.F) {
	// seed corpus: разные строки
	f.Add("hello")
	f.Add("привет")
	f.Add("")
	f.Add("a")
	f.Add(string([]byte{0xFF, 0xFE})) // невалидный UTF-8

	f.Fuzz(func(t *testing.T, s string) {
		// свойство: валидный UTF-8 должен быть валидным
		// свойство: невалидный UTF-8 должен быть невалидным

		isValidStd := utf8.ValidString(s)
		isValidMine := isValidUTF8(s)

		if isValidStd != isValidMine {
			t.Errorf("Mismatch for %q: std=%v, mine=%v", s, isValidStd, isValidMine)
		}
	})
}

// ПРИМЕР 5: КАК ЗАПУСКАТЬ ФАЗЗ-ТЕСТЫ
/*
ОСНОВНЫЕ КОМАНДЫ:

# запустить фазз-тест на 30 секунд
go test -fuzz=FuzzReverseBytes -fuzztime=30s

# запустить фазз-тест без ограничения по времени (до первой ошибки)
go test -fuzz=FuzzReverseBytes -fuzztime=0

# запустить фазз-тест на всех корутинах (по умолчанию использует все CPU)
go test -fuzz=FuzzReverseBytes -parallel=8

# воспроизвести найденную ошибку (панику или провал)
go test -run=FuzzReverseBytes/seed#0

# запустить обычные тесты И фазз-тесты (без генерации новых данных)
go test -run=. -fuzz=.

# посмотреть сохранённые кейсы
ls testdata/fuzz/FuzzReverseBytes/
*/

// ПРИМЕР 6: ЧАСТЫЕ ОШИБКИ ПРИ ФАЗЗИНГЕ
// ПЛОХО (НЕ ДЕЛАЙ ТАК):
func FuzzBadExample(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		//не используем t.Parallel() — фаззинг уже параллельный
		//не делаем глобальных побочных эффектов
		//globalState = data
		//не проверяем медленные свойства
		//time.Sleep(1 * time.Second)
		//не используем внешние зависимости (БД, сеть)
		//не проверяем то, что можно проверить обычным тестом
	})
}

// ХОРОШО:
func FuzzGoodExample(f *testing.F) {
	f.Add([]byte("hello")) // добавляем разнообразные seed

	f.Fuzz(func(t *testing.T, data []byte) {
		//только чистые функции
		//быстрые проверки
		//инварианты и свойства
		//изолированность
	})
}

/*
БЛОК 2: УГЛУБЛЁННОЕ ПОНИМАНИЕ ФАЗЗ-ТЕСТИРОВАНИЯ И go-fuzz

2.1 КАК РАБОТАЕТ ФАЗЗ-ТЕСТИРОВАНИЕ

основной цикл фаззера:
   - начинаем с seed corpus (начальные данные)
   - мутируем данные (битовые флипы, вставки, удаления)
   - выполняем функцию
   - если новое покрытие кода → добавляем в корпус
   - если ошибка/паника → сохраняем как crasher

2.2 ТИПЫ ОШИБОК, КОТОРЫЕ НАХОДИТ ФАЗЗИНГ

на практике фаззинг находит:
   - panic (index out of range, nil pointer)
   - бесконечные циклы
   - утечки памяти (memory leak)
   - утечки горутин (goroutine leak)
   - deadlock и data race

2.3 БИБЛИОТЕКА go-fuzz

что это:
   - сторонняя библиотека от Дмитрия Вьюкова (2015)
   - была стандартом ДО Go 1.18
   - использует coverage-guided fuzzing

установка:
   go install github.com/dvyukov/go-fuzz/go-fuzz@latest
   go install github.com/dvyukov/go-fuzz/go-fuzz-build@latest

2.4 СТРУКТУРА ФАЗЗ-ТЕСТА ДЛЯ go-fuzz

основная функция:
   func Fuzz(data []byte) int {
       // обрабатываем data
       return 1 // интересные данные (новое покрытие)
       return 0 // нейтральные данные
       return -1 // не добавлять в корпус
   }

чтобы сообщить об ошибке — просто паникуем:
   func Fuzz(data []byte) int {
       if isBug(data) {
           panic("found a bug!")
       }
       return 0
   }

2.5 ЗАПУСК go-fuzz

этапы:
   go-fuzz-build                    # компиляция с инструментацией
   go-fuzz -workdir=./workdir       # запуск фаззинга
   go-fuzz -http=:8080              # с веб-интерфейсом

флаги:
   -procs N       — количество процессов (все CPU по умолчанию)
   -timeout=d     — таймаут на один тест (1 секунда по умолчанию)

2.6 ВЫВОД go-fuzz (РАСШИФРОВКА)

пример:
   workers: 500, corpus: 186, crashers: 3, execs: 12009519 (121224/sec), cover: 2746

расшифровка:
   workers — сколько тестов параллельно
   corpus — количество интересных входов в корпусе
   crashers — количество найденных ошибок
   execs — всего выполнений (скорость в скобках)
   cover — покрытие (чем больше, тем лучше)

2.7 АНАЛИЗ РЕЗУЛЬТАТОВ (CRASHERS)

где лежат найденные ошибки:
   workdir/crashers/
   ├── file              — бинарный вход
   ├── file.quoted       — вход в кавычках (можно скопировать в тест)
   └── file.output       — стек ошибки

2.8 go-fuzz vs НАТИВНЫЙ ФАЗЗИНГ (Go 1.18+)

характеристика       | go-fuzz              | нативный fuzz
---------------------|----------------------|---------------------
появление            | 2015                 | 2022
установка            | отдельно (go install)| встроен в go test
формат данных        | только []byte        | любые типы
запуск               | go-fuzz              | go test -fuzz=FuzzXxx

когда выбирать go-fuzz:
   - старый код, написанный под go-fuzz
   - нужны специфические возможности go-fuzz
   - работаешь с OSS-Fuzz (есть готовая интеграция)

когда выбирать нативный fuzz:
   - go 1.18+ (почти всегда сейчас)
   - хочешь фаззить данные разных типов (int, string)
   - не хочешь устанавливать доп инструменты
*/

// ТЕСТИРУЕМЫЕ ФУНКЦИИ (С НАМЕРЕННЫМИ БАГАМИ)

// баг 1: деление на ноль
func safeDivide(a, b int) int {
	// баг: не проверяет b == 0
	return a / b
}

// баг 2: неправильная обработка пустой строки
func firstChar(s string) string {
	// баг: panic при пустой строке
	return string(s[0])
}

// ПРИМЕР 1: ФАЗЗ-ТЕСТ НАХОДИТ ДЕЛЕНИЕ НА НОЛЬ
func fuzzDivision(f *testing.F) {
	// seed corpus: нормальные значения
	f.Add(10, 2)
	f.Add(100, 5)
	f.Add(0, 1)

	f.Fuzz(func(t *testing.T, a, b int) {
		// ограничиваем диапазон для скорости
		if a < -10000 || a > 10000 || b < -10000 || b > 10000 {
			t.Skip()
		}

		// ловим панику
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic in safeDivide(%d, %d): %v", a, b, r)
			}
		}()
		result := safeDivide(a, b)
		_ = result
	})
}

// ПРИМЕР 2: ФАЗЗ-ТЕСТ НАХОДИТ PANIC ПРИ ПУСТОЙ СТРОКЕ
func fuzzFirstChar(f *testing.F) {
	// seed corpus
	f.Add("hello")
	f.Add("a")
	f.Add("world")

	f.Fuzz(func(t *testing.T, s string) {
		// ограничиваем длину строки
		if len(s) > 1000 {
			t.Skip()
		}

		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic in firstChar(%q): %v", s, r)
			}
		}()

		result := firstChar(s)
		_ = result
	})
}

// ПРИМЕР 3: ФАЗЗ-ТЕСТ С НЕСКОЛЬКИМИ ПАРАМЕТРАМИ РАЗНЫХ ТИПОВ
type config struct {
	port     int
	hostname string
	enabled  bool
}

func validateConfig(port int, hostname string, enabled bool) error {
	// баг: не проверяет порт на отрицательные значения
	// баг: не проверяет hostname на пустоту если enabled == true
	if enabled && hostname == "" {
		return fmt.Errorf("hostname required when enabled")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("invalid port: %d", port)
	}
	return nil
}

func fuzzValidateConfig(f *testing.F) {
	// seed corpus
	f.Add(8080, "localhost", true)
	f.Add(22, "example.com", true)
	f.Add(0, "", false)

	f.Fuzz(func(t *testing.T, port int, hostname string, enabled bool) {
		if port < -10000 || port > 100000 {
			t.Skip()
		}
		if len(hostname) > 100 {
			t.Skip()
		}

		err := validateConfig(port, hostname, enabled)

		// если валидация прошла, порт должен быть в диапазоне
		if err == nil {
			if port < 1 || port > 65535 {
				t.Errorf("validation passed but port %d is invalid", port)
			}
			if enabled && hostname == "" {
				t.Errorf("validation passed but enabled with empty hostname")
			}
		}
	})
}

/*
БЛОК 3: НАСТРОЙКА ФАЗЗ-ТЕСТОВ И АНАЛИЗ РЕЗУЛЬТАТОВ

1.1 НАСТРОЙКА ФАЗЗЕРА

основные параметры нативного фаззинга:
   -fuzztime=30s      # время фаззинга (можно 1m, 1h)
   -parallel=4        # количество параллельных воркеров
   -keepfuzzing       # не останавливаться после первой ошибки

важные настройки в коде:
   f.Add(...)                    # seed corpus (стартовые данные)
   if len(data) > 10000 { return } # ограничение размера
   if testing.Short() { t.Skip() } # пропуск в CI при -short

1.2 ОПТИМИЗАЦИЯ ФАЗЗИНГА

как ускорить:
   - не делай тяжёлых операций в фазз-функции
   - используй t.Skip() для слишком больших данных
   - не выделяй много памяти (меньше GC)
   - убери лишние аллокации

как улучшить поиск:
   - больше разнообразных seed в f.Add()
   - добавляй граничные значения (0, -1, пустые строки)
   - используй несколько фазз-тестов вместо одного
*/

// ТЕСТИРУЕМАЯ ФУНКЦИЯ (сложный парсер)

type packetHeader struct {
	Magic   uint32
	Version uint8
	Type    uint16
	Length  uint32
}

func parsePacket(data []byte) (*packetHeader, []byte, error) {
	if len(data) < 11 {
		return nil, nil, fmt.Errorf("too short")
	}

	hdr := &packetHeader{
		Magic:   binary.BigEndian.Uint32(data[0:4]),
		Version: data[4],
		Type:    binary.BigEndian.Uint16(data[5:7]),
		Length:  binary.BigEndian.Uint32(data[7:11]),
	}

	if hdr.Magic != 0xDEADBEEF {
		return nil, nil, fmt.Errorf("bad magic")
	}

	if uint32(len(data)) < 11+hdr.Length {
		return nil, nil, fmt.Errorf("truncated")
	}

	return hdr, data[11 : 11+hdr.Length], nil
}

// ПРИМЕР 1: ПРАВИЛЬНЫЙ ФАЗЗ-ТЕСТ С ОПТИМИЗАЦИЯМИ
func fuzzParsePacket(f *testing.F) {
	// seed corpus: добавляем ВСЕ граничные случаи
	f.Add([]byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x05, 'h', 'e', 'l', 'l', 'o'})
	f.Add([]byte{0xDE, 0xAD, 0xBE, 0xEF, 0x01, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00}) // length=0
	f.Add([]byte{0xDE, 0xAD, 0xBE, 0xEF})                                           // слишком короткий

	f.Fuzz(func(t *testing.T, data []byte) {
		// оптимизация 1: отсекаем гигантские данные
		if len(data) > 10000 {
			t.Skip()
		}

		// оптимизация 2: ловим панику
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic: %v, data len=%d", r, len(data))
			}
		}()

		hdr, payload, err := parsePacket(data)

		// проверка инвариантов
		if err == nil {
			if hdr.Magic != 0xDEADBEEF {
				t.Error("magic mismatch")
			}
			if int(hdr.Length) != len(payload) {
				t.Errorf("length mismatch: %d vs %d", hdr.Length, len(payload))
			}
		}
	})
}

// ПРИМЕР 2: ФАЗЗ-ТЕСТ С НЕСКОЛЬКИМИ ТИПАМИ ДАННЫХ
func fuzzJSONWithMetadata(f *testing.F) {
	// seed: разные сценарии
	f.Add("valid", "{\"key\":\"value\"}")
	f.Add("empty", "{}")
	f.Add("invalid", "not json")
	f.Add("array", "[1,2,3]")

	f.Fuzz(func(t *testing.T, scenario, jsonData string) {
		if len(jsonData) > 5000 {
			t.Skip()
		}

		var result map[string]interface{}
		err := json.Unmarshal([]byte(jsonData), &result)

		// проверка: если это object сценарий, json должен быть object
		if scenario == "valid" && err == nil {
			if _, ok := result["key"]; !ok {
				// не паника, просто логируем
				t.Log("key not found")
			}
		}
	})
}

// ПРИМЕР 4: УМЕНЬШЕНИЕ ШУМА ПРИ ФАЗЗИНГЕ
func fuzzLowNoise(f *testing.F) {
	f.Add([]byte("123"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// ❌ ПЛОХО: создаёт шум
		// fmt.Printf("processing %x\n", data)

		// ✅ ХОРОШО: только важные проверки
		if len(data) > 0 && data[0] == 0xFF {
			// только тут логируем
			t.Log("found 0xFF prefix")
		}

		// проверка без лишних аллокаций
		var result [1024]byte // на стеке, не в куче
		copy(result[:], data)
		_ = result
	})
}

// ПРИМЕР 6: ВОСПРОИЗВЕДЕНИЕ CRASHER ИЗ TESTDATA
/*
когда фаззер нашёл ошибку, сохраняется файл в testdata/fuzz/FuzzParsePacket/
содержимое такого файла можно скопировать и вставить в обычный тест
*/
func testReproduceCrasher(t *testing.T) {
	// данные из testdata/fuzz/FuzzParsePacket/5c6f8e9a7b3d1f2e
	crasherData := []byte{
		0xDE, 0xAD, 0xBE, 0xEF, // magic ok
		0x01,       // version
		0x00, 0x01, // type
		0xFF, 0xFF, 0xFF, 0xFF, // length = 4294967295 (переполнение!)
	}

	_, _, err := parsePacket(crasherData)

	// ожидаем ошибку (паники быть не должно)
	if err == nil {
		t.Error("expected error due to overflow")
	}
}

/*
БЛОК 4: ЭКСПЕРТНОЕ ФАЗЗ-ТЕСТИРОВАНИЕ И БЕЗОПАСНОСТЬ

1.1 ВНУТРЕННЯЯ РАБОТА ФАЗЗЕРА

как фаззер понимает, что данные "интересные":
   - замеряет покрытие кода (code coverage) через инструментацию
   - каждый новый блок кода (if, for, switch) увеличивает счётчик
   - разные пути выполнения = разные значения счётчика
   - go test -cover -coverprofile=c.out показывает покрытие

что такое coverage guidance:
   - фаззер мутирует данные, которые уже показали новое покрытие
   - данные без нового покрытия отбрасываются
   - так фаззер учится обходить все ветки кода

1.2 РАЗРАБОТКА СВОЕГО ФАЗЗЕРА (КОГДА НУЖНО)

когда стандартный фаззер не подходит:
   - нужен специфический формат данных (не просто байты)
   - нужна FSM (конечный автомат) для мутаций
   - нужно фаззить сетевой протокол с состояниями

примеры своих фаззеров:
   - генератор структурированных данных (json, xml)
   - фаззер с сохранением состояния (stateful fuzzing)
   - дифференциальный фаззер (сравнение двух реализаций)

1.3 CI/CD ИНТЕГРАЦИЯ

стратегия запуска фаззинга в CI:
   - каждый PR: быстрый фаззинг (30 секунд) на изменяемом коде
   - nightly: полный фаззинг (1-2 часа) на всём проекте
   - релизный цикл: длительный фаззинг (8+ часов)

как не перегрузить CI:
   - используй -fuzztime=30s для PR
   - ограничивай размер данных через t.Skip()
   - отдельный раннер для фаззинга (не смешивай с юнит-тестами)
   - сохраняй corpus в репозитории (testdata/fuzz/)
*/

// ПРИМЕР 1: ФАЗЗЕР ДЛЯ SQL INJECTION (СТРУКТУРИРОВАННЫЕ ДАННЫЕ)
// генератор структурированных данных для SQL
type sqlFuzzer struct {
	tokens []string
}

func newSQLFuzzer() *sqlFuzzer {
	return &sqlFuzzer{
		tokens: []string{"SELECT", "INSERT", "DELETE", "WHERE", "OR", "AND", "1=1",
			"'", "\"", "--", ";", "UNION", "DROP", "TABLE", "NULL", "TRUE", "FALSE"},
	}
}

func (f *sqlFuzzer) mutate(seed []byte) []byte {
	// мутируем с учётом SQL синтаксиса
	result := make([]byte, len(seed))
	copy(result, seed)

	if len(result) > 0 && rand.Intn(2) == 0 {
		// вставляем SQL токен
		token := f.tokens[rand.Intn(len(f.tokens))]
		pos := rand.Intn(len(result) + 1)
		newResult := make([]byte, 0, len(result)+len(token)+1)
		newResult = append(newResult, result[:pos]...)
		newResult = append(newResult, token...)
		newResult = append(newResult, result[pos:]...)
		result = newResult
	}
	return result
}

// функция, которую тестируем (опасный запрос)
func executeQuery(query string) error {
	// БАГ: не экранирует ввод
	if strings.Contains(query, "DROP") || strings.Contains(query, ";") {
		return fmt.Errorf("potential injection detected: %s", query)
	}
	return nil
}

func fuzzSQLInjection(f *testing.F) {
	fuzzer := newSQLFuzzer()

	// seed
	f.Add([]byte("SELECT * FROM users WHERE id = 1"))

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1000 {
			t.Skip()
		}

		// мутируем с учётом SQL
		mutated := fuzzer.mutate(data)
		query := string(mutated)

		// проверяем опасные паттерны
		err := executeQuery(query)

		// если прошло, но содержит опасный паттерн — баг
		if err == nil {
			dangerous := []string{"';", "--", "OR 1=1", "DROP", "UNION"}
			for _, d := range dangerous {
				if strings.Contains(strings.ToUpper(query), d) {
					t.Errorf("potential injection passed validation: %q", query)
				}
			}
		}
	})
}

// ПРИМЕР 2: ФАЗЗЕР ДЛЯ АСИНХРОННОГО КОДА (CONTEXT)
func asyncProcess(ctx context.Context, data []byte) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(len(data)) * time.Microsecond):
		return nil
	}
}

func fuzzAsync(f *testing.F) {
	f.Add([]byte("test"))

	f.Fuzz(func(t *testing.T, data []byte) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		err := asyncProcess(ctx, data)

		// если данные слишком большие, должен быть таймаут
		if len(data) > 10000 && err == nil {
			t.Errorf("no timeout for large data: %d bytes", len(data))
		}
	})
}
