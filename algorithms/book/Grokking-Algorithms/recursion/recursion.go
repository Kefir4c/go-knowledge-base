package main

import "fmt"

func main() {
	// recursNumber(10)
	// greet("Kolya")
	fmt.Println(factorial(4))
}

type Box struct {
	items  []Box
	hasKey bool
}

//1. Вариант без рекурсии (Итеративный)
func lookForKeyIterative(mainBox Box) {
	// Создаем "кучу" коробок и кладем туда первую
	pile := []Box{mainBox}

	// Пока куча не пуста
	if len(pile) > 0 {
		box := pile[len(pile)-1]   // Берем последнюю коробку из кучи
		pile := pile[:len(pile)-1] // Удаляем её из кучи (аналог pop)

		for _, item := range box.items {
			if item.hasKey {
				fmt.Println("Мы нашли ключ")
				return
			} else {
				// Если нашли коробку — кидаем её в кучу, чтобы проверить позже
				pile = append(pile, item)
				fmt.Println("Нашел коробку, добавил в кучу.")
			}
		}
	}
}

//2. Вариант с рекурсией
func lookForKeyRecursive(box Box) {
	for _, item := range box.items {
		if item.hasKey {
			fmt.Println("Мы нашли ключ")
			return
		} else {
			// Вместо того чтобы добавлять в список, мы просто
			// "ныряем" в функцию еще раз
			lookForKeyRecursive(item)
		}
	}
}

func recursNumber(i int) {
	if i == 0 {
		return
	}
	fmt.Println(i)
	recursNumber(i - 1)
}

func greet(name string) {
	fmt.Printf("hello %s!", name)
	fmt.Println()
	greet2(name)
	fmt.Println("getting ready to say bye...")
	bye()
}
func greet2(name string) {
	fmt.Printf("How are you %s?", name)
	fmt.Println()
}
func bye() { println("ok, bye! ") }

func factorial(nums int) int {
	if nums == 1 {
		return 1
	}
	return nums * factorial(nums-1)
}
