package generics

import (
	"cmp"
	"context"
	"fmt"
	"io"
	"slices"
	"sync"
)

//1.ОСНОВЫ ОБОБЩЕНИЙ / GENERICS
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:
1. Зачем нужны Generics?
До версии 1.18 в Go был выбор: либо дублировать код для каждого типа (int, string),
либо использовать interface{}, что приводило к потере типизации и ошибкам в рантайме.
Дженерики позволяют писать "шаблоны" кода, которые работают с разными типами безопасно.

2. Основные понятия:
- Type Parameters (Параметры типа): Переменная-тип, указанная в квадратных скобках [T any].
- Type Constraints (Ограничения): Интерфейс, определяющий, какие типы может принять T (например, any).
- Type Arguments (Аргументы типа): Конкретный тип (int, string), подставленный при вызове.
- Instantiation (Инстанцирование): Создание компилятором конкретной версии функции для конкретного типа.

3. any — это алиас для пустого интерфейса interface{}. Используется как самое широкое ограничение.
*/

// ПРИМЕР 1: Универсальная функция для фильтрации слайса
// [T any] позволяет функции работать со слайсом любого типа.
func demFilter[T any](slice []T, test func(T) bool) []T {
	var result []T
	for _, v := range slice {
		if test(v) {
			result = append(result, v)
		}
	}
	return result
}

// ПРИМЕР 2: Обобщенная структура (Стэк)
// Структура хранит данные типа T, который определится при создании экземпляра.

type stack[T any] struct {
	items []T
}

func (s *stack[T]) Push(value T) {
	s.items = append(s.items, value)
}

func (s *stack[T]) Pop() T {
	if len(s.items) == 0 {
		var zero T
		return zero
	}
	item := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return item
}

// ПРИМЕР 3: Обобщенный Map (Преобразование данных)
// Функция принимает слайс одного типа (F) и превращает его в слайс другого типа (T).
func demMapSlice[F any, T any](input []F, transform func(F) T) []T {
	result := make([]T, len(input))

	for i, v := range input {
		result[i] = transform(v)
	}
	return result
}

// ПРИМЕР 4: Демонстрация осознания унификации (Интерфейсы + Дженерики)
type stringer interface {
	String() string
}

// Функция работает с любым типом, который умеет превращать себя в строку.
func PrintAll[T stringer](items []T) {
	for _, item := range items {
		fmt.Println(item.String())
	}
}

//2.ГЛУБОКОЕ ПОНИМАНИЕ И СТАНДАРТ
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:
1. Семантика (Как это работает):
При компиляции Go использует гибридный подход. Для разных типов создаются разные
экземпляры функций (мономорфизация), но если типы похожи (например, все указатели),
они могут переиспользовать один и тот же код, чтобы бинарник не раздувался.

2. Стандартные контейнеры и sync.Map:
Важно понимать, что старые части стандартной библиотеки (как sync.Map) НЕ были
переписаны на дженерики ради обратной совместимости. Поэтому на этом уровне
мы должны уметь делать безопасные обертки.

3. sort.Slice vs slices.Sort:
sort.Slice работает через interface{} и рефлексию (медленно).
slices.Sort использует дженерики (быстро). На этом уровне ты должен знать,
что второй вариант предпочтительнее.
*/

// ПРИМЕР 1: Типизированная обертка над sync.Map
// Позволяет избежать постоянных приведений типов val.(string) и ошибок в рантайме.
type store[K comparable, V any] struct {
	data sync.Map
}

func (s *store[K, V]) Save(key K, val V) {
	s.data.Store(key, val)
}
func (s *store[K, V]) Get(key K) (V, bool) {
	res, ok := s.data.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return res.(V), true
}

// ПРИМЕР 2: Использование пакета 'slices' (Go 1.21+)
// Вместо написания своих функций поиска или сортировки, используем стандарт.
func demSlicesStandartExample() {
	numbers := []int{1, 6, 3, 91, 23, 55, 4, 59}

	// 1. Быстрая сортировка (Generics внутри)
	slices.Sort(numbers)

	// 2. Бинарный поиск (работает только на отсортированных данных)
	found, _ := slices.BinarySearch(numbers, 23)

	// 3. Проверка наличия
	hasTen := slices.Contains(numbers, 3)

	fmt.Printf("Nums: %v, Index of 8: %d, Has 10: %v\n", numbers, found, hasTen)
}

// ПРИМЕР 3: Сложная сортировка объектов (slices.SortFunc)
// Когда нужно сортировать не просто числа, а структуры по полю.
type Employee struct {
	name   string
	salary int
}

func demSortEmplExample() {
	staff := []Employee{
		{"Ivan", 50000},
		{"Oleg", 120000},
		{"Anna", 80000},
	}

	slices.SortFunc(staff, func(a, b Employee) int {
		return cmp.Compare(a.salary, b.salary)
	})
	fmt.Println("Отсортированный штат:", staff)
}

