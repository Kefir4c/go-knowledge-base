package deepgo

import (
	"fmt"
	"sync"

	"golang.org/x/exp/constraints"
)

//ДЖЕНЕРИКИ В GO: ПОЛНЫЙ РАЗБОР

/*
1. Что такое дженерики (типовые параметры)
Дженерики — это способ писать код, который работает
с разными типами, не повторяя его для каждого типа.
*/

// Без дженериков: нужно писать отдельные функции для каждого типа
func MaxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func MaxFloat(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func Max[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

/*
2. Синтаксис дженериков
Функция с типовым параметром:
*/

func MyFunc[T any](value T) T {
	return value
}

/*
// Вызов с явным указанием типа (редко)
result := MyFunc[int](42)

// Вызов с неявным выводом типа (чаще)
result := MyFunc(42)  // T выводится как int
*/

//Структура с типовым параметром:

type Box[T any] struct {
	Value T
}

func (b Box[T]) Get() T {
	return b.Value
}

/*
Использование
intBox := Box[int]{Value: 42}
strBox := Box[string]{Value: "hello"}
*/

/*
3. Constraints (ограничения)
Constraint — это интерфейс, который определяет, какие
типы можно использовать в качестве типового параметра.

any — любой тип
comparable — типы, которые можно сравнивать
*/

func IsEqual[T comparable](a, b T) bool {
	return a == b // допустимо, только если T имеет ограничение comparable
}

/*
// Работает с int, string, bool, указателями, но не с слайсами
IsEqual(1, 1)      // true
IsEqual("a", "b")  // false
*/

/*
Тип	Реализует comparable?
int, float64, bool, string	                               ✅ Да
pointer (*T)	                                           ✅ Да
array ([2]int)	                                           ✅ Да
slice ([]int)	                                           ❌ Нет
map	                                                       ❌ Нет
struct со всеми comparable полями	                       ✅ Да
struct с полем-слайсом	                                   ❌ Нет
*/

//constraints.Ordered — упорядоченные типы (Go 1.21+)

func xMax[T constraints.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

// T может быть: int, float64, string и т.д. (все, что поддерживает >, <, >=, <=)

/*
Тип	Входит в Ordered?
int, int32, int64	                                       ✅
float32, float64	                                       ✅
string	                                                   ✅
bool	                                                   ❌ (не поддерживает >)
slice, map	                                               ❌
*/

/*
4. Типовые set'ы (type sets)
Типовой set — это способ ограничить типовой параметр набором конкретных типов.
*/

// Типовой set для чисел
type Number interface {
	int | int64 | float32 | float64
}

func Sum[T Number](a, b T) T {
	return a + b // работает, потому что все типы из set поддерживают +
}

// Типовой set для строк и целых чисел
type StringOrInt interface {
	string | int
}

func Format[T StringOrInt](value T) string {
	return fmt.Sprintf("%v", value)
}

//5. Когда дженерики уместны

/*
1. Контейнерный структуры - Работают с любыми типами данных
2. Алгоритмы над слайсами - Логика не зависит от типа данных
3. Утилитарные функции - сравнение, сортировка, поиск
4. Щбработка результатов/ ошибок.
5. Пуллы объектов - Переиспользования любых типов.
*/

// 1. Контейнер
type Stack[T any] struct {
	items []T
}

func (s *Stack[T]) Push(item T) {
	s.items = append(s.items, item)
}

// 2. Алгоритм
func Map[T any, R any](slice []T, f func(T) R) []R {
	result := make([]R, len(slice))
	for i, v := range slice {
		result[i] = f(v)
	}
	return result
}

// 3. Утилита
func Contains[T comparable](slice []T, value T) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

//6. Когда дженерики НЕ уместны
/*
1. Бизнес-логика - Типы обычно фиксированы(User, Order)
2. Вызов методов - Дженерики не позволяют вызывать метода(Нужны интерфейсы)
3. Разные реализации одного интерфейса - Достаточно интерфейса.
4. Простой код - Дженерики усложняют чтение кода.
*/

//7. Практический пример: универсальный кэш

type Cache[T any] struct {
	data map[string]T
	mu   sync.RWMutex
}

func NewCache[T any]() *Cache[T] {
	return &Cache[T]{
		data: make(map[string]T),
	}
}

func (c *Cache[T]) Set(key string, value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = value
}

func (c *Cache[T]) Get(key string) (T, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.data[key]
	return val, ok
}
