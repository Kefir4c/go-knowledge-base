package types

import (
	"fmt"
	"unicode/utf8"
)

/*
iota — это счётчик строк внутри блока const. Начинается с 0 и увеличивается
на 1 для каждой следующей строки.
*/

const (
	A = iota // 0
	B        // 1
	C        // 2
)

// ПРИМЕР 1. БАЗОВЫЙ ENUM
type orderStatus int

const (
	created orderStatus = iota
	paid
	shipped
	delivered
	cancelled
)

func primer1() {
	status := shipped
	fmt.Println(status) // выведет 2
}

// ПРИМЕР 2. ENUM С STRINGER (правильный способ)
// 1. Объявляем тип
type orederStrStatus int

// 2. Объявляем константы с iota
const (
	createdStr orederStrStatus = iota
	paidStr
	shippedStr
	deliveredStr
	cancelledStr
)

// 3. Реализуем интерфейс String() string
func (s orederStrStatus) string() string {
	switch s {
	case createdStr:
		return "Created"
	case paidStr:
		return "Paid"
	case shippedStr:
		return "Shipped"
	case deliveredStr:
		return "Delivered"
	case cancelledStr:
		return "Cancelled"
	default:
		return "Unknown"
	}
}

func primer2() {
	status := shippedStr
	fmt.Println(status) // выведет "Shipped" 🎉

	// Можно сравнивать
	if status == shippedStr {
		fmt.Println("Order is shipped")
	}
}

// ПРИМЕР 3. ENUM С БОЛЬШЕ ВОЗМОЖНОСТЕЙ
// Дни недели
type weekday int

const (
	Monday    weekday = iota // 0
	Tuesday                  // 1
	Wednesday                // 2
	Thursday                 // 3
	Friday                   // 4
	Saturday                 // 5
	Sunday                   // 6
)

func (d weekday) String() string {
	return [...]string{
		"Monday",
		"Tuesday",
		"Wednesday",
		"Thursday",
		"Friday",
		"Saturday",
		"Sunday",
	}[d]
}

func (d weekday) inWeekend() bool {
	return d == Saturday || d == Sunday
}

func primer3() {
	day := Saturday
	fmt.Println(day)             // Saturday
	fmt.Println(day.inWeekend()) // true
}

//ПРИМЕР 4. КАСТОМНЫЕ ЗНАЧЕНИЯ (НЕ С 0)

// Уровни доступа
type Permission int

const (
	Read    Permission = 1 << iota // 1 << 0 = 1
	Write                          // 1 << 1 = 2
	Execute                        // 1 << 2 = 4
	Delete                         // 1 << 3 = 8
)

func (p Permission) String() string {
	switch p {
	case Read:
		return "Read"
	case Write:
		return "Write"
	case Execute:
		return "Execute"
	case Delete:
		return "Delete"
	}
	return "Unknown"
}

// Комбинирование разрешений (битовые маски)
func primer4() {
	var perms Permission = Read | Write   // 1 | 2 = 3
	fmt.Printf("Permission: &b\n", perms) // выведет 11 (в бинарном виде)

	if perms&Read != 0 {
		fmt.Println("Can read")
	}
	if perms&Write != 0 {
		fmt.Println("Can write")
	}
}

func dopPrimer4() {
	var per Permission = Read | Write
	fmt.Printf("Permission: %b\n", per)
	if per&Execute == 0 {
		per /= Execute
	}
	fmt.Printf("Permission: %b\n", per)
}

//Сравнить string, []byte, []rune на UTF-8

//ПРИМЕР 1. Разбор

func snrPrimer1() {
	// Строка с эмодзи (занимает 4 байта!)
	s := "Привет👋"

	fmt.Printf("Значение: %s\n", s)
	fmt.Printf("Тип: %T\n", s)
	fmt.Printf("Длина len(): %d байт\n", len(s))
	fmt.Printf("Количетсво символов (рун): &d\n", utf8.RuneCountInString(s))

	bs := []byte(s)
	fmt.Printf("Значение: %v\n", bs)
	fmt.Printf("Тип: %T\n", bs)
	fmt.Printf("Длина len(): %d байт\n", len(bs))
	fmt.Printf("Как строка: %s\n", string(bs))

	rs := []rune(s)
	fmt.Printf("Значение: %v\n", rs)
	fmt.Printf("Тип: %T\n", rs)
	fmt.Printf("Длина len(): %d символов\n", len(rs))
	fmt.Printf("Как строка: %s\n", string(rs))
}

//КАК ПРАВИЛЬНО ИТЕРИРОВАТЬ ПО СТРОКЕ

//НЕПРАВИЛЬНО (по байтам)

func intrStr1() {
	s := "Привет👋"
	for i := 0; i < len(s); i++ {
		fmt.Printf("c", s[i])
	}
}

// ПРАВИЛЬНО (по rune) - 3 способа
func intrStr2() {
	s := "Привет👋"

	// Способ 1: range автоматически даёт rune
	for ind, r := range s {
		fmt.Printf("%d: %c (U+%04X)\n", ind, r, r)
	}

	// Способ 2: преобразование в []rune
	rs := []rune(s)
	for i, r := range rs {
		fmt.Printf("%d: %c\n", i, r)
	}
}
