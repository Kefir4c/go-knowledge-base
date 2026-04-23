package main

import (
	"fmt"
	"slices"
	"sync"
	"time"
)

/*
# МАПЫ В GO:

Этот блок охватывает:
- Что такое мапы и как они устроены
- Внутреннее устройство хеш-таблицы
- Требования к ключам и нюансы совместимости
- Производительность и оптимизации
- Безопасность в конкурентной среде

Цель: дать полное понимание мап — от базового использования до продвинутых техник оптимизации.
*/

// 1. ЧТО ТАКОЕ МАПА?

/*
ЧТО ЭТО:
- Мапа — это ассоциативный массив (ключ-значение), реализованный как хеш-таблица.
- Позволяет быстро получать значения по ключу (в среднем O(1)).
- Неупорядочена (порядок итерации не гарантируется!).

ПОЧЕМУ ЭТО ВАЖНО:
- Мапы — основной способ хранения данных с быстрым доступом по ключу.
- Неправильное использование приводит к паникам (nil-мапа) или неожиданному поведению.

ПРИМЕР БАЗОВОГО ИСПОЛЬЗОВАНИЯ:
*/

/*
РАЗНИЦА МЕЖДУ make И ЛИТЕРАЛОМ?

1. make(map[K]V) — создаёт пустую, но инициализированную** мапу.
  - Внутри уже выделена память под хеш-таблицу (обычно на 8 бакетов).
  - Готова к работе: можно сразу писать m["key"] = value.
  - Аналогично: var m map[K]V; m = make(map[K]V)

2. map[K]V{...} — литерал **сразу с данными**.
  - Go требует, чтобы при создании через литерал ты указал хотя бы одну пару ключ-значение.
  - Пустой литерал map[K]V{} — это тоже валидно! Он создаёт **инициализированную, но пустую** мапу.

ВАЖНОЕ УТОЧНЕНИЕ:
- map[string]int{} — это НЕ nil-мапа! Это готовая к работе мапа без элементов.
- var m map[string]int — это nil-мапа! Запись в неё вызовет панику.

ПРИМЕРЫ:
*/
func demBasicMap() {
	// 1. Через make — пустая, но готовая
	m1 := make(map[string]int)
	m1["a"] = 1 // OK

	// 2. Через литерал с данными
	m2 := map[string]int{"b": 2}
	m2["c"] = 3 // OK

	// 3. Через ПУСТОЙ литерал — тоже OK!
	m3 := map[string]int{}
	m3["d"] = 4 // OK

	// 4. Nil-мапа — ОПАСНО!
	//var m4 map[string]int  /nil
	// m4["e"] = 5 // ← ПАНИКА!
	fmt.Println(m1, m2, m3)

	// Доступ по ключу
	value, exists := m1["a"]
	fmt.Printf("Value: %d, Exists: %v\n", value, exists) // 1, true

	// Удаление ключа
	delete(m1, "a")
}

/*
### КЛЮЧИ В МАПЕ

МАПА ТРЕБУЕТ, ЧТОБЫ КЛЮЧ БЫЛ **СРАВНИМЫМ (comparable)**.

МОЖНО ИСПОЛЬЗОВАТЬ:
- Все базовые типы: bool, int, uint, float, string, uintptr
- Указатели: *T
- Каналы: chan T
- Интерфейсы: interface{} (если содержащийся тип comparable)
- Структуры: struct{...} (если все поля comparable)
- Массивы: [N]T (если T comparable)

НЕЛЬЗЯ ИСПОЛЬЗОВАТЬ:
- Слайсы: []T
- Мапы: map[K]V
- Функции: func(...)
- Срезы любого вида (включая []byte)

ПОЧЕМУ?
- Go использует операторы `==` и `!=` для сравнения ключей при поиске.
- Для слайсов, мап и функций эти операторы **не определены** → компилятор запрещает их использование.

ПРИМЕРЫ:
*/
func demonstrateMapKeys() {
	// ✅ Валидные ключи
	_ = map[string]int{}
	_ = map[int]bool{}
	_ = map[[2]string]bool{}           // массив — можно!
	_ = map[struct{ a int }][]string{} // структура с comparable полями — можно!

	// Невалидные ключи — ОШИБКА КОМПИЛЯЦИИ:
	// _ = map[[]string]int{}     // слайс — нельзя!
	// _ = map[map[string]int]int{} // мапа — нельзя!
	// _ = map[func()]int{}       // функция — нельзя!
}

// 2. ПРОИЗВОДИТЕЛЬНОСТЬ
/*
СКОРОСТЬ ДОСТУПА ПО КЛЮЧУ:
- В среднем O(1) (постоянное время).
- В худшем случае O(n) (при коллизиях хешей).

ПРИМЕР С ИЗМЕРЕНИЕМ ПРОИЗВОДИТЕЛЬНОСТИ:
*/

