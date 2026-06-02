package main

import (
	"fmt"
	"runtime"
	"time"
)

//ПРИМЕР 1. VALUE VS POINTER RECEIVER ДЛЯ БОЛЬШОЙ СТРУКТУРЫ

// Большая структура (512 байт)
type bigStruct struct {
	data [102400]int
	id   int
}

// Value receiver (копирует всю структуру)
func (b bigStruct) valueMethod() int {
	sum := 0

	for i := 0; i < len(b.data); i++ {
		sum += b.data[i]
	}
	return sum + b.id
}

// Pointer receiver (не копирует)
func (b *bigStruct) pointerMethod() int {
	sum := 0

	for i := 0; i < len(b.data); i++ {
		sum += b.data[i]
	}
	return sum + b.id
}

func benchmark(method func()) time.Duration {
	start := time.Now()

	for i := 0; i < 1000; i++ {
		method()
	}
	return time.Since(start)
}

func primer1() {
	bs := &bigStruct{id: 42}

	// Убеждаемся, что структура не маленькая
	fmt.Printf("Size of BigStruct: %d bytes\n", 512)

	// Бенчмарк value receiver0
	valueTime := benchmark(func() {
		bs.valueMethod()
	})
	fmt.Printf("Value receiver: %v\n", valueTime)

	// Бенчмарк pointer receiver
	pointerTime := benchmark(func() {
		bs.pointerMethod()
	})
	fmt.Printf("Pointer receiver: %v\n", pointerTime)
}

// ПОЧЕМУ FOR _, V := RANGE КОПИРУЕТ ЭЛЕМЕНТЫ

type User struct {
	Name string
	Age  int
}

func primer2() {
	users := []User{
		{"Alice", 25},
		{"Bob", 30},
		{"Charlie", 35},
	}

	// ❌ НЕ РАБОТАЕТ: меняем копию
	for _, u := range users {
		u.Age++ // меняет копию, оригинал не изменился!
	}
	fmt.Println("After range (copy):", users) // [{Alice 25} {Bob 30} {Charlie 35}]

	for i := range users {
		users[i].Age++
	}
	fmt.Println("After index:", users) // [{Alice 26} {Bob 31} {Charlie 36}]

	// ✅ РАБОТАЕТ: со слайсом указателей
	pointers := []*User{
		{"Alice", 25},
		{"Bob", 30},
	}

	for _, us := range pointers {
		us.Age++ // меняем оригинал, потому что p — указатель
	}
	fmt.Println("After range on pointers:", pointers[0].Age) // 26
}

// *INT VS INT В MAP'Е

func mapWithIntValues() map[int]int {
	m := make(map[int]int)
	for i := 0; i < 100000; i++ {
		m[i] = i
	}
	return m
}

func mapWithPointerValues() map[int]*int {
	m := make(map[int]*int)

	for i := 0; i < 100000; i++ {
		val := i
		m[i] = &val
	}
	return m
}

// Изменение значения в map с int
func modifyIntValue(m map[int]int) {
	for k := range m {
		m[k] = k + 1
	}
}

// Изменение значения в map с *int
func modifyPointerValue(m map[int]*int) {
	for _, v := range m {
		*v++
	}
}

func main() {
	// 1. РАЗНИЦА В ПАМЯТИ
	fmt.Println("=== РАЗНИЦА В ПАМЯТИ ===")

	runtime.GC()
	m1 := mapWithIntValues()
	fmt.Printf("Map[int]int: %d elements\n", len(m1))

	runtime.GC()
	m2 := mapWithPointerValues()
	fmt.Printf("Map[int]*int: %d elements\n", len(m2))

	// 2. ИЗМЕНЕНИЕ ЗНАЧЕНИЙ
	fmt.Println("\n=== ИЗМЕНЕНИЕ ЗНАЧЕНИЙ ===")

	// С int значениями
	testInt := map[int]int{1: 10, 2: 20, 3: 30}
	fmt.Println("Before modify int:", testInt)
	modifyIntValue(testInt)
	fmt.Println("After modify int:", testInt)

	// С указателями
	testPtr := map[int]*int{}
	a, b, c := 10, 20, 30
	testPtr[1], testPtr[2], testPtr[3] = &a, &b, &c
	fmt.Println("Before modify ptr:", getValues(testPtr))
	modifyPointerValue(testPtr)
	fmt.Println("After modify ptr:", getValues(testPtr))

	// 3. ПРОБЛЕМА С УКАЗАТЕЛЯМИ
	fmt.Println("\n=== ПРОБЛЕМА С УКАЗАТЕЛЯМИ ===")

	// ❌ НЕПРАВИЛЬНО: все указывают на одну переменную!
	badMap := make(map[int]*int)
	var val int
	for i := 0; i < 5; i++ {
		val = i
		badMap[i] = &val // все указывают на val!
	}
	fmt.Println("Bad map pointers:")
	for k, v := range badMap {
		fmt.Printf("  %d -> %d\n", k, *v) // все 4! (последнее значение)
	}

	// ✅ ПРАВИЛЬНО: создаём новую переменную
	goodMap := make(map[int]*int)
	for i := 0; i < 5; i++ {
		val := i // новая переменная на каждой итерации
		goodMap[i] = &val
	}
	fmt.Println("Good map pointers:")
	for k, v := range goodMap {
		fmt.Printf("  %d -> %d\n", k, *v) // 0,1,2,3,4
	}
}

func getValues(m map[int]*int) map[int]int {
	result := make(map[int]int)
	for k, v := range m {
		result[k] = *v
	}
	return result
}
