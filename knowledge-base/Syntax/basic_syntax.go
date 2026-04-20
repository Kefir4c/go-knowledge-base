package main

import (
	"fmt"
	"time"
)

var counter = 0

var (
	counterV1 = 0
)

const Pi = 3.1415

const (
	PiV1 = 3.14
)

func main() {
	// 1. Объявления переменной и нюансы

	// Long Assign (var): используем, когда нужно zero value или четкий тип
	var i int
	var s string
	var b bool
	var f float64

	//short Assing (:=) используеться для только внутри функции, сахар: быстро и удобно
	timeSleep := 2 * time.Second
	message := "Привет друг"

	// множественное присваивание уменьшает код
	x, y := 10, 20

	// const: это неизменяемые переменные, не имеют адреса в памяти (нельзя взять указатель &)
	const Moscow = "Москва"

	// 2. Циклы for.

	// А. Классический вариант
	for i := 0; i < 10; i++ {
		fmt.Printf("Итерация: %d\n", i)
	}

	// Б. Аналог while (условие на входе)
	n := 1
	for n < 5 {
		n *= 2
	}

	//B. бесконечный цикл
	for {
		fmt.Println("Всем привет...")
		break
	}

	// Г. For Range (итерация по коллекции)
	// Важно: v — это всегда КОПИЯ значения
	slice := []string{"a", "b"}
	for i, v := range slice {
		fmt.Printf("Индекс: %d, Значение: %s\n", i, v)
	}

	// КАВЕРЗНЫЙ МОМЕНТ: Range и указатели
	numbers := []int{1, 2, 3}
	for _, v := range numbers {
		v = v * 10 // Мы меняем КОПИЮ
	}
	// numbers всё еще [1, 2, 3]!

	// Чтобы изменить оригинал, используй индекс:
	for i := range numbers {
		numbers[i] *= 10
	}
	// numbers теперь [10, 20, 30]

	//3. УСЛОВНЫЕ ОПЕРАТОРЫ И SWITCH (МАКСИМАЛЬНЫЙ ОХВАТ)

	// IF
	// Мы создаем переменную 'count'вне условия.
	count := 5
	if count > 3 {
		fmt.Printf("Значение: %d - больше 3", count)
	}
	fmt.Printf("Значение: %d - меньше 3", count)

	// --- IF С ИНИЦИАЛИЗАЦИЕЙ ---
	// Мы создаем переменную 'temp' прямо в условии.
	// Она видна ТОЛЬКО внутри этого if/else. Это "чистит" память после выполнения.
	if temp := 25; temp > 20 {
		fmt.Printf("Температура %d — тепло\n", temp)
	}
	// fmt.Println(temp) // ОШИБКА: temp здесь уже не существует!

	// SWITCH: БАЗА
	// 1. Можно проверять несколько значений в одном case
	// 2. break не нужен — он автоматический
	os := "linux"
	switch os {
	case "darwin", "ios":
		fmt.Println("Apple")
	case "linux":
		fmt.Println("Open Source")
	default:
		fmt.Println("Other OS")
	}

	// SWITCH БЕЗ УСЛОВИЯ (TAGLESS)
	// Идеально заменяет кучу if-else if. Выглядит чище.
	hour := 15
	switch {
	case hour < 12:
		fmt.Println("Утро")
	case hour >= 12 && hour < 18:
		fmt.Println("День")
	case hour >= 18:
		fmt.Println("Вечер")
	}

	// TYPE SWITCH
	// Это мастхэв. Проверка того, что пришло в "пустой интерфейс"
	var val interface{} = "I am a string"

	switch v := val.(type) {
	case int:
		fmt.Printf("Это целое число: %d\n", v)
	case string:
		fmt.Printf("Это строка длиной %d\n", len(v))
	default:
		fmt.Printf("Неизвестный тип: %T\n", v)
	}

	// 5. УПРАВЛЕНИЕ ЦИКЛАМИ: BREAK, CONTINUE & LABELS

	for i := 0; i < 5; i++ {
		if i == 1 {
			continue // Пропустить текущую итерацию (число 1 не напечатается)
		}
		if i == 3 {
			break // Полностью выйти из цикла при достижении 3
		}
		fmt.Print(i, " ") // Выведет: 0 2
	}
	fmt.Println()

	//МЕТКИ (LABELS)
	// Идеально, когда у тебя вложенные циклы и нужно выйти из обоих сразу
outerLoop:
	for i := 0; i < 3; i++ {
		for j := 0; j < 3; j++ {
			if i == 1 && j == 1 {
				fmt.Println("Выходим из всех циклов сразу через метку")
				break outerLoop
			}
			fmt.Printf("i:%d j:%d | ", i, j)
		}
	}

	//BREAK В SWITCH
	// Обычно не нужен, но полезен для раннего выхода
	num := 10
	switch {
	case num > 0:
		if num == 10 {
			fmt.Println("\nНашли 10, выходим из switch досрочно")
			break // Прерывает выполнение текущего кейса
		}
		fmt.Println("Это не напечатается")
	}

	//FALLTHROUGH
	// Если нужно, чтобы после одного кейса выполнился и следующий
	number := 10
	switch {
	case number <= 10:
		fmt.Println("Число <= 10")
		fallthrough // Перекинет в следующий блок вне зависимости от его условия
	case number > 100:
		fmt.Println("Это выведется из-за fallthrough, хотя условие ложно")
	}

	// 6. ПАМЯТЬ: NEW vs MAKE

	// new: память под ноль, возвращает указатель *T
	p := new(int) // *p = 0

	// make: создает структуру, возвращает готовое значение T
	// Только для slice, map, chan
	m := make(map[string]int)
	m["ready"] = 1

	// 7. ПРОИЗВОДИТЕЛЬНОСТЬ: ЛОКАЛЬНОСТЬ КЭША

	const (
		rows = 2000
		cols = 2000
	)

	// Создаем матрицу (слайс слайсов)
	matrix := make([][cols]int, rows)

	// Эффективный способ: Row-major order (по строкам)
	// Мы идем последовательно по памяти, процессор счастлив.
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			matrix[i][j] = 1
		}
	}

	// Неэффективный способ: Column-major order (по столбцам)
	// Мы прыгаем через огромные куски памяти, заставляя кэш постоянно обновляться.
	for j := 0; j < cols; j++ {
		for i := 0; i < rows; i++ {
			matrix[i][j] = 2
		}
	}
}