func demMapPerf() {
	const size = 1_000_000
	m := make(map[int]bool, size)

	for i := 0; i < size; i++ {
		m[i] = true
	}

	start := time.Now()
	for i := 0; i < size; i++ {
		_ = m[i]
	}
	fmt.Printf("Access time: %v\n", time.Since(start))
}

// 4. КОНКУРЕНТНОСТЬ И BEST PRACTICES
/*
ПРОБЛЕМА: МАПЫ НЕ ПОТОКОБЕЗОПАСНЫ
- Параллельная запись/чтение в мапу вызывает **data race**.
- Решение: Использовать `sync.RWMutex` или `sync.Map`.

ПРИМЕР БЕЗОПАСНОГО ДОСТУПА:
*/
var (
	mu sync.RWMutex
	m  = make(map[string]int)
)

func safeWrite(key string, value int) {
	mu.Lock()
	defer mu.Unlock()
	m[key] = value
}

func safeRead(key string) (int, bool) {
	mu.RLock()
	defer mu.RUnlock()
	value, exists := m[key]
	return value, exists
}

/*
BEST PRACTICES:
1. Всегда инициализируй мапу через `make` (никогда не используй `new`).
2. Проверяй существование ключа через `value, exists := m[key]`.
3. Используй `map[key] = value` для обновления, а не `m[key] = m[key] + 1` (если значение может быть nil).
4. Для сортировки по ключам → создавай слайс ключей и сортируй его.
*/

func demBestPractices() {
	m := map[string]int{"c": 3, "a": 1, "b": 2}
	keys := make([]string, len(m))

	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		fmt.Printf("%s: %d\n", k, m[k])
	}
}

// 5. ВАЖНЫЕ ПРЕДУПРЕЖДЕНИЯ И АНТИПАТТЕРНЫ
/*
1. НИКОГДА не используй `new(map[string]int)`
   → Это создаёт nil-мапу, и запись в неё вызовет панику.
Пример антипаттерна:
*/
/*
func bedMapInit(){
	m := new(map[string]int)
	m["key"] = 42
}
*/
/*
2. МАПЫ НЕУПОРЯДОЧЕНЫ
   → Не полагайся на порядок итерации — он может меняться между запусками.

Пример проблемы:
*/
func unorderedMap() {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	for k, v := range m {
		fmt.Printf("%s: %d\n", k, v) // Порядок не гарантирован!
	}
}

/*
 3. УТЕЧКИ ПАМЯТИ ПРИ УДАЛЕНИИ КЛЮЧЕЙ
    Удаление ключа не освобождает память.
    Используй `m = make(map[K]V)` для полного сброса.

Пример:
*/
func fixMapLeak() {
	var size = 1000000
	bigMap := make(map[string]int, size)
	// ... заполняем bigMap
	// Удаляем все ключи
	for k := range bigMap {
		delete(bigMap, k)
	}
	// bigMap всё ещё занимает память!
	// Правильно: bigMap = make(map[string]int)
}

// 8. ПОДВОДНЫЕ КАМНИ И НЕОЧЕВИДНЫЕ ПОВЕДЕНИЯ
/*
1. НЕОЖИДАННОЕ ПОВЕДЕНИЕ С НУЛЕВЫМ ЗНАЧЕНИЕМ:
   - Если значение — структура, то её нулевое значение будет использовано при отсутствии ключа.
Пример:
*/
func zeroValueBehavior() {
	type User struct {
		Name string
		age  int
	}
	m := make(map[int]User)
	// При отсутствии ключа вернётся User{Name: "", Age: 0}
	user := m[1]
	fmt.Printf("User: %+v\n", user)
}

/*
2. МАПЫ И ОТСУТСТВИЕ КЛЮЧА:
  - Если ключа нет, вернётся нулевое значение типа значения.

Пример:
*/
func keyNotExists() {
	m := map[string]int{"a": 1}
	value, exists := m["b"]
	fmt.Printf("Value: %d, Exists: %v\n", value, exists)
}

/*
3. КАК ПРОВЕРИТЬ, ЧТО МАПА ПУСТАЯ:
  - Проверяй длину, а не nil-состояние.

Пример:
*/
func checkEmptyMap() {
	m := make(map[string]int)
	if len(m) == 0 {
		fmt.Println("Мапа пустая")
	}
}

func main() {
	demBasicMap()
	demonstrateMapKeys()
	demMapPerf()
	demBestPractices()
	unorderedMap()
	fixMapLeak()
	zeroValueBehavior()
	keyNotExists()
	checkEmptyMap()
}
