package _interface

import "fmt"

// 1.ЧТО ТАКОЕ ИНТЕРФЕЙСЫ
/*
ЧТО ЭТО:
- Интерфейс — это набор методов, которые должен реализовать тип.
- Интерфейсы в Go — **неявные**: тип реализует интерфейс, если реализует все его методы.
- Это основной механизм полиморфизма в Go.

ПОЧЕМУ ЭТО ВАЖНО:
- Позволяет писать гибкий и расширяемый код.
- Упрощает тестирование через моки.
- Реализует принцип "программируй к интерфейсу, а не к типу".

ПРИМЕР БАЗОВОГО ИНТЕРФЕЙСА:
*/
type Speaker interface {
	Speak() string
}
type Person struct {
	Name string
}

func (p Person) Speak() string {
	return "Привет, меня зовут" + p.Name
}
func (p Person) Greet() string {
	return "Здравствуйте, " + p.Name
}

func demBasicInterface() {
	p := Person{Name: "Vlad"}

	// Person реализует интерфейс Speaker, потому что имеет метод Speak()
	var s Speaker = p
	fmt.Println(s.Speak())

	// Person не реализует интерфейс, который требует метод Greet()
	// type Greeter interface { Greet() string }
	// var g Greeter = p // ОШИБКА: Person не имеет метода Greet()
}
