package __5_slice_arr_map

import (
	"fmt"
	"sort"
)

//УДАЛЕНИЕ ЭЛЕМЕНТА ИЗ СЛАЙСА (НЕСКОЛЬКО СПОСОБОВ)

// Способ 1: Сохранить порядок (медленный, O(n))
// Сдвигаем элементы влево
func removeOrdered[T any](sl []T, i int) []T {
	if i < 0 || i >= len(sl) {
		return sl
	}
	copy(sl[i:], sl[i+1:])
	return sl[:len(sl)-1]
}

// Способ 2: Без сохранения порядка (быстрый, O(1))
// Меняем с последним и отрезаем
func removeUnordered[T any](sl []T, i int) []T {
	if i < 0 || i >= len(sl) {
		return sl
	}
	sl[i] = sl[len(sl)-1]
	return sl[:len(sl)-1]
}

// Способ 3: С выделением новой памяти (без мутаций)
func removeNewSlice[T any](s []T, i int) []T {
	if i < 0 || i >= len(s) {
		return s
	}

	result := make([]T, len(s)-1)
	result = append(result, s[:i]...)
	result = append(result, s[i+1:]...)
	return result
}

// ДОБАВЛЕНИЕ В НАЧАЛО СЛАЙСА
func prependNew[T any](s []T, elem T) []T {
	return append([]T{elem}, s...)
}

// Способ 2: in-place с сдвигом (переиспользует память, если хватает capacity)
func prependInPlace[T any](s []T, elem T) []T {
	s = append(s, elem) // добавляем в конец (временный)
	copy(s[1:], s)      // сдвигаем вправо
	s[0] = elem         // вставляем в начало
	return s
}

// Добавление нескольких элементов в начало
func prependMany[T any](s []T, elems ...T) []T {
	return append(elems, s...)
}

// 2. ОБЪЕДИНЕНИЕ ДВУХ СЛАЙСОВ

// Способ 1: через append с разворотом
func concat1[T any](a, b []T) []T {
	return append(a, b...)
}

// Способ 2: через make + copy (более эффективно, без лишних аллокаций)
func concat2[T any](a, b []T) []T {
	result := make([]T, len(a)+len(b))
	copy(result, b)
	copy(result[len(a):], b)
	return result
}

// Объединение нескольких слайсов
func concatMany[T any](slices ...[]T) []T {
	totalLen := 0
	for _, s := range slices {
		totalLen += len(s)
	}

	result := make([]T, 0, totalLen)
	for _, s := range slices {
		result = append(result, s...)
	}
	return result
}

// 3. РАЗБИВКА СЛАЙСА НА N ЧАСТЕЙ

// Разбить на слайсы по size элементов

func chunk[T any](s []T, size int) [][]T {
	if size <= 0 {
		return [][]T{}
	}

	chunks := make([][]T, 0, (len(s)+size-1)/size)
	for i := 0; i < len(s); i++ {
		end := i + size
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[i:end])
	}
	return chunks
}

// Разбить на n равных частей (насколько возможно)
func chunkByCount[T any](s []T, n int) [][]T {
	if n <= 0 {
		return [][]T{}
	}
	if n > len(s) {
		n = len(s)
	}

	chunks := make([][]T, 0, n)
	chunkSize := (len(s) + n - 1) / n

	for i := 0; i < n; i++ {
		start := i * chunkSize
		if start >= len(s) {
			chunks = append(chunks, []T{})
			continue
		}
		end := start + chunkSize
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[start:end])
	}
	return chunks
}

// 4. СОРТИРОВКА СЛАЙСОВ

// Базовая сортировка для встроенных типов
func sortBasic() {
	// Числа (по возрастанию)
	nums := []int{5, 2, 8, 1, 9}
	sort.Ints(nums)
	fmt.Println("Sorted ints:", nums)

	// Числа (по убыванию)
	numsDesc := []int{5, 2, 8, 1, 9}
	sort.Sort(sort.Reverse(sort.IntSlice(numsDesc)))
	fmt.Println("Desc ints:", numsDesc)

	// Строки (по возрастанию)
	strs := []string{"banana", "apple", "cherry"}
	sort.Strings(strs)
	fmt.Println("Sorted strings:", strs)

	// Float64
	floats := []float64{3.14, 1.5, 2.7}
	sort.Float64s(floats)
	fmt.Println("Sorted floats:", floats)
}

