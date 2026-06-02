package deepgo

import (
	"database/sql"
	"fmt"
	"time"
)

/*
эмбеддинг структур и интерфейсов, повышение методов, диамонд-проблема
и почему её нет в Go
*/

//ЭМБЕДДИНГ СТРУКТУР И ИНТЕРФЕЙСОВ В GO
/*
Что такое эмбеддинг (встраивание)
Эмбеддинг в Go — это механизм, при котором одна структура или интерфейс
включает в себя другой тип без указания имени поля. В отличие от классического
наследования в ООП, эмбеддинг в Go — это чистая композиция: встроенный тип
становится частью внешнего, а его поля и методы "поднимаются" (promote) во
внешний тип.
*/

type animal struct {
	name string
}

func (a animal) speak() string {
	return "Some sound"
}

type dog struct {
	animal // эмбеддинг без имени!
	Breed  string
}

func prime1() {
	d := dog{
		animal: animal{name: "Rex"},
		Breed:  "German",
	}

	// Поля Animal доступны напрямую
	fmt.Println(d.name)    // "Rex" — поднятое поле
	fmt.Println(d.speak()) // "Some sound" — поднятый метод
}

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Writer interface {
	Write(p []byte) (n int, err error)
}

type ReadWriter interface {
	Reader // встроенный интерфейс
	Writer // встроенный интерфейс
}

type file struct{}

func (f file) Read(p []byte) (n int, err error)  { return 0, nil }
func (f file) Write(p []byte) (n int, err error) { return 0, nil }

var rw ReadWriter = file{}

//Повышение методов (method promotion)
/*
Когда тип встроен во внешнюю структуру, все его методы становятся
доступны напрямую через внешнюю структуру. Это называется повышением методов.
*/

type logger struct{}

func (l logger) log(msg string) {
	fmt.Println("[LOG]", msg)
}

type userService struct {
	logger // эмбеддинг
	db     *sql.DB
}

func primer2() {
	serv := userService{}
	serv.log("user created")
}

type animal3 struct{}

func (animal3) speak3() string {
	return "???"
}

type dog3 struct {
	animal3
}

func (dog3) speak3() string { // переопределение
	return "Woof!"
}

func primer3() {
	d := dog3{}
	fmt.Println(d.speak3())         // "Woof!" — метод Dog
	fmt.Println(d.animal3.speak3()) // "???" — явный вызов метода Animal
}

//Почему в Go нет проблемы ромба (diamond problem)

/*
Что такое diamond problem: Это проблема множественного наследования
в языках типа C++, когда класс наследуется от двух классов, которые,
в свою очередь, наследуются от одного общего предка. Возникает неоднозначность:
какой метод предка вызывать?

    A
   / \
  B   C
   \ /
    D
*/

type A struct{}

func (A) Do() string { return "A" }

type B struct{ A }

func (B) Do() string { return "B" } // переопределение

type C struct{ A }

func (C) Do() string { return "C" } // переопределение

type D struct {
	B
	C
}

func primer4() {
	d := D{}
	// d.Do() — ОШИБКА! Неоднозначность
	// Go не знает, вызывать B.Do или C.Do
	// Но это НЕ diamond problem в классическом смысле!
	// Это просто конфликт имён на одном уровне
	// Решается явным указанием:
	fmt.Println(d.B.Do()) // "B"
	fmt.Println(d.C.Do()) // "C"
}

/*
Когда использовать эмбеддинг
Эмбеддинг структур — уместен:
* Для общего набора полей (BaseEntity с ID, CreatedAt, UpdatedAt)
* Для композиции сервисов (Service с встроенным Logger)
* Для переиспользования методов (embedding sync.Mutex)
*/

type base struct {
	ID        int64
	CreatedAt time.Time
}

type user struct {
	base
	Name string
}

type Product struct {
	base
	Price int
}