// ПРИМЕР 4: Обобщенный интерфейс
// Ты должен понимать, что интерфейсы тоже могут быть дженериками.
type Validator[T any] interface {
	Validate(value T) error
}
type StringValidator struct{}

func (sv StringValidator) Validate(v string) error {
	if len(v) == 0 {
		return fmt.Errorf("empty string")
	}
	return nil
}

// ПРИМЕР 5: Паттерн "Result" (часто встречается в Rust/Swift, теперь и в Go)
// Помогает возвращать либо результат, либо ошибку в одной структуре.
type Result[T any] struct {
	data  T
	error error
}

func NewResult[T any](data T, err error) Result[T] {
	return Result[T]{
		data:  data,
		error: err,
	}
}

func demExampleResultUsage() {
	// Пример: работаем со слайсом результатов обработки
	results := []Result[string]{
		NewResult("Success 1", nil),
		NewResult("", fmt.Errorf("network error")),
	}

	for _, res := range results {
		if res.error != nil {
			fmt.Println("Ошибка:", res.error)
			continue
		}
		fmt.Println("Данные:", res.data)
	}
}

// ПРИМЕР 6: Универсальный Set (Множество) через дженерики
// До дженериков мы писали map[string]struct{}, теперь это можно обернуть в тип.
type Set[T comparable] struct {
	m map[T]struct{}
}

func NewSet[T comparable]() *Set[T] {
	return &Set[T]{m: make(map[T]struct{})}
}

func (s *Set[T]) Add(v T) {
	s.m[v] = struct{}{}
}

func (s *Set[T]) Has(v T) bool {
	_, ok := s.m[v]
	return ok
}

