package main

import (
	"fmt"
	"unsafe"
)

/*
# ТИПЫ В GO: ПОЛНЫЙ ОБЗОР

Этот блок описывает:
- Все встроенные типы Go
- Как они устроены и где используются
- Разницу между алиасами и новыми типами
- Как создавать переменные разных видов
- Что такое builtin-функции и почему они не требуют импорта
*/
/*
// 2. ПОЛНЫЙ СПИСОК ВСТРОЕННЫХ ТИПОВ
//
// Go имеет 25+ встроенных типов, но их можно разделить на категории:
//
// • Булевы:                bool
// • Целочисленные:         int, int8, int16, int32, int64,
//                          uint, uint8, uint16, uint32, uint64, uintptr
// • Алиасы целых:          byte (== uint8), rune (== int32)
// • Вещественные:          float32, float64
// • Комплексные:           complex64, complex128
// • Строки:                string
// • Составные типы:        []T (слайс), [N]T (массив), map[K]V, struct{}, func(...)
// • Указатели:             *T
// • Каналы:                chan T
// • Интерфейсы:            interface{},any
//
// Примечание: все эти типы встроены в компилятор — их не нужно импортировать.
*/

// 1. ПРИМЕРЫ ВСЕХ ОСНОВНЫХ ТИПОВ

func demonstrateAllTypes() {
	// Булевые
	var ok bool = true

	// Целые
	var age int = 25
	var small uint8 = 255
	var unicode rune = 'λ' // Unicode code point
	var b byte = 'A'       // ASCII/байт

	// Вещественные
	var pi float64 = 3.1415926535
	var approx float32 = 3.14

	// Комплексные
	var z complex128 = 1 + 3i

	// Строка
	var name string = "Go"

	// Слайс и Массив
	sl := []int{1, 2, 3, 4, 5, 6, 7, 8}
	arr := [4]int{1, 2, 3, 4}

	// Мапа
	counts := map[string]int{"apple": 3}

	// Функция как тип
	sum := func(x, y int) int { return x + y }

	// Указатель
	ptr := &age

	// Канал
	ch := make(chan int)

	// Интерфейс
	var int1 interface{} = 2
	var int2 any = "Пеписи Кола"

	// Вывод для проверки
	fmt.Printf("small=%d,approx=%.2f,b=%x", small, approx, b)
	fmt.Printf("string=%s", name)
	fmt.Printf("ok=%v, age=%d, rune=%c, pi=%.2f, z=%v\n", ok, age, unicode, pi, z)
	fmt.Printf("slice=%v, array=%v, counts=%v\n", sl, arr, counts)
	fmt.Println("sum(5) =", sum(5, 6))
	fmt.Println("Ptr to age:", *ptr)
	go func() { ch <- 42 }()
	fmt.Println("From channel:", <-ch)
	fmt.Println("Any №1:", int1)
	fmt.Println("Any №2:", int2)
}

// 2. АЛИАСЫ ТИПОВ И СОЗДАНИЕ ПЕРЕМЕННЫХ
/*
В Go есть два способа создать «новое имя» для типа:

1. АЛИАС (type T = U):
   - Это просто другое имя для существующего типа.
   - T и U — один и тот же тип.
   - Можно свободно присваивать значения без преобразования.
   - Нельзя добавлять методы к алиасу.

2. НОВЫЙ ТИП (type T U):
   - Создаётся новый, уникальный тип.
   - T и U — разные типы, даже если имеют одинаковое представление.
   - Требуется явное преобразование: T(u).
   - Можно определять собственные методы.

Почему это важно?
- Совместимость при вызовах функций
- Возможность расширять поведение через методы
- Безопасность: новые типы предотвращают случайную подстановку
*/

// 3.ПРАКТИЧЕСКИЕ ПРИМЕРЫ АЛИАСОВ VS НОВЫХ ТИПОВ

// Алиас: просто синоним
type myAlias = string
type Celsius = float64
type Fahrenheit = float64

var c Celsius = 105.3

func convert(c Celsius) Fahrenheit {
	return c
}

//func (m myAlias) ToUpper() string {
//	return strings.ToUpper(string(m))
//} --- Ошибка

// Новый тип: создаёт изолированную сущность
type Kelvin float64
type UsID string

var k Kelvin = 273.15

func (k Kelvin) String() string {
	return fmt.Sprintf("%.2f K", k)
}
func (id UsID) IsValid() bool {
	return len(id) > 0
}

func demonstrateTypeCreation() {
	temp := Celsius(20.0)
	fmt.Println("Fahrenheit:", convert(temp)) // Работает без проблем

	id := UsID("user123")
	fmt.Println("Valid ID?", id.IsValid()) // true

	// Алиас работает как обычная строка
	var alias myAlias = "test"
	fmt.Println("Alias length:", len(alias))
}

// 4. BUILTIN ПАКЕТ: ЧТО ЭТО И КАК РАБОТАЕТ?
/*
В Go есть набор встроенных (builtin) функций, которые:
- Доступны в любом месте программы без импорта
- Реализованы на уровне компилятора
- Не являются частью какого-либо пакета (хотя документация иногда ссылается на "builtin")

Полный список builtin-функций:

• len(x)      — длина массива, слайса, строки, мапы, канала
• cap(x)      — ёмкость слайса или буферизованного канала
• make(t, ...)— создаёт слайс, мапу или канал
• new(t)      — выделяет память под тип t, возвращает *t
• append(s, ...) — добавляет элементы в слайс
• copy(dst, src) — копирует данные между слайсами
• delete(m, k)   — удаляет ключ из мапы
• close(c)       — закрывает канал
• panic(v)       — вызывает панику
• recover()      — восстанавливает выполнение после паники
• complex(r, i)  — создаёт комплексное число
• real(c), imag(c) — получают вещественную/мнимую часть
• print(), println() — низкоуровневый вывод (не для продакшена!)

ВАЖНО:
- `builtin` — это не настоящий пакет. Попытка `import "builtin"` вызовет ошибку.
- Эти функции нельзя переопределить или заменить.
- Они работают только с определёнными типами (например, `len` не работает с int).

Примеры использования:
*/
func demonstrateBuiltin() {
	s := make([]int, 2, 10) // len=2, cap=10
	s = append(s, 3)        // теперь len=3
	copy(s[1:], s[:2])      // копируем первые 2 элемента в позиции 1-2

	m := make(map[string]int)
	m["key"] = 1
	delete(m, "key") // удаляем

	ch := make(chan int, 1)
	ch <- 1
	close(ch)

	//recover() — ловит панику (только внутри defer!)
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered from panic:", r)
		}
	}()

	//  complex(r, i) — создаёт комплексное число
	c := complex(3.0, 4.0) // 3 + 4i
	fmt.Println("complex:", c)

	// real(c), imag(c) — части комплексного числа
	fmt.Printf("real=%.0f, imag=%.0f\n", real(c), imag(c)) // 3, 4

	// len/cap работают с разными типами
	fmt.Println("Slice len:", len(s), "cap:", cap(s))
	fmt.Println("Map len:", len(m))
	fmt.Println("Chan len:", len(ch)) // количество элементов в буфере
}

func main() {
	demonstrateAllTypes()
	demonstrateTypeCreation()
	demonstrateBuiltin()

	// Проверка размеров (для любознательных)
	fmt.Printf("\nSize of bool: %d\n", unsafe.Sizeof(true))
	fmt.Printf("Size of int: %d\n", unsafe.Sizeof(int(0)))
	fmt.Printf("Size of string: %d\n", unsafe.Sizeof("hello"))
}