// Кастомная сортировка (для структур)
type Person struct {
	Name string
	Age  int
}

func sortCustom() {
	people := []Person{
		{"Alice", 30},
		{"Bob", 25},
		{"Charlie", 35},
	}

	// По возрасту
	sort.Slice(people, func(i, j int) bool {
		return people[i].Age < people[j].Age
	})
	fmt.Println("By age:", people)

	// По имени
	sort.Slice(people, func(i, j int) bool {
		return people[i].Name < people[j].Name
	})
	fmt.Println("By name:", people)

	// По нескольким полям
	sort.Slice(people, func(i, j int) bool {
		if people[i].Age == people[j].Age {
			return people[i].Name < people[j].Name
		}
		return people[i].Age < people[j].Age
	})
}

// 5. ДРУГИЕ ВАЖНЫЕ ОПЕРАЦИИ
// Реверс слайса
func reverse[T any](s []T) []T {
	result := make([]T, len(s))
	for i, j := 0, len(s)-1; i <= j; i, j = i+1, j-1 {
		result[i], result[j] = s[j], s[i]
	}
	return result
}

// Реверс in-place
func reverseInPlace[T any](s []T) {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
}

// Уникальные элементы (для comparable)
func unique[T comparable](s []T) []T {
	seen := make(map[T]bool)
	result := make([]T, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}

// Вставка в произвольную позицию
func insert[T any](s []T, index int, value T) []T {
	if index < 0 || index > len(s) {
		return s
	}
	// Расширяем слайс
	s = append(s, value)
	// Сдвигаем элементы вправо
	copy(s[index+1:], s[index:])
	s[index] = value
	return s
}

// Разность двух слайсов (a - b)
func difference[T comparable](a, b []T) []T {
	bSet := make(map[T]bool)
	for _, v := range b {
		bSet[v] = true
	}

	result := make([]T, 0)
	for _, v := range a {
		if !bSet[v] {
			result = append(result, v)
		}
	}
	return result
}

// Пересечение слайсов
func intersection[T comparable](a, b []T) []T {
	set := make(map[T]bool)
	for _, v := range a {
		set[v] = true
	}

	result := make([]T, 0)
	for _, v := range b {
		if set[v] {
			result = append(result, v)
			delete(set, v) // убираем, чтобы не дублировать
		}
	}
	return result
}

func main() {
	fmt.Println("=== ДОБАВЛЕНИЕ В НАЧАЛО ===")
	s1 := []int{2, 3, 4}
	fmt.Println("Original:", s1)
	fmt.Println("Prepend 1:", prependNew(s1, 1))
	fmt.Println("Prepend many:", prependMany(s1, 0, 1, 2))

	fmt.Println("\n=== ОБЪЕДИНЕНИЕ ===")
	a := []int{1, 2}
	b := []int{3, 4}
	fmt.Println("Concat:", concat1(a, b))
	fmt.Println("Concat many:", concatMany(a, b, []int{5, 6}))

	fmt.Println("\n=== РАЗБИВКА ===")
	s2 := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	fmt.Println("Chunk size 3:", chunk(s2, 3))
	fmt.Println("Chunk count 4:", chunkByCount(s2, 4))

	fmt.Println("\n=== СОРТИРОВКА ===")
	sortBasic()
	sortCustom()

	fmt.Println("\n=== ДРУГИЕ ОПЕРАЦИИ ===")
	s3 := []int{1, 2, 3, 4, 5}
	fmt.Println("Reverse:", reverse(s3))

	s4 := []int{1, 2, 2, 3, 3, 3, 4}
	fmt.Println("Unique:", unique(s4))

	fmt.Println("Insert 99 at index 2:", insert(s3, 2, 99))

	fmt.Println("\n=== МНОЖЕСТВА ===")
	setA := []int{1, 2, 3, 4}
	setB := []int{3, 4, 5, 6}
	fmt.Println("Difference (A-B):", difference(setA, setB))
	fmt.Println("Intersection:", intersection(setA, setB))
}