// ПРИМЕР 7: Сравнение и выбор (Использование пакета cmp)
// Напишем функцию, которая находит минимальное и максимальное значение одновременно.
func MinMax[T cmp.Ordered](slice []T) (T, T) {
	var min, max T
	if len(slice) == 0 {
		return min, max
	}
	min, max = slice[0], slice[0]
	for _, v := range slice {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

//3.ADVANCED CONSTRAINTS & ALGORITHMS
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:
1. Type Sets (Интерфейсы как наборы типов):
В Middle-Go интерфейс — это не просто список методов, это "набор типов".
Union (|): Позволяет перечислить конкретные типы (int | string).
Approximation (~): Тильда разрешает использовать кастомные типы на базе основных.
Без тильды `type MyID int` НЕ подойдет под ограничение `int`.

2. Алгоритмы vs Интерфейсы:
Дженерики подходят для алгоритмов, где логика одинакова для всех типов (сортировка, поиск).
Интерфейсы подходят, когда поведение типов разное (метод Draw() у Круга и Квадрата).

3. Когда НЕ использовать дженерики:
- Если вам нужно просто вызвать метод у разных типов (используйте обычные интерфейсы).
- Если реализация алгоритма для каждого типа должна быть разной.
- Если это усложняет чтение кода без реальной выгоды в производительности.
*/

// 3.1. ТИЛЬДА И КАСТОМНЫЕ ТИПЫ (Type Sets)
type myInt int

// Numeric — ограничение, включающее базовые типы и их производные (~)
type numeric interface {
	~int | ~int64 | ~float64
}

func Sum[T numeric](a, b T) T {
	return a + b
}
func demExampleTilde() {
	var x, y myInt = 5, 12
	fmt.Println(Sum(x, y))
}

// 3.2. ПРОДВИНУТЫЕ КОНТЕЙНЕРЫ
type eventBus[T any] struct {
	subs []chan T
}

func (eb *eventBus[T]) Subscribe() chan T {
	ch := make(chan T, 1)
	eb.subs = append(eb.subs, ch)
	return ch
}
func (eb *eventBus[T]) Publish(event T) {
	for _, ch := range eb.subs {
		select {
		case ch <- event:
		default: // Не блокируемся, если подписчик тормозит
		}
	}
}

// 3.3. АЛГОРИТМЫ: АСИНХРОННЫЙ ПАРАЛЛЕЛЬНЫЙ МАППЕР
// Показывает умение работать с горутинами, контекстом и дженериками одновременно.
func demParallelMap[F any, T any](ctx context.Context, input []F, fn func(F) T) []T {
	resCh := make(chan struct {
		idx int
		val T
	}, len(input))

	for i, v := range input {
		go func(i int, val F) {
			resCh <- struct {
				idx int
				val T
			}{idx: i, val: fn(val)}
		}(i, v)
	}

	result := make([]T, len(input))
	for i := 0; i < len(input); i++ {
		select {
		case <-ctx.Done():
			return nil
		case res := <-resCh:
			result[res.idx] = res.val
		}
	}
	return result
}

// 3.4. КРИТЕРИИ ВЫБОРА: ДЖЕНЕРИКИ VS ИНТЕРФЕЙСЫ
// ПЛОХО (Overengineering):
// Мы не используем свойства T, просто вызываем Close. Тут T только мешает.
func CloseAllBad[T io.Closer](items []T) {
	for _, item := range items {
		item.Close()
	}
}

// ХОРОШО:
// Обычный интерфейс короче и работает быстрее (меньше работы компилятору).
func CloseAllGood(items []io.Closer) {
	for _, item := range items {
		item.Close()
	}
}

// ХОРОШО (Дженерик):
// Нам нужно вернуть ИМЕННО тот тип, что пришел, без приведения типов.
func Identity[T any](v T) T {
	return v
}

//4. АРХИТЕКТУРА, ЭКСПЛУАТАЦИЯ И ВНУТРЕННИЕ ТЕХНОЛОГИИ
/*
ТЕОРЕТИЧЕСКАЯ СПРАВКА:

1. Реализация: GCShape и Диктофоны (Dictionaries):
Go не делает полную мономорфизацию (как C++), чтобы не раздувать бинарник.
Он использует "GCShape": типы с одинаковым "видом" для сборщика мусора (например,
все указатели) используют ОДНУ И ТУ ЖЕ реализацию функции. Для различения типов
в рантайме передается невидимый параметр — статическая таблица (Dictionary).

2. Влияние на производительность:
- Вызов дженерик-функции может быть чуть медленнее прямого вызова из-за передачи словаря.
- НО это почти всегда быстрее, чем работа через interface{}, так как нет аллокаций
в куче (boxing) и динамических проверок типов (type assertion).

3. Дизайн библиотек:
разработчик использует дженерики для создания "ортогональных" API,
где логика хранения данных отделена от логики самих данных.
*/

// 4.1. АРХИТЕКТУРНЫЙ ПАТТЕРН: ТИПИЗИРОВАННЫЙ MIDDLEWAR
// Позволяет строить конвейеры обработки данных с сохранением типов.
type Handler[I any, O any] func(context.Context, I) (O, error)
type Middleware[I any, O any] func(Handler[I, O]) Handler[I, O]

func WithLogging[I any, O any](next Handler[I, O]) Handler[I, O] {
	return func(ctx context.Context, input I) (O, error) {
		fmt.Printf("Log: Input received: %v\n", input)
		return next(ctx, input)
	}
}

// 4.2. СЛОЖНЫЕ СТРУКТУРЫ ДАННЫХ: ОБОБЩЕННЫЙ GRAPH (Граф) ---
type edge[T comparable] struct {
	From, To *vertex[T]
	Weight   int
}

type vertex[T comparable] struct {
	Value T
}

type graph[T comparable] struct {
	Vertices []*vertex[T]
	Edges    []*edge[T]
	mu       sync.RWMutex
}

// Добавление вершины с проверкой уникальности через сопоставимость (comparable)
func (g *graph[T]) AddVertex(v T) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.Vertices = append(g.Vertices, &vertex[T]{Value: v})
}

// 4.3. ОПТИМИЗАЦИЯ: POOL ОБОБЩЕННЫХ ОБЪЕКТОВ (Zero-Allocation)
// Стандартный sync.Pool работает с any. Обертка-дженерик убирает накладные расходы на cast.

type ObjectPool[T any] struct {
	pool sync.Pool
}

func NewObjectPool[T any](allocator func() T) *ObjectPool[T] {
	return &ObjectPool[T]{
		pool: sync.Pool{
			New: func() any { return allocator() },
		},
	}
}
func (p *ObjectPool[T]) Get() T {
	return p.pool.Get().(T) // Cast внутри обертки, клиент кода об этом не знает
}
func (p *ObjectPool[T]) Put(x T) {
	p.pool.Put(x)
}

// 4.4. ADVANCED: ОГРАНИЧЕНИЯ НА ОСНОВЕ СТРУКТУРНЫХ ТИПОВ
// Мы можем требовать, чтобы тип не только имел методы, но и был, например, мапой.
type mapTransformer[K comparable, V any, M ~map[K]V] func(M)

func clearMap[K comparable, V any, M ~map[K]V](m M) {
	for k := range m {
		delete(m, k)
	}
}

// 4.5. GENERIC REPOSITORY (Абстракция над БД) ---
// Позволяет не писать CRUD операции для каждой новой таблицы.
type Entity interface {
	GetID() int64
}
type Repository[T Entity] struct {
	mu   sync.RWMutex
	data map[int64]T
}

func (r *Repository[T]) Save(item T) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.data == nil {
		r.data = make(map[int64]T)
	}
	r.data[item.GetID()] = item
}

func (r *Repository[T]) FindByID(id int64) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	val, ok := r.data[id]
	return val, ok
}
